package market

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"foodflow/internal/dbgen"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/net/html"
)

const nationalIndex = "https://www.chinaprice.cn/sp/index.jhtml"
const suzhouIndex = "https://fg.suzhou.gov.cn/szfgw/scdt/nav_list.shtml"

var cityCodes = map[string]string{"北京市": "110100", "天津市": "120100", "石家庄市": "130100", "太原市": "140100", "呼和浩特市": "150100", "沈阳市": "210100", "大连市": "210200", "长春市": "220100", "哈尔滨市": "230100", "上海市": "310100", "南京市": "320100", "苏州市": "320500", "杭州市": "330100", "宁波市": "330200", "合肥市": "340100", "福州市": "350100", "厦门市": "350200", "南昌市": "360100", "济南市": "370100", "青岛市": "370200", "郑州市": "410100", "武汉市": "420100", "长沙市": "430100", "广州市": "440100", "深圳市": "440300", "南宁市": "450100", "海口市": "460100", "重庆市": "500100", "成都市": "510100", "贵阳市": "520100", "昆明市": "530100", "拉萨市": "540100", "西安市": "610100", "兰州市": "620100", "西宁市": "630100", "银川市": "640100", "乌鲁木齐市": "650100"}

type RegionalQuote struct {
	City, Name, Original, Spec, Price, Unit, Agency, URL string
	Start, End                                           time.Time
}

func content(n *html.Node) string {
	var s strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			s.WriteString(n.Data)
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return strings.Join(strings.Fields(s.String()), "")
}

// HTML table headers may merge columns (e.g. lean pork and ribs). Expand colspan
// before matching to data, and reject unsupported rowspans rather than shift prices.
func tables(body []byte) ([][][]string, error) {
	return readTables(body, false)
}

func readTables(body []byte, allowCategoryRowspan bool) ([][][]string, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	var out [][][]string
	var visit func(*html.Node)
	visit = func(n *html.Node) {
		if n.Type == html.ElementNode && n.Data == "table" {
			var rows [][]string
			var rowWalk func(*html.Node)
			rowWalk = func(x *html.Node) {
				if x.Type == html.ElementNode && x.Data == "tr" {
					var row []string
					for c := x.FirstChild; c != nil; c = c.NextSibling {
						if c.Data != "td" && c.Data != "th" {
							continue
						}
						span := 1
						for _, a := range c.Attr {
							if a.Key == "rowspan" && a.Val != "1" && !allowCategoryRowspan {
								err = fmt.Errorf("unsupported rowspan")
							}
							if a.Key == "colspan" {
								span, _ = strconv.Atoi(a.Val)
							}
						}
						if span < 1 || span > 30 {
							err = fmt.Errorf("invalid colspan")
							return
						}
						for i := 0; i < span; i++ {
							row = append(row, content(c))
						}
					}
					if len(row) > 0 {
						rows = append(rows, row)
					}
					return
				}
				for c := x.FirstChild; c != nil; c = c.NextSibling {
					rowWalk(c)
				}
			}
			rowWalk(n)
			out = append(out, rows)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			visit(c)
		}
	}
	visit(doc)
	return out, err
}

var periodRE = regexp.MustCompile(`附表1[：:]?(\d{4})年(\d{1,2})月36个大中城市成品粮`)

