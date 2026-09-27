// Package multimeal searches a bounded sequence of meals using only caller-owned snapshots.
package multimeal

import (
	"errors"
	"sync"
)

const (
	MaxMeals       = 7
	MaxRecipes     = 20
	MaxBatches     = 128
	MaxIngredients = 64
)

type Recipe struct {
	Minutes, Main int
	Needs         [MaxIngredients]int64
}
type Batch struct {
	Ingredient int
	Quantity   int64
	Expires    int64
}
type Input struct {
	Recipes              []Recipe
	Batches              []Batch
	Times                []int64
	Now                  int64
	MaxMinutes, MaxNodes int
}
type Meal struct {
	Recipe          int
	Used, Remaining [MaxBatches]int64
}
type Result struct {
	Meals        [MaxMeals]Meal
	Count, Nodes int
	Score        float64
	Truncated    bool
}
type workspace struct {
	input     Input
	remaining [MaxBatches]int64
	order     [MaxBatches]int
	recipes   [MaxRecipes]int
	upper     [MaxRecipes]float64
	path      [MaxMeals]Meal
	best      Result
	maxGain   float64
}

var pool = sync.Pool{New: func() any { return new(workspace) }}

// Plan uses FEFO allocation and branch-and-bound over one recipe per slot.
// The admissible bound ignores shared stock and repetition penalties. A node
// budget returns the best feasible sequence found, never an infeasible optimum.
// Fixed scratch arrays eliminate allocation inside the search tree.
func Plan(in Input) (Result, error) {
	if len(in.Times) < 1 || len(in.Times) > MaxMeals || len(in.Recipes) > MaxRecipes || len(in.Batches) > MaxBatches || in.Now <= 0 || in.MaxMinutes < 1 || in.MaxMinutes > 240 {
		return Result{}, errors.New("invalid planner bounds")
	}
	if in.MaxNodes == 0 {
		in.MaxNodes = 50000
	}
	if in.MaxNodes < 1 || in.MaxNodes > 200000 {
		return Result{}, errors.New("invalid node budget")
	}
	for i, t := range in.Times {
		if t < in.Now || (i > 0 && t < in.Times[i-1]) {
			return Result{}, errors.New("meal times must be ordered and not in the past")
		}
	}
	for _, r := range in.Recipes {
		if r.Minutes < 1 || r.Main < 0 || r.Main >= MaxIngredients {
			return Result{}, errors.New("invalid recipe")
		}
		has := false
		for _, n := range r.Needs {
			if n < 0 {
				return Result{}, errors.New("negative need")
			}
			has = has || n > 0
		}
		if !has {
			return Result{}, errors.New("empty recipe")
		}
	}
	for _, b := range in.Batches {
		if b.Ingredient < 0 || b.Ingredient >= MaxIngredients || b.Quantity < 0 {
			return Result{}, errors.New("invalid batch")
		}
	}
	w := pool.Get().(*workspace)
	*w = workspace{input: in}
	defer func() { w.input = Input{}; pool.Put(w) }()
	w.best.Count = len(in.Times)
	for i := range w.best.Meals {
		w.best.Meals[i].Recipe = -1
		w.path[i].Recipe = -1
	}
	for i, b := range in.Batches {
		w.remaining[i] = b.Quantity
		w.order[i] = i
	}
	// Fixed-array stable insertion sort; unknown expiries sort last.
	expiry := func(i int) int64 {
		v := in.Batches[i].Expires
		if v == 0 {
			return 1<<63 - 1
		}
		return v
	}
	for i := 1; i < len(in.Batches); i++ {
		x := w.order[i]
		j := i
		for j > 0 && expiry(x) < expiry(w.order[j-1]) {
			w.order[j] = w.order[j-1]
			j--
		}
		w.order[j] = x
	}
	for i, r := range in.Recipes {
		w.recipes[i] = i
		gain := 10.0
		for _, b := range in.Batches {
			if b.Quantity > 0 && b.Expires > in.Now && b.Expires-in.Now <= 48*3600 {
				gain += 100 * float64(min(r.Needs[b.Ingredient], b.Quantity)) / float64(b.Quantity)
			}
		}
		w.upper[i] = gain
		w.maxGain = max(w.maxGain, gain)
	}
	for i := 1; i < len(in.Recipes); i++ {
		x := w.recipes[i]
		j := i
		for j > 0 && w.upper[x]/float64(in.Recipes[x].Minutes) > w.upper[w.recipes[j-1]]/float64(in.Recipes[w.recipes[j-1]].Minutes) {
			w.recipes[j] = w.recipes[j-1]
			j--
		}
		w.recipes[j] = x
	}
	for i := range in.Times {
		w.best.Meals[i].Remaining = w.remaining
	}
	w.visit(0, -1, 0)
	return w.best, nil
}
func (w *workspace) visit(slot, previous int, score float64) {
	if w.best.Nodes >= w.input.MaxNodes {
		w.best.Truncated = true
		return
	}
	w.best.Nodes++
	if slot == len(w.input.Times) {
		if score > w.best.Score || w.best.Nodes == 1 {
			w.best.Score = score
			w.best.Meals = w.path
		}
		return
	}
	if score+float64(len(w.input.Times)-slot)*w.maxGain < w.best.Score {
		return
	}
	for _, ri := range w.recipes[:len(w.input.Recipes)] {
		if w.best.Nodes >= w.input.MaxNodes {
			w.best.Truncated = true
			return
		}
		w.best.Nodes++
		r := w.input.Recipes[ri]
		if r.Minutes > w.input.MaxMinutes {
			continue
		}
		before := w.remaining
		w.path[slot] = Meal{Recipe: ri}
		gain := 10.0 - 0.02*float64(r.Minutes)
		feasible := true
		needed := r.Needs
		for _, bi := range w.order[:len(w.input.Batches)] {
			b := w.input.Batches[bi]
			if needed[b.Ingredient] == 0 || (b.Expires != 0 && b.Expires <= w.input.Times[slot]) {
				continue
			}
			use := min(needed[b.Ingredient], w.remaining[bi])
			w.remaining[bi] -= use
			w.path[slot].Used[bi] = use
			needed[b.Ingredient] -= use
			if b.Quantity > 0 && b.Expires > w.input.Now && b.Expires-w.input.Now <= 48*3600 {
				gain += 100 * float64(use) / float64(b.Quantity)
			}
		}
		for _, left := range needed {
			if left > 0 {
				feasible = false
				break
			}
		}
		if previous >= 0 && w.input.Recipes[previous].Main == r.Main {
			gain -= 20
		}
		if feasible {
			w.path[slot].Remaining = w.remaining
			w.visit(slot+1, ri, score+gain)
		}
		w.remaining = before
	}
	// Empty slots are explicit; insufficient stock is never disguised as a meal.
	w.path[slot] = Meal{Recipe: -1, Remaining: w.remaining}
	w.visit(slot+1, -1, score)
}
