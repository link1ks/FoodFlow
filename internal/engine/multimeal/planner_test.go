package multimeal

import "testing"

func fixture() Input {
	r := Recipe{Minutes: 15, Main: 0}
	r.Needs[0] = 300000
	return Input{Recipes: []Recipe{r}, Batches: []Batch{{Ingredient: 0, Quantity: 600000, Expires: 100000 + 36*3600}}, Times: []int64{100000, 100000 + 24*3600, 100000 + 48*3600}, Now: 100000, MaxMinutes: 30}
}
func TestRolloverExpiryAndConservation(t *testing.T) {
	in := fixture()
	out, err := Plan(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Meals[0].Used[0] != 300000 || out.Meals[0].Remaining[0] != 300000 || out.Meals[1].Used[0] != 300000 || out.Meals[2].Recipe != -1 {
		t.Fatalf("unexpected rollover %+v", out)
	}
	if in.Batches[0].Quantity != 600000 {
		t.Fatal("input mutated")
	}
	in.Times[1] = 100000 + 40*3600
	out, err = Plan(in)
	if err != nil {
		t.Fatal(err)
	}
	if out.Meals[1].Used[0] != 0 {
		t.Fatal("used expired batch")
	}
}
func TestRepeatedMainPenaltyAndBudget(t *testing.T) {
	in := fixture()
	in.Batches[0].Expires = 0
	r := Recipe{Minutes: 15, Main: 1}
	r.Needs[1] = 1000
	in.Recipes = append(in.Recipes, r)
	in.Batches = append(in.Batches, Batch{Ingredient: 1, Quantity: 2000})
	out, _ := Plan(in)
	if out.Meals[0].Recipe == out.Meals[1].Recipe || out.Meals[1].Recipe == out.Meals[2].Recipe {
		t.Fatal("repetition penalty ignored")
	}
	in.MaxNodes = 1
	out, _ = Plan(in)
	if !out.Truncated {
		t.Fatal("budget not reported")
	}
	for _, m := range out.Meals[:out.Count] {
		if m.Remaining[0] < 0 {
			t.Fatal("negative balance")
		}
	}
}
func TestEmptyAndValidation(t *testing.T) {
	in := fixture()
	in.Recipes = nil
	out, err := Plan(in)
	if err != nil || out.Meals[0].Remaining[0] != 600000 {
		t.Fatal("empty state lost")
	}
	in.Times[0] = in.Now - 1
	if _, err = Plan(in); err == nil {
		t.Fatal("past accepted")
	}
}
func BenchmarkPlan(b *testing.B) {
	in := fixture()
	Plan(in)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := Plan(in); err != nil {
			b.Fatal(err)
		}
	}
}