func ParseNational(body []byte, source string, now time.Time) ([]RegionalQuote, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	match := periodRE.FindStringSubmatch(content(doc))
	if match == nil {
		return nil, fmt.Errorf("national monitoring period missing")
	}
	y, _ := strconv.Atoi(match[1])
	m, _ := strconv.Atoi(match[2])
	if m < 1 || m > 12 {
		return nil, fmt.Errorf("invalid month")
	}
	start := time.Date(y, time.Month(m), 1, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 1, -1)
	if end.After(now) {
		return nil, fmt.Errorf("future report")
	}
	ts, err := tables(body)
	if err != nil {
		return nil, err
	}
	if len(ts) < 7 {
		return nil, fmt.Errorf("national food tables missing")
	}
	var out []RegionalQuote
	for ti, t := range ts[:7] {
		if len(t) < 35 || len(t[0]) < 2 || t[0][0] != "品种名称" || t[1][0] != "规格等级" || t[2][0] != "计量单位" {
			return nil, fmt.Errorf("changed table %d", ti)
		}
		cols := len(t[0])
		if len(t[1]) != cols || len(t[2]) != cols {
			return nil, fmt.Errorf("header mismatch %d", ti)
		}
		cities := map[string]bool{}
		for _, row := range t[3:] {
			if len(row) == 0 || cityCodes[row[0]] == "" {
				continue
			}
			if cities[row[0]] || len(row) != cols {
				return nil, fmt.Errorf("city row mismatch %s", row[0])
			}
			cities[row[0]] = true
			for col := 1; col < cols; col++ {
				original, spec, unit := t[0][col], t[1][col], t[2][col]
				name := original
				switch original {
				case "晚籼米", "粳米":
					name = "大米"
				case "面粉一", "面粉二":
					name = "面粉"
				case "鲜猪肉":
					if spec == "肋排" {
						name = "排骨"
					} else {
						name = "猪肉"
					}
				case "鲜牛肉":
					name = "牛肉"
				case "鲜羊肉":
					name = "羊肉"
				case "西红柿":
					name = "番茄"
				case "圆白菜":
					name = "卷心菜"
				case "海虾":
					name = "虾"
				case "食用盐":
					name = "食盐"
				case "白砂糖":
					name = "白糖"
				case "花生油", "菜籽油", "豆油", "大豆调和油":
					name = "食用油"
				}
				value := row[col]
				if value == "" || value == "-" || value == "—" {
					continue
				}
				price, ok := new(big.Rat).SetString(value)
				if !ok || price.Sign() <= 0 {
					return nil, fmt.Errorf("invalid price %s %s", row[0], original)
				}
				switch unit {
				case "元/500克":
					unit = "500g"
				case "元/5升":
					unit = "1l"
					price.Quo(price, big.NewRat(5, 1))
					spec += "（原5升包装，折算每升）"
				default:
					continue
				}
				out = append(out, RegionalQuote{row[0], name, original, spec, price.FloatString(2), unit, "中国价格信息网（价格监测中心授权发布）", source, start, end})
			}
		}
		if len(cities) != 36 {
			return nil, fmt.Errorf("incomplete city table %d: %d", ti, len(cities))
		}
	}
	return out, nil
}

var suzhouDate = regexp.MustCompile(`苏州市部分农贸市场零售均价[（(](\d{4})年(\d{1,2})月(\d{1,2})日`)

func ParseSuzhou(body []byte, source string, now time.Time) ([]RegionalQuote, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	txt := content(doc)
	d := suzhouDate.FindStringSubmatch(txt)
	if d == nil || !strings.Contains(txt, "单位：元/500g") {
		return nil, fmt.Errorf("Suzhou date/unit missing")
	}
	y, _ := strconv.Atoi(d[1])
	m, _ := strconv.Atoi(d[2])
	day, _ := strconv.Atoi(d[3])
	date := fmt.Sprintf("%04d-%02d-%02d", y, m, day)
	ts, err := tables(body)
	if err != nil {
		return nil, err
	}
	var out []RegionalQuote
	alias := map[string]string{"大众粳米": "大米", "中等粳米": "大米", "特等粳米": "大米", "面粉（特一）": "面粉", "面粉（特二）": "面粉", "挂面": "面条", "生面": "面条", "腿肉": "猪肉", "夹心": "猪肉", "肋条": "猪肉", "大排": "排骨", "草鸡": "鸡肉", "肉鸡": "鸡肉", "鸭子": "鸭肉", "河虾": "虾", "包菜": "卷心菜", "西红柿": "番茄", "大蒜头": "蒜", "生姜": "姜", "山芋": "红薯"}
	for _, t := range ts {
		for _, row := range t {
			if len(row) != 3 {
				continue
			}
			if _, err := strconv.Atoi(row[0]); err != nil {
				continue
			}
			name := row[1]
			if strings.ContainsAny(name, "（(装") {
				if alias[name] == "" {
					continue
				}
			}
			q, err := quote(name, "", row[2], date, now)
			if err != nil {
				return nil, err
			}
			canonical := name
			if v := alias[name]; v != "" {
				canonical = v
			}
			out = append(out, RegionalQuote{"苏州市", canonical, name, "部分农贸市场零售均价 · " + name, q.Price, "500g", "苏州市发展和改革委员会", source, q.Day, q.Day})
		}
	}
	if len(out) < 30 {
		return nil, fmt.Errorf("Suzhou price rows missing")
	}
	return out, nil
}

