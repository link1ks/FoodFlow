package market

import (
	"testing"
	"time"
)

func TestOfficialFeeds(t *testing.T) {
	now := time.Date(2026, 9, 26, 0, 0, 0, 0, time.UTC)
	raw := []byte(`{"code":200,"data":{"exeNo":"2026-09-25","data":[["西红柿"],["52.31"],["1.65"],["2.83"],["2.96"]]}}`)
	rows, err := ParseVegetables(raw, now)
	if err != nil || len(rows) != 1 || rows[0].Name != "番茄" || rows[0].Price != "2.96" || rows[0].Day.Format("2006-01-02") != "2026-09-25" {
		t.Fatalf("%+v %v", rows, err)
	}
	for _, bad := range []string{
		`{"code":200,"data":{"exeNo":"2026-09-25","data":[["番茄"],[],[],[],[]]}}`,
		`{"code":200,"data":{"exeNo":"2026-09-27","data":[["番茄"],["1"],["1"],["1"],["1"]]}}`,
		`{"code":200,"data":{"exeNo":"2026-09-25","data":[["番茄"],["1"],["1"],["1"],["-1"]]}}`,
	} {
		if _, err := ParseVegetables([]byte(bad), now); err == nil {
			t.Fatal("accepted invalid feed")
		}
	}
	rows, err = ParseStaples([]byte(`{"oilList":[{"ItemName":"油","ItemUnit":"元／5升","price04":50}],"meatList":[{"ItemName":"鸡蛋","ItemUnit":"元／500克","ItemLevel":"新鲜完整","PriceDate":"2026-09-25","price04":6.5},{"ItemName":"牛肉","ItemUnit":"元／500克","price04":null},{"ItemName":"猪肉","ItemUnit":"元／500克","price04":0}]}`), now)
	if err != nil || len(rows) != 1 || rows[0].Price != "6.50" {
		t.Fatalf("%+v %v", rows, err)
	}
}
