package core

import (
	"math"
	"testing"
)

func TestRecipeDependenciesAndExactUnits(t *testing.T) {
	recipe := RecipeCandidate{ID: "tomato-eggs", Title: "番茄炒蛋", Servings: 2, Minutes: 15, Requires: []RecipeRequirement{{Name: "番茄", Unit: "g", Quantity: 300000}, {Name: "鸡蛋", Unit: "个", Quantity: 2000}}}
	pantry := []PantryItem{{Name: "番茄", Unit: "kg", Quantity: 1000, Selected: true}, {Name: "鸡蛋", Unit: "个", Quantity: 2000, Selected: false}}
	option, e := EvaluateRecipe(recipe, pantry, 2)
	if e != nil || option.Status != "one_missing" || option.Matched != 1 || option.Missing != 1 {
		t.Fatalf("unexpected partial option %+v: %v", option, e)
	}
	pantry[1].Selected = true
	option, e = EvaluateRecipe(recipe, pantry, 2)
	if e != nil || option.Status != "ready" || option.Missing != 0 {
		t.Fatalf("selected inventory not ready %+v: %v", option, e)
	}
	option, e = EvaluateRecipe(recipe, pantry, 4)
	if e != nil || option.Missing != 1 {
		t.Fatalf("servings not rescaled %+v: %v", option, e)
	}
	pantry[1].Unit = "g"
	option, e = EvaluateRecipe(recipe, pantry, 2)
	if e != nil || option.Status != "unit_confirmation" || !option.UnitUncertain {
		t.Fatalf("incompatible units treated as exact %+v: %v", option, e)
	}
	if _, e = Scale(math.MaxInt64, 20, 2); e == nil {
		t.Fatal("quantity overflow accepted")
	}
}

func TestRecipeOptionOrder(t *testing.T) {
	options := []RecipeOption{{Recipe: RecipeCandidate{Title: "uncertain"}, Status: "unit_confirmation"}, {Recipe: RecipeCandidate{Title: "ready"}, Status: "ready"}, {Recipe: RecipeCandidate{Title: "one"}, Status: "one_missing"}}
	SortRecipeOptions(options)
	if options[0].Recipe.Title != "ready" || options[1].Recipe.Title != "one" || options[2].Recipe.Title != "uncertain" {
		t.Fatalf("wrong dependency order: %+v", options)
	}
}
