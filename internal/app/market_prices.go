package app

import (
	"fmt"
	"math/big"
	"strconv"
	"strings"
	"time"

	"foodflow/internal/dbgen"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

func priceToday() time.Time {
	now := time.Now().In(time.FixedZone("Asia/Shanghai", 8*3600))
	day, _ := time.Parse("2006-01-02", now.Format("2006-01-02"))
	return day
}
func numericText(n pgtype.Numeric) string {
	v, e := n.Value()
	if e != nil || v == nil {
		return ""
	}
	return fmt.Sprint(v)
}
func priceChange(current, previous string) *string {
	a, ok := new(big.Rat).SetString(current)
	if !ok {
		return nil
	}
	b, ok := new(big.Rat).SetString(previous)
	if !ok || b.Sign() <= 0 {
		return nil
	}
	delta := new(big.Rat).Sub(a, b)
	delta.Quo(delta, b)
	delta.Mul(delta, big.NewRat(100, 1))
	s := delta.FloatString(2)
	return &s
}
func benchmarkJSON(b dbgen.MarketBenchmarkPrice) gin.H {
	return gin.H{"id": strconv.FormatInt(b.ID, 10), "ingredient_name": b.IngredientName, "category": b.Category, "city": b.CityName,
		"price": numericText(b.Price), "unit": b.Unit, "source_agency": b.SourceAgency, "source_url": b.SourceUrl,
		"source_item_name": b.SourceItemName, "specification": b.Specification, "price_type": b.PriceType,
		"recorded_date": b.RecordedDate.Time.Format("2006-01-02"), "period_start": b.PeriodStart.Time.Format("2006-01-02"),
		"collected_at": b.CollectedAt.Time, "historical": !isTodayPrice(b.RecordedDate.Time, b.PeriodStart.Time, priceToday())}
}
func validPriceCity(city string) bool { return len([]rune(city)) > 0 && len([]rune(city)) <= 32 }

func (a *App) benchmarks(c *gin.Context) {
	city := strings.TrimSpace(c.Query("city"))
	category := strings.TrimSpace(c.Query("category"))
	if !validPriceCity(city) || len([]rune(category)) > 32 {
		fail(c, 400, "请选择有效城市")
		return
	}
	rows, e := dbgen.New(a.DB).ListBenchmarkPricesByCity(c, dbgen.ListBenchmarkPricesByCityParams{CityName: city, Column2: category})
	if e != nil {
		fail(c, 500, "官方基准读取失败")
		return
	}
	out := []gin.H{}
	for _, row := range rows {
		out = append(out, benchmarkJSON(row))
	}
	c.JSON(200, out)
}

func (a *App) priceDashboard(c *gin.Context) {
	city := strings.TrimSpace(c.Query("city"))
	name := strings.TrimSpace(c.Query("ingredient_name"))
	if !validPriceCity(city) || len([]rune(name)) == 0 || len([]rune(name)) > 64 {
		fail(c, 400, "请选择城市与食材")
		return
	}
	q := dbgen.New(a.DB)
	ingredient, e := q.FindPriceIngredient(c, name)
	if e == pgx.ErrNoRows {
		fail(c, 400, "食材不在目录中")
		return
	}
	if e != nil {
		fail(c, 500, "食材查询失败")
		return
	}
	rows, e := q.GetLatestBenchmarkPrice(c, dbgen.GetLatestBenchmarkPriceParams{CityName: city, IngredientName: ingredient.Name})
	if e != nil {
		fail(c, 500, "官方基准读取失败")
		return
	}
	out := []gin.H{}
	for _, r := range rows {
		b := dbgen.MarketBenchmarkPrice{ID: r.ID, CityCode: r.CityCode, CityName: r.CityName, IngredientCatalogID: r.IngredientCatalogID, IngredientName: r.IngredientName, Category: r.Category, Price: r.Price, Unit: r.Unit, SourceAgency: r.SourceAgency, SourceUrl: r.SourceUrl, SourceItemName: r.SourceItemName, Specification: r.Specification, PriceType: r.PriceType, RecordedDate: r.RecordedDate, PeriodStart: r.PeriodStart, CollectedAt: r.CollectedAt}
		item := benchmarkJSON(b)
		item["weekly_change_percent"] = priceChange(numericText(r.Price), r.PreviousPrice)
		out = append(out, item)
	}
	c.JSON(200, gin.H{"benchmarks": out, "as_of": priceToday().Format("2006-01-02")})
}

func isTodayPrice(end, start, today time.Time) bool {
	return end.Equal(today) && start.Equal(today)
}
