package app

import (
	"context"
	"foodflow/internal/core"
	"foodflow/internal/dbgen"
	"foodflow/internal/market"
	"github.com/jackc/pgx/v5/pgtype"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"
)

func TestPriceChange(t *testing.T) {
	if priceChange("5", "") != nil {
		t.Fatal("missing comparison treated as stable")
	}
	got := priceChange("4.75", "5")
	if got == nil || *got != "-5.00" {
		t.Fatal(got)
	}
}

func TestRegionalPriceStorage(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	q := dbgen.New(pool)
	for _, fixture := range []struct {
		file  string
		parse func([]byte, string, time.Time) ([]market.RegionalQuote, error)
	}{{"national", market.ParseNational}, {"suzhou", market.ParseSuzhou}, {"fuzhou", market.ParseFuzhou}} {
		body, err := os.ReadFile("../market/testdata/" + fixture.file + ".html")
		if err != nil {
			t.Fatal(err)
		}
		rows, err := fixture.parse(body, "https://example.com/"+fixture.file, time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC))
		if err != nil {
			t.Fatal(err)
		}
		for pass := 0; pass < 2; pass++ {
			for _, r := range rows {
				var price pgtype.Numeric
				_ = price.Scan(r.Price)
				_, err = q.UpsertRegionalPrice(ctx, dbgen.UpsertRegionalPriceParams{CityCode: r.City, City: r.City, Ingredient: r.Name, Original: r.Original, Price: price, Unit: r.Unit, Agency: r.Agency, SourceUrl: r.URL, Specification: r.Spec, Day: pgtype.Date{Time: r.End, Valid: true}, PeriodStart: pgtype.Date{Time: r.Start, Valid: true}})
				if err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	var cities int
	err := pool.QueryRow(ctx, "SELECT count(DISTINCT city_name) FROM market_benchmark_prices WHERE source_url LIKE 'https://example.com/%'").Scan(&cities)
	if err != nil || cities != 37 {
		t.Fatalf("cities %d %v", cities, err)
	}
	var duplicates int
	err = pool.QueryRow(ctx, "SELECT count(*) FROM (SELECT city_code,ingredient_catalog_id,source_agency,source_item_name,specification,price_type,unit,recorded_date FROM market_benchmark_prices GROUP BY 1,2,3,4,5,6,7,8 HAVING count(*)>1) x").Scan(&duplicates)
	if err != nil || duplicates != 0 {
		t.Fatal(duplicates, err)
	}
	rows, err := q.ListBenchmarkPricesByCity(ctx, dbgen.ListBenchmarkPricesByCityParams{CityName: "福州市"})
	if err != nil || len(rows) < 25 {
		t.Fatal(len(rows), err)
	}
	for _, r := range rows {
		if r.CityName != "福州市" {
			t.Fatal("city leakage")
		}
	}
}
func TestDailyPriceStatus(t *testing.T) {
	today := priceToday()
	if !isTodayPrice(today, today, today) {
		t.Fatal("today missing")
	}
	for _, dates := range [][2]time.Time{{today.AddDate(0, 0, -1), today.AddDate(0, 0, -1)}, {today, today.AddDate(0, 0, -30)}, {today.AddDate(0, 0, 1), today.AddDate(0, 0, 1)}} {
		if isTodayPrice(dates[0], dates[1], today) {
			t.Fatal("non-daily or non-current data called today")
		}
	}
}
func TestMarketPricesIntegration(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	home := func() (string, string) {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "prices"})
		must(t, code, 200, v)
		token := get(v, "token")
		code, v = h.call("POST", "/households", token, "", map[string]any{"name": "prices", "servings": 2})
		must(t, code, 201, v)
		return token, "/households/" + get(v, "id") + "/prices"
	}
	token, root := home()
	other, _ := home()
	for _, city := range []string{"北京市", "上海市", "广州市", "深圳市", "成都市"} {
		code, rows := h.list(root+"/benchmarks?city="+url.QueryEscape(city), token)
		if code != 200 || len(rows) != 20 {
			t.Fatalf("seed %s %d %d", city, code, len(rows))
		}
		for _, r := range rows {
			if r["historical"] != true || r["recorded_date"] != "2026-06-30" {
				t.Fatal(r)
			}
		}
	}
	code, _ := h.list(root+"/benchmarks?city=x", other)
	if code != 404 {
		t.Fatal(code)
	}
	dashboard := root + "/dashboard?" + url.Values{"city": {"北京市"}, "ingredient_name": {"西红柿"}}.Encode()
	code, v := h.call("GET", dashboard, token, "", nil)
	must(t, code, 200, v)
	if _, ok := v["community"]; ok {
		t.Fatal("removed community data exposed")
	}
	if len(v["benchmarks"].([]any)) != 1 {
		t.Fatal("alias failed")
	}
	req, _ := http.NewRequest("POST", server.URL+"/api"+root+"/records", strings.NewReader("{}"))
	req.Header.Set("Authorization", "Bearer "+token)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 404 {
		t.Fatal(res.StatusCode)
	}
	ctx := context.Background()
	q := dbgen.New(pool)
	var price pgtype.Numeric
	_ = price.Scan("4.75")
	arg := dbgen.UpsertOfficialPriceParams{Price: price, SourceItem: "西红柿", Specification: "测试超市零售", Ingredient: "番茄", Day: pgtype.Date{Time: priceToday(), Valid: true}}
	for i := 0; i < 2; i++ {
		n, err := q.UpsertOfficialPrice(ctx, arg)
		if err != nil || n != 1 {
			t.Fatalf("%d %v", n, err)
		}
	}
	code, v = h.call("GET", dashboard, token, "", nil)
	must(t, code, 200, v)
	current := 0
	for _, b := range v["benchmarks"].([]any) {
		m := b.(map[string]any)
		if m["historical"] == false {
			current++
			if m["price"] != "4.75" {
				t.Fatal(m)
			}
		}
	}
	if current != 1 {
		t.Fatalf("replayed price duplicated: %d", current)
	}
}
