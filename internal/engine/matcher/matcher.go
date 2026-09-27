// Package matcher performs ingredient-set matching on an already authorized,
// in-memory snapshot. Quantities and units must be checked separately before a
// recipe is labelled cookable.
package matcher

import (
	"fmt"
	"math/bits"
)

// Recipe contains canonical ingredient keys (normally catalog IDs). The
// caller resolves recipe names and aliases before building the index.
type Recipe struct {
	ID          string
	Ingredients []string
}

type compiledRecipe struct {
	id    string
	mask  []uint64
	total int
}

// Index is immutable after construction and safe to share between goroutines.
type Index struct {
	ids     map[string]int
	recipes []compiledRecipe
	words   int
}

type Selection struct {
	index *Index
	mask  []uint64
}

type Result struct {
	RecipeID string
	Matched  int
	Missing  int
	Total    int
}

func New(recipes []Recipe) (*Index, error) {
	idx := &Index{ids: make(map[string]int)}
	for _, recipe := range recipes {
		if recipe.ID == "" || len(recipe.Ingredients) == 0 {
			return nil, fmt.Errorf("recipe ID and ingredients are required")
		}
		for _, key := range recipe.Ingredients {
			if key == "" {
				return nil, fmt.Errorf("empty ingredient key in recipe %s", recipe.ID)
			}
			if _, ok := idx.ids[key]; !ok {
				idx.ids[key] = len(idx.ids)
			}
		}
	}
	idx.words = (len(idx.ids) + 63) / 64
	idx.recipes = make([]compiledRecipe, 0, len(recipes))
	seenRecipes := make(map[string]struct{}, len(recipes))
	for _, recipe := range recipes {
		if _, ok := seenRecipes[recipe.ID]; ok {
			return nil, fmt.Errorf("duplicate recipe %s", recipe.ID)
		}
		seenRecipes[recipe.ID] = struct{}{}
		compiled := compiledRecipe{id: recipe.ID, mask: make([]uint64, idx.words)}
		for _, key := range recipe.Ingredients {
			bit := idx.ids[key]
			word, offset := bit/64, uint(bit%64)
			if compiled.mask[word]&(uint64(1)<<offset) == 0 {
				compiled.mask[word] |= uint64(1) << offset
				compiled.total++
			}
		}
		idx.recipes = append(idx.recipes, compiled)
	}
	return idx, nil
}

// Select prepares a reusable mask. Unknown inventory keys are ignored: they
// cannot satisfy any recipe in this index.
func (idx *Index) Select(keys []string) Selection {
	selection := Selection{index: idx, mask: make([]uint64, idx.words)}
	for _, key := range keys {
		if bit, ok := idx.ids[key]; ok {
			selection.mask[bit/64] |= uint64(1) << uint(bit%64)
		}
	}
	return selection
}

// MatchInto appends recipes missing at most maxMissing ingredients to dst.
// The hot path only performs bit operations and appends into caller capacity.
// It neither queries a database nor allocates when dst has enough capacity.
func (idx *Index) MatchInto(selection Selection, maxMissing int, dst []Result) []Result {
	if selection.index != idx || maxMissing < 0 {
		return dst
	}
	for _, recipe := range idx.recipes {
		missing := 0
		for word, required := range recipe.mask {
			missing += bits.OnesCount64(required &^ selection.mask[word])
			if missing > maxMissing {
				break
			}
		}
		if missing <= maxMissing {
			dst = append(dst, Result{RecipeID: recipe.id, Matched: recipe.total - missing, Missing: missing, Total: recipe.total})
		}
	}
	return dst
}

func (idx *Index) RecipeCount() int     { return len(idx.recipes) }
func (idx *Index) IngredientCount() int { return len(idx.ids) }
