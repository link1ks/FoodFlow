// Package scheduler finds a feasible combination of recipes from an in-memory
// inventory snapshot. It never changes persistent stock; the eventual cooking
// transaction must read and lock current batches again.
package scheduler

import (
	"errors"
	"fmt"
	"math"
	"sort"
	"time"

	"foodflow/internal/core"
)

// Ingredient IDs represent canonical ingredient AND unit. The caller must
// convert compatible units to the same milli-unit and reject uncertain units.
type Need struct {
	Ingredient int
	Quantity   int64
}

type Recipe struct {
	ID       string
	Servings int
	Minutes  int
	Needs    []Need
}

type Batch struct {
	ID         string
	Ingredient int
	Quantity   int64
	ExpiresAt  time.Time // zero means unknown expiry and is scheduled last
	Usable     bool      // false for spoiled or otherwise unavailable stock
}

type Weights struct {
	Urgency  float64
	Coverage float64
	Time     float64
}

type Request struct {
	Servings   int
	MaxMinutes int // total sequential cooking time
	Selected   []int
	AsOf       time.Time
	MaxNodes   int
	Weights    Weights
}

type Allocation struct {
	RecipeID   string
	BatchID    string
	Ingredient int
	Quantity   int64
	batchIndex int
}

type Conflict struct {
	RecipeA, RecipeB string
	Ingredient       int
}

type Plan struct {
	RecipeIDs   []string
	Allocations []Allocation
	Conflicts   []Conflict
	Minutes     int
	Score       float64
	Truncated   bool // node budget reached; result remains feasible
}

type candidate struct {
	recipe     Recipe
	needs      []Need
	coverage   float64
	upperScore float64
}

// Schedule scales recipe BOM quantities to the requested servings, builds
// per-ingredient expiry min-heaps, then uses branch-and-bound. At most 20
// candidate recipes are accepted to keep the search bounded.
func Schedule(recipes []Recipe, batches []Batch, req Request) (Plan, error) {
	if req.Servings < 1 || req.Servings > 20 || req.MaxMinutes < 1 || len(recipes) > 20 || req.AsOf.IsZero() {
		return Plan{}, errors.New("invalid scheduling bounds")
	}
	if req.MaxNodes == 0 {
		req.MaxNodes = 200_000
	}
	if req.MaxNodes < 1 {
		return Plan{}, errors.New("invalid node budget")
	}
	if req.Weights == (Weights{}) {
		req.Weights = Weights{Urgency: 100, Coverage: 10, Time: 0.1}
	}
	if req.Weights.Urgency < 0 || req.Weights.Coverage < 0 || req.Weights.Time < 0 ||
		math.IsNaN(req.Weights.Urgency) || math.IsNaN(req.Weights.Coverage) || math.IsNaN(req.Weights.Time) ||
		math.IsInf(req.Weights.Urgency, 0) || math.IsInf(req.Weights.Coverage, 0) || math.IsInf(req.Weights.Time, 0) {
		return Plan{}, errors.New("invalid scoring weights")
	}
	selected := make(map[int]bool, len(req.Selected))
	for _, id := range req.Selected {
		selected[id] = true
	}
	usable := make([]Batch, 0, len(batches))
	groups := make(map[int][]int)
	seenBatch := make(map[string]bool, len(batches))
	for _, batch := range batches {
		if batch.Quantity < 0 || batch.Ingredient < 0 || batch.ID == "" {
			return Plan{}, errors.New("invalid batch")
		}
		if seenBatch[batch.ID] {
			return Plan{}, fmt.Errorf("duplicate batch %s", batch.ID)
		}
		seenBatch[batch.ID] = true
		if !batch.Usable || batch.Quantity == 0 || (!batch.ExpiresAt.IsZero() && !batch.ExpiresAt.After(req.AsOf)) {
			continue
		}
		index := len(usable)
		usable = append(usable, batch)
		groups[batch.Ingredient] = append(groups[batch.Ingredient], index)
	}
	// Heap extraction gives stable FEFO order within each ingredient. The DFS
	// then uses that order while tracking remaining amounts in its own state.
	for id, indices := range groups {
		groups[id] = expiryOrder(indices, usable)
	}
	remaining := make([]int64, len(usable))
	for i, b := range usable {
		remaining[i] = b.Quantity
	}
	available := make(map[int]int64, len(groups))
	urgent := make(map[int]int64, len(groups))
	for _, b := range usable {
		if available[b.Ingredient] > math.MaxInt64-b.Quantity {
			return Plan{}, errors.New("stock quantity overflow")
		}
		available[b.Ingredient] += b.Quantity
		if !b.ExpiresAt.IsZero() && b.ExpiresAt.Sub(req.AsOf) <= 48*time.Hour {
			if urgent[b.Ingredient] > math.MaxInt64-b.Quantity {
				return Plan{}, errors.New("urgent quantity overflow")
			}
			urgent[b.Ingredient] += b.Quantity
		}
	}
	candidates := make([]candidate, 0, len(recipes))
	seen := make(map[string]bool, len(recipes))
	for _, recipe := range recipes {
		if recipe.ID == "" || seen[recipe.ID] || recipe.Servings < 1 || recipe.Minutes < 1 || len(recipe.Needs) == 0 {
			return Plan{}, fmt.Errorf("invalid or duplicate recipe %q", recipe.ID)
		}
		seen[recipe.ID] = true
		if recipe.Minutes > req.MaxMinutes {
			continue
		}
		c := candidate{recipe: recipe, needs: make([]Need, 0, len(recipe.Needs))}
		seenNeed := make(map[int]bool, len(recipe.Needs))
		matched := 0
		urgencyBound := 0.0
		feasible := true
		for _, need := range recipe.Needs {
			if need.Ingredient < 0 || need.Quantity <= 0 || seenNeed[need.Ingredient] {
				return Plan{}, fmt.Errorf("invalid BOM for recipe %s", recipe.ID)
			}
			seenNeed[need.Ingredient] = true
			quantity, err := core.Scale(need.Quantity, req.Servings, recipe.Servings)
			if err != nil {
				return Plan{}, err
			}
			c.needs = append(c.needs, Need{Ingredient: need.Ingredient, Quantity: quantity})
			if available[need.Ingredient] < quantity {
				feasible = false
			}
			if selected[need.Ingredient] {
				matched++
			}
			if urgent[need.Ingredient] > 0 {
				urgencyBound += float64(min(urgent[need.Ingredient], quantity)) / float64(urgent[need.Ingredient])
			}
		}
		if !feasible {
			continue
		}
		c.coverage = float64(matched) / float64(len(c.needs))
		c.upperScore = req.Weights.Urgency*urgencyBound + req.Weights.Coverage*c.coverage - req.Weights.Time*float64(recipe.Minutes)
		candidates = append(candidates, c)
	}
	sort.Slice(candidates, func(i, j int) bool {
		a, b := candidates[i], candidates[j]
		av := a.upperScore / float64(a.recipe.Minutes)
		bv := b.upperScore / float64(b.recipe.Minutes)
		if av != bv {
			return av > bv
		}
		return a.recipe.ID < b.recipe.ID
	})
	plan := Plan{Conflicts: pairConflicts(candidates, available)}
	var chosen []string
	var allocations []Allocation
	nodes := 0
	var visit func(int, int, float64)
	visit = func(position, minutes int, score float64) {
		if nodes >= req.MaxNodes {
			plan.Truncated = true
			return
		}
		nodes++
		if score > plan.Score || (score == plan.Score && len(chosen) > len(plan.RecipeIDs)) {
			plan.Score = score
			plan.Minutes = minutes
			plan.RecipeIDs = append(plan.RecipeIDs[:0], chosen...)
			plan.Allocations = append(plan.Allocations[:0], allocations...)
		}
		if position == len(candidates) {
			return
		}
		// Sum of positive individual best-case gains is an admissible bound:
		// it ignores shared stock and time, so it can only overestimate.
		bound := score
		for i := position; i < len(candidates); i++ {
			if candidates[i].recipe.Minutes <= req.MaxMinutes-minutes && candidates[i].upperScore > 0 {
				bound += candidates[i].upperScore
			}
		}
		if bound < plan.Score {
			return
		}
		c := candidates[position]
		if c.recipe.Minutes <= req.MaxMinutes-minutes {
			mark := len(allocations)
			gain, ok := allocate(c, usable, groups, remaining, urgent, req, &allocations)
			if ok {
				chosen = append(chosen, c.recipe.ID)
				visit(position+1, minutes+c.recipe.Minutes, score+gain)
				chosen = chosen[:len(chosen)-1]
			}
			rollback(allocations[mark:], remaining)
			allocations = allocations[:mark]
		}
		visit(position+1, minutes, score)
	}
	visit(0, 0, 0)
	return plan, nil
}

