package app

import (
	"sort"
	"time"

	"foodflow/internal/core"
	"foodflow/internal/engine/scheduler"
	"github.com/gin-gonic/gin"
)

type clearFridgeRequest struct {
	SelectedIngredientIDs []string `json:"selected_ingredient_ids"`
	Servings              int      `json:"servings"`
	MaxMinutes            int      `json:"max_minutes"`
}

func canonicalRecipeUnit(unit string) (string, error) {
	dimension, _, err := core.Dimension(unit)
	if err != nil {
		return "", err
	}
	switch dimension {
	case "mass":
		return "g", nil
	case "volume":
		return "ml", nil
	default:
		return unit, nil
	}
}

// clearFridge only recommends. A later meal confirmation and cooking operation
// must read current inventory again; no reservation or stock write happens here.
func (a *App) clearFridge(c *gin.Context) {
	var request clearFridgeRequest
	if !input(c, &request) {
		return
	}
	if request.Servings < 1 || request.Servings > 20 || request.MaxMinutes < 1 || request.MaxMinutes > 240 || len(request.SelectedIngredientIDs) == 0 || len(request.SelectedIngredientIDs) > 50 {
		fail(c, 400, "invalid clear-fridge request")
		return
	}
	selectedInventory := make(map[string]bool, len(request.SelectedIngredientIDs))
	for _, id := range request.SelectedIngredientIDs {
		if id == "" || selectedInventory[id] {
			fail(c, 400, "invalid ingredient selection")
			return
		}
		selectedInventory[id] = true
	}
	var excluded []string
	if err := a.DB.QueryRow(c, "SELECT excluded_ingredients FROM households WHERE id=$1", hid(c)).Scan(&excluded); err != nil {
		fail(c, 500, "preferences unavailable")
		return
	}
	rows, err := a.DB.Query(c, `SELECT r.id,r.title,r.servings,r.minutes,ri.name,ri.quantity_milli,ri.unit,COALESCE(ri.catalog_id::text,ri.name)
		FROM recipes r JOIN recipe_items ri ON ri.recipe_id=r.id
		WHERE r.minutes<=$1 AND NOT EXISTS
		(SELECT 1 FROM recipe_items blocked WHERE blocked.recipe_id=r.id AND blocked.name=ANY($2::text[]))
		ORDER BY r.minutes,r.id,ri.name LIMIT 400`, request.MaxMinutes, excluded)
	if err != nil {
		fail(c, 500, "recipes unavailable")
		return
	}
	keys := map[string]int{}
	displayByKey := map[string]string{}
	recipes := map[string]*scheduler.Recipe{}
	titles := map[string]string{}
	for rows.Next() {
		var id, title, name, unit, ingredientKey string
		var servings, minutes int
		var quantity int64
		if err = rows.Scan(&id, &title, &servings, &minutes, &name, &quantity, &unit, &ingredientKey); err != nil {
			break
		}
		if recipes[id] == nil {
			if len(recipes) == 20 {
				continue
			}
			recipes[id] = &scheduler.Recipe{ID: id, Servings: servings, Minutes: minutes}
			titles[id] = title
		}
		originalUnit := unit
		unit, err = canonicalRecipeUnit(unit)
		if err != nil {
			break
		}
		quantity, err = core.Convert(quantity, originalUnit, unit)
		if err != nil {
			break
		}
		key := ingredientKey + "\x00" + unit
		displayByKey[key] = name + " · " + unit
		index, ok := keys[key]
		if !ok {
			index = len(keys)
			keys[key] = index
		}
		merged := false
		for i := range recipes[id].Needs {
			if recipes[id].Needs[i].Ingredient == index {
				recipes[id].Needs[i].Quantity += quantity
				merged = true
				break
			}
		}
		if !merged {
			recipes[id].Needs = append(recipes[id].Needs, scheduler.Need{Ingredient: index, Quantity: quantity})
		}
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(c, 500, "recipe snapshot failed")
		return
	}
	rows, err = a.DB.Query(c, `SELECT i.id,i.name,i.unit,COALESCE(i.catalog_id::text,ic.id::text,i.name),
		b.id,b.quantity_milli,b.expires_at,b.condition
		FROM ingredients i LEFT JOIN batches b ON b.ingredient_id=i.id AND b.household_id=i.household_id AND b.quantity_milli>0
		LEFT JOIN ingredient_catalog ic ON ic.name=i.name
		WHERE i.household_id=$1 AND i.archived_at IS NULL ORDER BY i.id,b.id`, hid(c))
	if err != nil {
		fail(c, 500, "inventory unavailable")
		return
	}
	batches := []scheduler.Batch{}
	selectedKeys := map[int]bool{}
	found := map[string]bool{}
	uncertain := map[string]bool{}
	for rows.Next() {
		var ingredientID, name, unit, ingredientKey string
		var batchID *string
		var quantity *int64
		var expiry *time.Time
		var condition *string
		if err = rows.Scan(&ingredientID, &name, &unit, &ingredientKey, &batchID, &quantity, &expiry, &condition); err != nil {
			break
		}
		if selectedInventory[ingredientID] {
			found[ingredientID] = true
		}
		if batchID == nil || quantity == nil {
			continue
		}
		canonical, conversionErr := canonicalRecipeUnit(unit)
		if conversionErr != nil {
			uncertain[name] = true
			continue
		}
		key := ingredientKey + "\x00" + canonical
		index, ok := keys[key]
		if !ok {
			continue
		}
		converted, conversionErr := core.Convert(*quantity, unit, canonical)
		if conversionErr != nil {
			uncertain[name] = true
			continue
		}
		if selectedInventory[ingredientID] {
			selectedKeys[index] = true
		}
		batch := scheduler.Batch{ID: *batchID, Ingredient: index, Quantity: converted, Usable: condition != nil && *condition == "normal"}
		if expiry != nil {
			batch.ExpiresAt = *expiry
		}
		batches = append(batches, batch)
	}
	if err == nil {
		err = rows.Err()
	}
	rows.Close()
	if err != nil {
		fail(c, 500, "inventory snapshot failed")
		return
	}
	if len(found) != len(selectedInventory) {
		fail(c, 404, "selected ingredient unavailable")
		return
	}
	selected := make([]int, 0, len(selectedKeys))
	for id := range selectedKeys {
		selected = append(selected, id)
	}
	allRecipes := make([]scheduler.Recipe, 0, len(recipes))
	for _, recipe := range recipes {
		for _, need := range recipe.Needs {
			if selectedKeys[need.Ingredient] {
				allRecipes = append(allRecipes, *recipe)
				break
			}
		}
	}
	keyByID := make([]string, len(keys))
	for key, id := range keys {
		keyByID[id] = displayByKey[key]
	}
	plan, err := scheduler.Schedule(allRecipes, batches, scheduler.Request{Servings: request.Servings, MaxMinutes: request.MaxMinutes, Selected: selected, AsOf: time.Now()})
	if err != nil {
		fail(c, 400, err.Error())
		return
	}
	chosen := make([]gin.H, 0, len(plan.RecipeIDs))
	for _, id := range plan.RecipeIDs {
		chosen = append(chosen, gin.H{"id": id, "title": titles[id]})
	}
	conflicts := make([]gin.H, 0, len(plan.Conflicts))
	for _, conflict := range plan.Conflicts {
		conflicts = append(conflicts, gin.H{"recipe_a": titles[conflict.RecipeA], "recipe_b": titles[conflict.RecipeB], "ingredient": keyByID[conflict.Ingredient]})
	}
	uses := make([]gin.H, 0, len(plan.Allocations))
	for _, allocation := range plan.Allocations {
		uses = append(uses, gin.H{"recipe_id": allocation.RecipeID, "batch_id": allocation.BatchID, "ingredient": keyByID[allocation.Ingredient], "quantity": core.Format(allocation.Quantity)})
	}
	warnings := make([]string, 0, len(uncertain))
	for name := range uncertain {
		warnings = append(warnings, name+"的单位无法精确换算，未计入推荐库存")
	}
	sort.Strings(warnings)
	c.JSON(200, gin.H{"recipes": chosen, "minutes": plan.Minutes, "score": plan.Score, "conflicts": conflicts, "batch_uses": uses, "warnings": warnings, "truncated": plan.Truncated, "inventory_recheck_required": true})
}
