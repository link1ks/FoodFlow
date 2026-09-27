package matcher

import (
	"fmt"
	"testing"
)

func TestMultiwordMatchAndMissingCounts(t *testing.T) {
	keys := make([]string, 130)
	for i := range keys {
		keys[i] = fmt.Sprintf("ingredient-%03d", i)
	}
	idx, err := New([]Recipe{
		{ID: "ready", Ingredients: []string{keys[0], keys[64], keys[129]}},
		{ID: "one", Ingredients: []string{keys[0], keys[65]}},
		{ID: "two", Ingredients: []string{keys[0], keys[65], keys[128]}},
		{ID: "three", Ingredients: []string{keys[0], keys[65], keys[128], keys[127]}},
	})
	if err != nil || idx.IngredientCount() != 6 {
		t.Fatalf("index: %v, count=%d", err, idx.IngredientCount())
	}
	// Add other recipes so the registered IDs cross both 64-bit boundaries.
	all := make([]Recipe, 0, 131)
	for i, key := range keys {
		all = append(all, Recipe{ID: fmt.Sprintf("single-%d", i), Ingredients: []string{key}})
	}
	all = append(all, Recipe{ID: "across", Ingredients: []string{keys[0], keys[64], keys[129]}})
	idx, err = New(all)
	if err != nil || idx.IngredientCount() != 130 {
		t.Fatalf("multiword index: %v", err)
	}
	selection := idx.Select([]string{keys[0], keys[64], "unknown", keys[0]})
	results := idx.MatchInto(selection, 1, make([]Result, 0, len(all)))
	if got := results[len(results)-1]; got.RecipeID != "across" || got.Missing != 1 || got.Matched != 2 {
		t.Fatalf("cross-word result: %+v", got)
	}
	selection = idx.Select([]string{keys[0], keys[64], keys[129]})
	results = idx.MatchInto(selection, 0, results[:0])
	if got := results[len(results)-1]; got.RecipeID != "across" || got.Missing != 0 {
		t.Fatalf("subset result: %+v", got)
	}
}

func TestMatchIntoZeroAlloc(t *testing.T) {
	idx, err := New([]Recipe{{ID: "one", Ingredients: []string{"番茄", "鸡蛋"}}, {ID: "two", Ingredients: []string{"番茄", "蒜"}}})
	if err != nil {
		t.Fatal(err)
	}
	selection := idx.Select([]string{"番茄", "鸡蛋"})
	buffer := make([]Result, 0, idx.RecipeCount())
	allocations := testing.AllocsPerRun(1000, func() { buffer = idx.MatchInto(selection, 2, buffer[:0]) })
	if allocations != 0 {
		t.Fatalf("hot path allocated %.2f times", allocations)
	}
}

func TestInvalidRecipes(t *testing.T) {
	if _, err := New([]Recipe{{ID: "r", Ingredients: []string{"x"}}, {ID: "r", Ingredients: []string{"y"}}}); err == nil {
		t.Fatal("duplicate recipe accepted")
	}
	if _, err := New([]Recipe{{ID: "r", Ingredients: []string{""}}}); err == nil {
		t.Fatal("empty ingredient accepted")
	}
}

func BenchmarkMatchInto(b *testing.B) {
	keys := make([]string, 128)
	for i := range keys {
		keys[i] = fmt.Sprintf("item-%d", i)
	}
	recipes := make([]Recipe, 100)
	for i := range recipes {
		recipes[i] = Recipe{ID: fmt.Sprintf("r-%d", i), Ingredients: []string{keys[i%128], keys[(i+7)%128], keys[(i+67)%128]}}
	}
	idx, err := New(recipes)
	if err != nil {
		b.Fatal(err)
	}
	selection := idx.Select(keys[:64])
	buffer := make([]Result, 0, len(recipes))
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		buffer = idx.MatchInto(selection, 2, buffer[:0])
	}
}