func allocate(c candidate, batches []Batch, groups map[int][]int, remaining []int64, urgent map[int]int64, req Request, out *[]Allocation) (float64, bool) {
	urgency := 0.0
	for _, need := range c.needs {
		left := need.Quantity
		for _, index := range groups[need.Ingredient] {
			if left == 0 {
				break
			}
			use := min(left, remaining[index])
			if use == 0 {
				continue
			}
			remaining[index] -= use
			left -= use
			b := batches[index]
			*out = append(*out, Allocation{RecipeID: c.recipe.ID, BatchID: b.ID, Ingredient: need.Ingredient, Quantity: use, batchIndex: index})
			if !b.ExpiresAt.IsZero() && b.ExpiresAt.Sub(req.AsOf) <= 48*time.Hour && urgent[need.Ingredient] > 0 {
				remainingLife := b.ExpiresAt.Sub(req.AsOf)
				urgency += (float64(use) / float64(urgent[need.Ingredient])) * (1 - float64(remainingLife)/float64(48*time.Hour))
			}
		}
		if left != 0 {
			return 0, false
		}
	}
	return req.Weights.Urgency*urgency + req.Weights.Coverage*c.coverage - req.Weights.Time*float64(c.recipe.Minutes), true
}

func rollback(allocations []Allocation, remaining []int64) {
	for _, a := range allocations {
		remaining[a.batchIndex] += a.Quantity
	}
}

func pairConflicts(candidates []candidate, available map[int]int64) []Conflict {
	var conflicts []Conflict
	for i := range candidates {
		for j := i + 1; j < len(candidates); j++ {
			for _, a := range candidates[i].needs {
				for _, b := range candidates[j].needs {
					if a.Ingredient == b.Ingredient && a.Quantity > available[a.Ingredient]-b.Quantity {
						conflicts = append(conflicts, Conflict{RecipeA: candidates[i].recipe.ID, RecipeB: candidates[j].recipe.ID, Ingredient: a.Ingredient})
					}
				}
			}
		}
	}
	return conflicts
}
