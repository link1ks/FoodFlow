package market

import (
	"os"
	"strings"
	"testing"
	"time"
)

func TestNationalReport(t *testing.T) {
	body, err := os.ReadFile("testdata/national.html")
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	rows, err := ParseNational(body, "https://www.chinaprice.cn/jsdzqk/61480.jhtml", now)
	if err != nil {
		t.Fatal(err)
	}
	cities := map[string]int{}
	foundPork, foundRibs, foundOil := false, false, false
	for _, r := range rows {
		cities[r.City]++
		if r.Start.Format("2006-01-02") != "2026-08-01" || r.End.Format("2006-01-02") != "2026-08-31" {
			t.Fatal(r)
		}
		if r.City == "北京市" {
			switch {
			case r.Name == "猪肉":
				foundPork = r.Price == "12.82"
			case r.Name == "排骨":
				foundRibs = r.Price == "27.73"
			case r.Original == "花生油":
				foundOil = r.Unit == "1l" && r.Price == "31.79"
			}
		}
	}
	if len(cities) != 36 || !foundPork || !foundRibs || !foundOil {
		t.Fatalf("cities=%d pork=%v ribs=%v oil=%v", len(cities), foundPork, foundRibs, foundOil)
	}
	for _, bad := range []string{strings.Replace(string(body), "colspan=\"2\"", "colspan=\"3\"", 1), strings.ReplaceAll(string(body), "拉萨市", "未知城市"), strings.ReplaceAll(string(body), "2026年8月", "2027年8月")} {
		if _, err := ParseNational([]byte(bad), "", now); err == nil {
			t.Fatal("invalid report accepted")
		}
	}
	t.Logf("%d observations from %d cities", len(rows), len(cities))
}

func TestSuzhouReport(t *testing.T) {
	body, err := os.ReadFile("testdata/suzhou.html")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseSuzhou(body, "https://fg.suzhou.gov.cn/example", time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	egg := false
	for _, r := range rows {
		if r.End.Format("2006-01-02") != "2026-09-26" || r.City != "苏州市" {
			t.Fatal(r)
		}
		if strings.Contains(r.Original, "装") {
			t.Fatal("package treated as 500g", r)
		}
		if r.Name == "鸡蛋" {
			egg = true
		}
	}
	if !egg {
		t.Fatal("egg missing")
	}
	if _, err := ParseSuzhou([]byte(strings.ReplaceAll(string(body), "元/500g", "元/公斤")), "", time.Now()); err == nil {
		t.Fatal("unit mismatch accepted")
	}
}

func TestOfficialLinkDiscovery(t *testing.T) {
	got, err := nationalURL([]byte(`<a onclick="go('http://www.chinaprice.cn/jsdzqk/61480.jhtml')">中国市场价格监测报告2026.9-2</a>`))
	if err != nil || got != "https://www.chinaprice.cn/jsdzqk/61480.jhtml" {
		t.Fatal(got, err)
	}
	_, err = suzhouURL([]byte(`<a href="https://evil.example/szfgw/scdt/x">苏州市部分农贸市场零售均价(2026年9月26日)</a>`))
	if err == nil {
		t.Fatal("untrusted host accepted")
	}
}

func TestFuzhouReport(t *testing.T) {
	body, err := os.ReadFile("testdata/fuzhou.html")
	if err != nil {
		t.Fatal(err)
	}
	rows, err := ParseFuzhou(body, "https://fgw.fuzhou.gov.cn/test", time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	rice, oil := false, false
	for _, r := range rows {
		if r.City != "福州市" || r.End.Format("2006-01-02") != "2026-09-26" {
			t.Fatal(r)
		}
		if r.Original == "晚籼米" {
			rice = r.Price == "2.68" && r.Unit == "500g"
		}
		if r.Original == "花生油" {
			oil = r.Unit == "1l"
		}
	}
	if !rice || !oil {
		t.Fatal("category rowspans shifted values")
	}
}
