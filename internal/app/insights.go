package app

import (
	"context"
	"encoding/json"
	"github.com/gin-gonic/gin"
	"io"
	"net/http"
	"net/url"
	"os"
	"time"
)

// Household membership is verified by the existing router middleware before
// this gateway calls the independently authenticated projection service.
func (a *App) kitchenInsights(c *gin.Context) {
	base := os.Getenv("INSIGHTS_URL")
	secret := os.Getenv("INSIGHTS_SERVICE_TOKEN")
	if base == "" || len(secret) < 32 {
		fail(c, 503, "厨房统计服务未启用")
		return
	}
	var zone string
	if err := a.DB.QueryRow(c, "SELECT timezone FROM households WHERE id=$1", hid(c)).Scan(&zone); err != nil {
		fail(c, 503, "家庭信息读取失败")
		return
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		fail(c, 503, "家庭时区无效")
		return
	}
	month := c.Query("month")
	if month == "" {
		month = time.Now().In(loc).Format("2006-01")
	}
	if _, err = time.Parse("2006-01", month); err != nil {
		fail(c, 400, "月份格式应为 YYYY-MM")
		return
	}
	endpoint, err := url.Parse(base)
	if err != nil || endpoint.Host == "" || endpoint.Scheme != "http" && endpoint.Scheme != "https" {
		fail(c, 503, "统计服务配置无效")
		return
	}
	endpoint.Path = "/v1/summary"
	endpoint.RawQuery = url.Values{"household_id": {hid(c)}, "month": {month}, "timezone": {zone}}.Encode()
	ctx, cancel := context.WithTimeout(c.Request.Context(), 3*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, "GET", endpoint.String(), nil)
	if err != nil {
		fail(c, 503, "统计服务不可用")
		return
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	client := http.Client{Timeout: 3 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	resp, err := client.Do(req)
	if err != nil {
		fail(c, 503, "统计服务暂不可用，请稍后重试")
		return
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20+1))
	if err != nil || resp.StatusCode != 200 || len(raw) > 1<<20 || !json.Valid(raw) {
		fail(c, 503, "统计服务暂不可用，请稍后重试")
		return
	}
	c.Data(200, "application/json", raw)
}
