package core

import (
	"fmt"
	"sort"
)

type PantryItem struct {
	Name, Unit string
	Quantity   int64
	Selected   bool
}

type RecipeRequirement struct {
	Name, Unit string
	Quantity   int64
}

type RecipeCandidate struct {
	ID, Title         string
	Servings, Minutes int
	Tags              []string
	Requires          []RecipeRequirement
}

type RequirementCoverage struct {
	Name, Unit                          string
	Needed, SelectedAvailable, Shortage int64
	UnitUncertain                       bool
}

type RecipeOption struct {
	Recipe        RecipeCandidate
	Coverage      []RequirementCoverage
	Matched       int
	Missing       int
	UnitUncertain bool
	Status        string
}

func canonicalUnit(unit string) (string, error) {
	dim, _, e := Dimension(unit)
	if e != nil {
		return "", e
	}
	if dim == "mass" {
		return "g", nil
	}
	if dim == "volume" {
		return "ml", nil
	}
	return unit, nil
}

// EvaluateRecipe treats the selected pantry items as the ingredients the user
// wants to use. Other household stock is intentionally not counted as selected
// availability, but incompatible units are still surfaced for later confirmation.
func EvaluateRecipe(recipe RecipeCandidate, pantry []PantryItem, servings int) (RecipeOption, error) {
	out := RecipeOption{Recipe: recipe, Coverage: []RequirementCoverage{}}
	merged := map[string]*RequirementCoverage{}
	for _, need := range recipe.Requires {
		unit, e := canonicalUnit(need.Unit)
		if e != nil {
			return out, e
		}
		q, e := Scale(need.Quantity, servings, recipe.Servings)
		if e != nil {
			return out, e
		}
		q, e = Convert(q, need.Unit, unit)
		if e != nil {
			return out, e
		}
		key := need.Name + "\x00" + unit
		if merged[key] == nil {
			merged[key] = &RequirementCoverage{Name: need.Name, Unit: unit}
		}
		if q > int64(^uint64(0)>>1)-merged[key].Needed {
			return out, fmt.Errorf("recipe quantity overflow")
		}
		merged[key].Needed += q
	}
	for _, cover := range merged {
		matched := false
		for _, item := range pantry {
			if item.Name != cover.Name || item.Quantity <= 0 {
				continue
			}
			if item.Selected {
				matched = true
			}
			q, e := Convert(item.Quantity, item.Unit, cover.Unit)
			if e != nil {
				cover.UnitUncertain = true
				continue
			}
			if item.Selected {
				if q > int64(^uint64(0)>>1)-cover.SelectedAvailable {
					return out, fmt.Errorf("pantry quantity overflow")
				}
				cover.SelectedAvailable += q
			}
		}
		if matched {
			out.Matched++
		}
		if cover.SelectedAvailable < cover.Needed {
			cover.Shortage = cover.Needed - cover.SelectedAvailable
			out.Missing++
		}
		if cover.UnitUncertain {
			out.UnitUncertain = true
		}
		out.Coverage = append(out.Coverage, *cover)
	}
	sort.Slice(out.Coverage, func(i, j int) bool {
		return out.Coverage[i].Name+out.Coverage[i].Unit < out.Coverage[j].Name+out.Coverage[j].Unit
	})
	switch {
	case out.UnitUncertain:
		out.Status = "unit_confirmation"
	case out.Missing == 0:
		out.Status = "ready"
	case out.Missing == 1:
		out.Status = "one_missing"
	case out.Missing == 2:
		out.Status = "two_missing"
	default:
		out.Status = "more_missing"
	}
	return out, nil
}

func SortRecipeOptions(options []RecipeOption) {
	rank := map[string]int{"ready": 0, "one_missing": 1, "two_missing": 2, "more_missing": 3, "unit_confirmation": 4}
	sort.Slice(options, func(i, j int) bool {
		a, b := options[i], options[j]
		if rank[a.Status] != rank[b.Status] {
			return rank[a.Status] < rank[b.Status]
		}
		if a.Missing != b.Missing {
			return a.Missing < b.Missing
		}
		if a.Matched != b.Matched {
			return a.Matched > b.Matched
		}
		if a.Recipe.Minutes != b.Recipe.Minutes {
			return a.Recipe.Minutes < b.Recipe.Minutes
		}
		return a.Recipe.Title < b.Recipe.Title
	})
}