func saveRegional(ctx context.Context, db *pgxpool.Pool, rows []RegionalQuote) (int, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(context.Background())
	q := dbgen.New(tx)
	count := 0
	for _, r := range rows {
		var price pgtype.Numeric
		_ = price.Scan(r.Price)
		n, err := q.UpsertRegionalPrice(ctx, dbgen.UpsertRegionalPriceParams{CityCode: cityCodes[r.City], City: r.City, Price: price, Unit: r.Unit, Agency: r.Agency, SourceUrl: r.URL, Original: r.Original, Specification: r.Spec, Day: pgtype.Date{Time: r.End, Valid: true}, PeriodStart: pgtype.Date{Time: r.Start, Valid: true}, Ingredient: r.Name})
		if err != nil {
			return 0, err
		}
		count += int(n)
	}
	return count, tx.Commit(ctx)
}

var reportLink = regexp.MustCompile(`https?://www\.chinaprice\.cn/jsdzqk/\d+\.jhtml`)

func nationalURL(body []byte) (string, error) {
	// The food index lists report parts 1,2,3. Part 2 contains the city food tables.
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Data == "a" && strings.HasPrefix(content(n), "中国市场价格监测报告") && strings.HasSuffix(content(n), "-2") {
			for _, a := range n.Attr {
				if m := reportLink.FindString(a.Val); m != "" && found == "" {
					found = strings.Replace(m, "http://", "https://", 1)
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found == "" {
		return "", fmt.Errorf("national report link missing")
	}
	return found, nil
}
func suzhouURL(body []byte) (string, error) {
	doc, err := html.Parse(bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	var found string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Data == "a" && suzhouDate.MatchString(content(n)) && found == "" {
			for _, a := range n.Attr {
				if a.Key == "href" {
					base, _ := url.Parse(suzhouIndex)
					v, e := base.Parse(a.Val)
					if e == nil && v.Host == base.Host && v.Scheme == "https" && strings.HasPrefix(v.Path, "/szfgw/scdt/") {
						found = v.String()
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	if found == "" {
		return "", fmt.Errorf("Suzhou daily link missing")
	}
	return found, nil
}

// SyncAll isolates source failures: a failed daily feed cannot roll back the
// successful national monthly update, and stale source dates never become today.
func SyncAll(ctx context.Context, db *pgxpool.Pool) (int, error) {
	total := 0
	var errs []error
	for _, source := range []struct {
		name, index string
		discover    func([]byte) (string, error)
		parse       func([]byte, string, time.Time) ([]RegionalQuote, error)
	}{{"national", nationalIndex, nationalURL, ParseNational}, {"suzhou", suzhouIndex, suzhouURL, ParseSuzhou}, {"fuzhou", fuzhouIndex, fuzhouURL, ParseFuzhou}} {
		n, err := func() (int, error) {
			ctx, cancel := context.WithTimeout(ctx, 65*time.Second)
			defer cancel()
			client := &http.Client{Timeout: 30 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
			index, err := fetch(ctx, client, source.index)
			if err != nil {
				return 0, err
			}
			link, err := source.discover(index)
			if err != nil {
				return 0, err
			}
			body, err := fetch(ctx, client, link)
			if err != nil {
				return 0, err
			}
			rows, err := source.parse(body, link, time.Now())
			if err != nil {
				return 0, err
			}
			return saveRegional(ctx, db, rows)
		}()
		total += n
		if err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", source.name, err))
		}
	}
	n, err := SyncBeijing(ctx, db)
	total += n
	if err != nil {
		errs = append(errs, fmt.Errorf("beijing: %w", err))
	}
	return total, errors.Join(errs...)
}
