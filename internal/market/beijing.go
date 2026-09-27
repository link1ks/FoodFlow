// Package market reads public official monitoring data without estimating missing prices.
package market

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"time"

	"foodflow/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
)

const vegetableURL = "https://www.beijingprice.cn/data/template/93ee358f-fac8-11ee-acc7-00ffefc89a05.json"
const staplesURL = "https://www.beijingprice.cn/data/drjg/drjg.json"

type Quote struct {
	Name, Original, Specification, Price string
	Day                                  time.Time
}
type vegetableFeed struct {
	Code int `json:"code"`
	Data struct {
		Date    string     `json:"exeNo"`
		Columns [][]string `json:"data"`
	} `json:"data"`
}
type staple struct {
	Name  string      `json:"ItemName"`
	Unit  string      `json:"ItemUnit"`
	Level string      `json:"ItemLevel"`
	Date  string      `json:"PriceDate"`
	Price json.Number `json:"price04"`
}

// Mappings preserve original variety/grade in the quote; broad categories are never
// assigned to a narrower ingredient such as chicken breast or a particular fish.
var names = map[string]string{"西红柿": "番茄", "葱头": "洋葱", "圆茄子": "茄子", "圆白菜": "卷心菜", "菜花": "花菜", "生姜": "姜", "大蒜": "蒜", "鲜牛肉": "牛肉", "鲜羊肉": "羊肉", "鲜猪肉": "猪肉", "白条鸡": "鸡肉", "鸭子": "鸭肉", "粳米": "大米", "富强粉": "面粉", "标准粉": "面粉"}

func quote(name, spec, price, date string, now time.Time) (Quote, error) {
	day, err := time.Parse("2006-01-02", date)
	if err != nil || date > now.In(time.FixedZone("Asia/Shanghai", 8*3600)).Format("2006-01-02") {
		return Quote{}, fmt.Errorf("invalid monitoring date %q", date)
	}
	n, ok := new(big.Rat).SetString(price)
	if !ok || n.Sign() <= 0 || n.Cmp(big.NewRat(99999999, 100)) > 0 {
		return Quote{}, fmt.Errorf("invalid official price %q", price)
	}
	canonical := name
	if v, ok := names[name]; ok {
		canonical = v
	}
	return Quote{canonical, name, "超市零售均价 · " + spec, n.FloatString(2), day}, nil
}

func ParseVegetables(body []byte, now time.Time) ([]Quote, error) {
	var f vegetableFeed
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	c := f.Data.Columns
	if f.Code != 200 || len(c) != 5 || len(c[0]) == 0 || len(c[0]) > 500 {
		return nil, fmt.Errorf("unexpected vegetable feed schema")
	}
	for _, column := range c {
		if len(column) != len(c[0]) {
			return nil, fmt.Errorf("mismatched price columns")
		}
	}
	out := make([]Quote, 0, len(c[0]))
	for i, name := range c[0] {
		// Column 4 is supermarket retail CNY/斤 (500g), not wholesale or volume.
		if c[4][i] == "" || c[4][i] == "-" {
			continue
		}
		q, err := quote(name, "普通鲜菜", c[4][i], f.Data.Date, now)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

func ParseStaples(body []byte, now time.Time) ([]Quote, error) {
	var f struct {
		Oil  []staple `json:"oilList"`
		Meat []staple `json:"meatList"`
	}
	if err := json.Unmarshal(body, &f); err != nil {
		return nil, err
	}
	if len(f.Oil)+len(f.Meat) == 0 {
		return nil, fmt.Errorf("empty staple feed")
	}
	var out []Quote
	for _, item := range append(f.Oil, f.Meat...) {
		// Skip absent/zero retail observations and unsupported pack sizes. Never use
		// a wholesale price as the missing retail price, or convert oil mass to volume.
		if item.Unit != "元／500克" || item.Price == "" {
			continue
		}
		n, ok := new(big.Rat).SetString(string(item.Price))
		if ok && n.Sign() == 0 {
			continue
		}
		q, err := quote(item.Name, item.Level, string(item.Price), item.Date, now)
		if err != nil {
			return nil, err
		}
		out = append(out, q)
	}
	return out, nil
}

func fetch(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	res, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("official source HTTP %d", res.StatusCode)
	}
	b, err := io.ReadAll(io.LimitReader(res.Body, 2*1024*1024+1))
	if len(b) > 2*1024*1024 {
		return nil, fmt.Errorf("official response too large")
	}
	return b, err
}

// SyncBeijing is atomic and replay-safe. No household data leaves this process.
func SyncBeijing(ctx context.Context, db *pgxpool.Pool) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 45*time.Second)
	defer cancel()
	client := &http.Client{Timeout: 20 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	veg, err := fetch(ctx, client, vegetableURL)
	if err != nil {
		return 0, err
	}
	rows, err := ParseVegetables(veg, time.Now())
	if err != nil {
		return 0, err
	}
	raw, err := fetch(ctx, client, staplesURL)
	if err != nil {
		return 0, err
	}
	extra, err := ParseStaples(raw, time.Now())
	if err != nil {
		return 0, err
	}
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())
	q := dbgen.New(tx)
	count := 0
	for _, r := range append(rows, extra...) {
		var price pgtype.Numeric
		_ = price.Scan(r.Price)
		n, err := q.UpsertOfficialPrice(ctx, dbgen.UpsertOfficialPriceParams{Price: price, SourceItem: r.Original, Specification: r.Specification, Day: pgtype.Date{Time: r.Day, Valid: true}, Ingredient: r.Name})
		if err != nil {
			return 0, err
		}
		count += int(n)
	}
	return count, tx.Commit(ctx)
}
