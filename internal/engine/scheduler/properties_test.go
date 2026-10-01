package scheduler

import (
	"foodflow/internal/core"
	"testing"
	"time"
)

func FuzzScheduleStockConservation(f *testing.F) {
	f.Add(uint32(100), uint32(200), uint32(150), uint8(2))
	f.Add(uint32(1), uint32(1), uint32(1000), uint8(20))
	f.Fuzz(func(t *testing.T, a, b, needRaw uint32, people uint8) {
		first, second, need := int64(a%1000000+1), int64(b%1000000+1), int64(needRaw%1000000+1)
		servings := int(people%20) + 1
		recipes := []Recipe{{ID: "meal", Servings: 2, Minutes: 10, Needs: []Need{{Ingredient: 1, Quantity: need}}}}
		batches := []Batch{{ID: "first", Ingredient: 1, Quantity: first, ExpiresAt: now.Add(time.Hour), Usable: true}, {ID: "second", Ingredient: 1, Quantity: second, ExpiresAt: now.Add(2 * time.Hour), Usable: true}, {ID: "expired", Ingredient: 1, Quantity: 10000000, ExpiresAt: now.Add(-time.Hour), Usable: true}, {ID: "spoiled", Ingredient: 1, Quantity: 10000000, Usable: false}}
		// Selected ingredients give feasible recipes positive coverage; otherwise
		// the scoring policy may correctly prefer an empty plan for tiny demand.
		plan, err := Schedule(recipes, batches, Request{Selected: []int{1}, Servings: servings, MaxMinutes: 10, AsOf: now})
		if err != nil {
			t.Fatal(err)
		}
		scaled, err := core.Scale(need, servings, 2)
		if err != nil {
			t.Fatal(err)
		}
		used := map[string]int64{}
		var total int64
		for _, allocation := range plan.Allocations {
			if allocation.Quantity <= 0 || allocation.Ingredient != 1 || allocation.RecipeID != "meal" {
				t.Fatal("invalid allocation")
			}
			used[allocation.BatchID] += allocation.Quantity
			total += allocation.Quantity
		}
		if used["first"] > first || used["second"] > second || used["expired"] != 0 || used["spoiled"] != 0 {
			t.Fatal("overdraw or unusable stock allocated")
		}
		if len(plan.RecipeIDs) > 0 {
			if total != scaled || used["second"] > 0 && used["first"] != first {
				t.Fatal("demand conservation or FEFO violated")
			}
		} else if total != 0 || first+second >= scaled {
			t.Fatal("feasible stock rejected or phantom allocation")
		}
	})
}
