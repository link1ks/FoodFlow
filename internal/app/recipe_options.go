package app

import (
	"foodflow/internal/core"
	"foodflow/internal/engine/matcher"
	"github.com/gin-gonic/gin"
)

type recipeOptionsRequest struct {
	SelectedIngredientIDs []string `json:"selected_ingredient_ids"`
	Servings              int      `json:"servings"`
	MaxMinutes            int      `json:"max_minutes"`
}

func (a *App) recipeOptions(c *gin.Context) {
	var req recipeOptionsRequest
	if !input(c, &req) {
		return
	}
	if req.Servings < 1 || req.Servings > 20 || req.MaxMinutes < 1 || req.MaxMinutes > 240 || len(req.SelectedIngredientIDs) > 50 {
		fail(c, 400, "invalid selection, servings or cooking time")
		return
	}
	selected := map[string]bool{}
	for _, id := range req.SelectedIngredientIDs {
		if id == "" || selected[id] {
			fail(c, 400, "duplicate or invalid ingredient selection")
			return
		}
		selected[id] = true
	}
	if len(selected) == 0 {
		c.JSON(200, []gin.H{})
		return
	}
	rows, e := a.DB.Query(c, `SELECT i.id,i.name,i.unit,COALESCE(i.catalog_id::text,ic.id::text,i.name),
		COALESCE(sum(b.quantity_milli) FILTER (WHERE b.condition='normal' AND (b.expires_at IS NULL OR b.expires_at>now())),0)
		FROM ingredients i LEFT JOIN ingredient_catalog ic ON ic.name=i.name
		LEFT JOIN batches b ON b.ingredient_id=i.id AND b.household_id=i.household_id
		WHERE i.household_id=$1 AND i.archived_at IS NULL GROUP BY i.id,ic.id`, hid(c))
	if e != nil {
		fail(c, 500, "inventory unavailable")
		return
	}
	pantry := []core.PantryItem{}
	selectedNames := []string{}
	selectedKeys := []string{}
	selectedFound := 0
	for rows.Next() {
		var id, name, unit, key string
		var qty int64
		if e = rows.Scan(&id, &name, &unit, &key, &qty); e != nil {
			break
		}
		isSelected := selected[id]
		if isSelected {
			selectedFound++
			if qty <= 0 {
				rows.Close()
				fail(c, 409, "selected ingredient has no stock")
				return
			}
			selectedNames = append(selectedNames, name)
			selectedKeys = append(selectedKeys, key)
		}
		pantry = append(pantry, core.PantryItem{Name: name, Unit: unit, Quantity: qty, Selected: isSelected})
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		fail(c, 500, "inventory unavailable")
		return
	}
	if selectedFound != len(selected) {
		fail(c, 404, "selected ingredient unavailable")
		return
	}
	var excluded []string
	if e = a.DB.QueryRow(c, "SELECT excluded_ingredients FROM households WHERE id=$1", hid(c)).Scan(&excluded); e != nil {
		fail(c, 500, "preferences unavailable")
		return
	}
	rows, e = a.DB.Query(c, `WITH candidates AS (
		SELECT r.id FROM recipes r WHERE r.minutes<=$1
		AND EXISTS (SELECT 1 FROM recipe_items ri WHERE ri.recipe_id=r.id AND ri.name=ANY($2::text[]))
		AND NOT EXISTS (SELECT 1 FROM recipe_items ri WHERE ri.recipe_id=r.id AND ri.name=ANY($3::text[]))
		ORDER BY r.minutes,r.id LIMIT 50
	) SELECT r.id,r.title,r.servings,r.minutes,r.tags,ri.name,ri.quantity_milli,ri.unit,COALESCE(ri.catalog_id::text,ri.name)
	FROM candidates ca JOIN recipes r ON r.id=ca.id JOIN recipe_items ri ON ri.recipe_id=r.id
	ORDER BY r.id,ri.name,ri.unit`, req.MaxMinutes, selectedNames, excluded)
	if e != nil {
		fail(c, 500, "recipes unavailable")
		return
	}
	recipes := map[string]*core.RecipeCandidate{}
	recipeKeys := map[string][]string{}
	for rows.Next() {
		var id, title, name, unit, key string
		var tags []string
		var servings, minutes int
		var qty int64
		if e = rows.Scan(&id, &title, &servings, &minutes, &tags, &name, &qty, &unit, &key); e != nil {
			break
		}
		if recipes[id] == nil {
			recipes[id] = &core.RecipeCandidate{ID: id, Title: title, Servings: servings, Minutes: minutes, Tags: tags}
		}
		recipes[id].Requires = append(recipes[id].Requires, core.RecipeRequirement{Name: name, Unit: unit, Quantity: qty})
		recipeKeys[id] = append(recipeKeys[id], key)
	}
	if e == nil {
		e = rows.Err()
	}
	rows.Close()
	if e != nil {
		fail(c, 500, "recipes unavailable")
		return
	}
	indexed := make([]matcher.Recipe, 0, len(recipes))
	for _, recipe := range recipes {
		indexed = append(indexed, matcher.Recipe{ID: recipe.ID, Ingredients: recipeKeys[recipe.ID]})
	}
	index, err := matcher.New(indexed)
	if err != nil {
		fail(c, 500, "recipe index unavailable")
		return
	}
	selection := index.Select(selectedKeys)
	matches := index.MatchInto(selection, 2, make([]matcher.Result, 0, len(indexed)))
	options := make([]core.RecipeOption, 0, len(matches))
	for _, match := range matches {
		recipe := recipes[match.RecipeID]
		option, err := core.EvaluateRecipe(*recipe, pantry, req.Servings)
		if err != nil {
			fail(c, 500, "recipe calculation failed")
			return
		}
		if option.Matched > 0 && option.Missing <= 2 {
			options = append(options, option)
		}
	}
	core.SortRecipeOptions(options)
	out := []gin.H{}
	for _, option := range options {
		needs := []gin.H{}
		for _, item := range option.Coverage {
			var shortage any = core.Format(item.Shortage)
			if item.UnitUncertain {
				shortage = nil
			}
			needs = append(needs, gin.H{"name": item.Name, "unit": item.Unit, "needed": core.Format(item.Needed), "selected_available": core.Format(item.SelectedAvailable), "shortage": shortage, "unit_needs_confirmation": item.UnitUncertain})
		}
		out = append(out, gin.H{"recipe_id": option.Recipe.ID, "title": option.Recipe.Title, "minutes": option.Recipe.Minutes, "tags": option.Recipe.Tags, "status": option.Status, "matched_ingredients": option.Matched, "missing_ingredients": option.Missing, "requirements": needs})
	}
	c.JSON(200, out)
}
