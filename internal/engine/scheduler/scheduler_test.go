package scheduler

import (
	"testing"
	"time"
)

var now = time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)

func TestSharedIngredientConflict(t *testing.T) {
	recipes := []Recipe{
		{ID: "番茄炒蛋", Servings: 2, Minutes: 15, Needs: []Need{{Ingredient: 1, Quantity: 2000}}},
		{ID: "蛋花汤", Servings: 2, Minutes: 10, Needs: []Need{{Ingredient: 1, Quantity: 2000}}},
	}
	batches := []Batch{{ID: "eggs", Ingredient: 1, Quantity: 3000, ExpiresAt: now.Add(24 * time.Hour), Usable: true}}
	plan, err := Schedule(recipes, batches, Request{Servings: 2, MaxMinutes: 30, Selected: []int{1}, AsOf: now})
	if err != nil || len(plan.RecipeIDs) != 1 || len(plan.Conflicts) != 1 {
		t.Fatalf("expected one recipe and a conflict, got %+v, %v", plan, err)
	}
	used := int64(0)
	for _, allocation := range plan.Allocations {
		used += allocation.Quantity
	}
	if used != 2000 {
		t.Fatalf("stock overallocated: %d", used)
	}
}

func TestFEFOSplitAndExpiredExcluded(t *testing.T) {
	recipe := Recipe{ID: "omelette", Servings: 2, Minutes: 10, Needs: []Need{{Ingredient: 1, Quantity: 3000}}}
	batches := []Batch{
		{ID: "later", Ingredient: 1, Quantity: 2000, ExpiresAt: now.Add(72 * time.Hour), Usable: true},
		{ID: "expired", Ingredient: 1, Quantity: 9000, ExpiresAt: now.Add(-time.Hour), Usable: true},
		{ID: "soon", Ingredient: 1, Quantity: 1000, ExpiresAt: now.Add(12 * time.Hour), Usable: true},
		{ID: "spoiled", Ingredient: 1, Quantity: 9000, ExpiresAt: now.Add(6 * time.Hour), Usable: false},
	}
	plan, err := Schedule([]Recipe{recipe}, batches, Request{Servings: 2, MaxMinutes: 10, AsOf: now})
	if err != nil || len(plan.Allocations) != 2 {
		t.Fatalf("split failed: %+v, %v", plan, err)
	}
	if plan.Allocations[0].BatchID != "soon" || plan.Allocations[0].Quantity != 1000 || plan.Allocations[1].BatchID != "later" || plan.Allocations[1].Quantity != 2000 {
		t.Fatalf("wrong FEFO order: %+v", plan.Allocations)
	}
}

func TestBranchAndBoundChoosesBetterCombination(t *testing.T) {
	recipes := []Recipe{
		{ID: "large", Servings: 2, Minutes: 30, Needs: []Need{{Ingredient: 1, Quantity: 4000}}},
		{ID: "small-a", Servings: 2, Minutes: 12, Needs: []Need{{Ingredient: 1, Quantity: 2000}}},
		{ID: "small-b", Servings: 2, Minutes: 12, Needs: []Need{{Ingredient: 1, Quantity: 2000}}},
	}
	batches := []Batch{{ID: "near", Ingredient: 1, Quantity: 4000, ExpiresAt: now.Add(time.Hour), Usable: true}}
	plan, err := Schedule(recipes, batches, Request{Servings: 2, MaxMinutes: 30, AsOf: now})
	if err != nil || len(plan.RecipeIDs) != 2 || plan.Minutes != 24 {
		t.Fatalf("expected the two short recipes: %+v, %v", plan, err)
	}
}

func TestServingScaleAndNodeLimit(t *testing.T) {
	recipes := []Recipe{{ID: "egg", Servings: 2, Minutes: 5, Needs: []Need{{Ingredient: 1, Quantity: 2000}}}}
	batches := []Batch{{ID: "lot", Ingredient: 1, Quantity: 3000, ExpiresAt: now.Add(time.Hour), Usable: true}}
	plan, err := Schedule(recipes, batches, Request{Servings: 4, MaxMinutes: 5, AsOf: now})
	if err != nil || len(plan.RecipeIDs) != 0 {
		t.Fatalf("servings ignored: %+v, %v", plan, err)
	}
	plan, err = Schedule(recipes, batches, Request{Servings: 2, MaxMinutes: 5, AsOf: now, MaxNodes: 1})
	if err != nil || !plan.Truncated {
		t.Fatalf("node limit not reported: %+v, %v", plan, err)
	}
}

func BenchmarkSchedule(b *testing.B) {
	recipes := make([]Recipe, 12)
	batches := make([]Batch, 12)
	for i := range recipes {
		recipes[i] = Recipe{ID: string(rune('A' + i)), Servings: 2, Minutes: 10 + i, Needs: []Need{{Ingredient: i % 4, Quantity: 1000}}}
		batches[i] = Batch{ID: string(rune('a' + i)), Ingredient: i % 4, Quantity: 1000, ExpiresAt: now.Add(time.Duration(i+1) * time.Hour), Usable: true}
	}
	req := Request{Servings: 2, MaxMinutes: 60, AsOf: now}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Schedule(recipes, batches, req); err != nil {
			b.Fatal(err)
		}
	}
}
