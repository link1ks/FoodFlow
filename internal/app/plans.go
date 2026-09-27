package app

import (
	"fmt"
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"sort"
	"strings"
	"time"
)

type mealInput struct {
	Day                 string   `json:"day"`
	Meal                string   `json:"meal"`
	Servings            int      `json:"servings"`
	RecipeID            string   `json:"recipe_id"`
	AdditionalRecipeIDs []string `json:"additional_recipe_ids"`
}

func validMeal(x mealInput) error {
	if _, e := time.Parse("2006-01-02", x.Day); e != nil {
		return e
	}
	if x.Meal != "breakfast" && x.Meal != "lunch" && x.Meal != "dinner" {
		return fmt.Errorf("invalid meal")
	}
	if x.Servings < 1 || x.Servings > 20 {
		return fmt.Errorf("servings must be 1-20")
	}
	if x.RecipeID == "" {
		return fmt.Errorf("recipe required")
	}
	if err := validAdditionalRecipes(x.RecipeID, x.AdditionalRecipeIDs); err != nil {
		return err
	}
	return nil
}

func validAdditionalRecipes(primary string, additional []string) error {
	if len(additional) > 19 {
		return fmt.Errorf("at most 20 dishes per meal")
	}
	seen := map[string]bool{primary: true}
	for _, id := range additional {
		if id == "" || seen[id] {
			return fmt.Errorf("duplicate or invalid dish")
		}
		seen[id] = true
	}
	return nil
}
func (a *App) recipes(c *gin.Context) {
	max := c.Query("max_minutes")
	tag := c.Query("tag")
	ingredient := c.Query("ingredient")
	rows, e := a.DB.Query(c, `SELECT r.id,r.title,r.servings,r.minutes,r.tags,r.steps,r.source FROM recipes r WHERE ($1='' OR r.minutes<=NULLIF($1,'')::int) AND ($2='' OR $2=ANY(r.tags)) AND ($3='' OR EXISTS(SELECT 1 FROM recipe_items ri WHERE ri.recipe_id=r.id AND ri.name ILIKE '%'||$3||'%')) ORDER BY r.minutes,r.title`, max, tag, ingredient)
	if e != nil {
		fail(c, 400, "invalid filter")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, title, source string
		var servings, minutes int
		var tags, steps []string
		if rows.Scan(&id, &title, &servings, &minutes, &tags, &steps, &source) == nil {
			ir, er := a.DB.Query(c, "SELECT name,quantity_milli,unit FROM recipe_items WHERE recipe_id=$1", id)
			items := []gin.H{}
			if er == nil {
				for ir.Next() {
					var name, unit string
					var q int64
					if ir.Scan(&name, &q, &unit) == nil {
						items = append(items, gin.H{"name": name, "quantity": core.Format(q), "unit": unit})
					}
				}
				ir.Close()
			}
			out = append(out, gin.H{"id": id, "title": title, "servings": servings, "minutes": minutes, "tags": tags, "steps": steps, "source": source, "ingredients": items})
		}
	}
	c.JSON(200, out)
}
func (a *App) createPlan(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x struct {
		Meals []mealInput `json:"meals"`
	}
	if !input(c, &x) {
		return
	}
	if len(x.Meals) == 0 || len(x.Meals) > 21 {
		fail(c, 400, "1-21 meals required")
		return
	}
	for _, m := range x.Meals {
		if e := validMeal(m); e != nil {
			fail(c, 400, e.Error())
			return
		}
	}
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "transaction failed")
		return
	}
	defer tx.Rollback(c)
	id := core.ID()
	_, e = tx.Exec(c, "INSERT INTO plans(id,household_id,status,created_by,excluded_ingredients) VALUES($1,$2,'draft',$3,'{}')", id, hid(c), uid(c))
	if e != nil {
		fail(c, 500, "create failed")
		return
	}
	for _, m := range x.Meals {
		mealID := core.ID()
		_, e = tx.Exec(c, "INSERT INTO plan_meals(id,plan_id,day,meal,servings,recipe_id) VALUES($1,$2,$3,$4,$5,$6)", mealID, id, m.Day, m.Meal, m.Servings, m.RecipeID)
		if e != nil {
			fail(c, 400, "invalid or duplicate meal")
			return
		}
		for _, recipeID := range m.AdditionalRecipeIDs {
			_, e = tx.Exec(c, "INSERT INTO plan_meal_dishes(id,plan_meal_id,recipe_id) VALUES($1,$2,$3)", core.ID(), mealID, recipeID)
			if e != nil {
				fail(c, 400, "invalid additional recipe")
				return
			}
		}
	}
	if e = tx.Commit(c); e != nil {
		fail(c, 500, "commit failed")
		return
	}
	c.JSON(201, gin.H{"id": id, "status": "draft"})
}
func (a *App) plans(c *gin.Context) {
	rows, e := a.DB.Query(c, "SELECT id,status,revision,created_at FROM plans WHERE household_id=$1 ORDER BY created_at DESC LIMIT 30", hid(c))
	if e != nil {
		fail(c, 500, "query failed")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, status string
		var revision int
		var created time.Time
		if rows.Scan(&id, &status, &revision, &created) == nil {
			out = append(out, gin.H{"id": id, "status": status, "revision": revision, "created_at": created})
		}
	}
	c.JSON(200, out)
}

func (a *App) today(c *gin.Context) {
	day := c.Query("day")
	if _, e := time.Parse("2006-01-02", day); e != nil {
		fail(c, 400, "valid day required")
		return
	}
	rows, e := a.DB.Query(c, `SELECT plan_id,meal_id,meal,servings,title,minutes,status FROM (
		SELECT DISTINCT ON (pm.meal) p.id AS plan_id,pm.id AS meal_id,pm.meal,pm.servings,
			r.title||COALESCE(' + '||extra.titles,'') AS title,r.minutes+extra.minutes AS minutes,
			CASE WHEN mc.id IS NOT NULL OR p.status='consumed' THEN 'consumed' ELSE 'confirmed' END AS status
		FROM plan_meals pm JOIN plans p ON p.id=pm.plan_id JOIN recipes r ON r.id=pm.recipe_id
		LEFT JOIN LATERAL (SELECT string_agg(r2.title,' + ' ORDER BY r2.title) AS titles,
			COALESCE(sum(r2.minutes),0)::int AS minutes FROM plan_meal_dishes pmd
			JOIN recipes r2 ON r2.id=pmd.recipe_id WHERE pmd.plan_meal_id=pm.id) extra ON true
		LEFT JOIN meal_completions mc ON mc.plan_meal_id=pm.id
		WHERE p.household_id=$1 AND p.status IN ('confirmed','consumed') AND pm.day=$2::date
		ORDER BY pm.meal,p.confirmed_at DESC,p.created_at DESC,p.id DESC
	) picked ORDER BY CASE meal WHEN 'breakfast' THEN 1 WHEN 'lunch' THEN 2 ELSE 3 END`, hid(c), day)
	if e != nil {
		fail(c, 500, "today unavailable")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var plan, mealID, meal, title, status string
		var servings, minutes int
		if rows.Scan(&plan, &mealID, &meal, &servings, &title, &minutes, &status) == nil {
			out = append(out, gin.H{"plan_id": plan, "meal_id": mealID, "meal": meal, "servings": servings, "title": title, "minutes": minutes, "status": status})
		}
	}
	if rows.Err() != nil {
		fail(c, 500, "today unavailable")
		return
	}
	c.JSON(200, out)
}

type demand struct {
	Name, Unit                  string
	Needed, Available, Shortage int64
	Uncertain                   bool
}

func (a *App) demands(c *gin.Context, tx pgx.Tx, plan string) ([]demand, error) {
	rows, e := tx.Query(c, `WITH dishes AS (
		SELECT pm.servings,pm.recipe_id FROM plan_meals pm WHERE pm.plan_id=$1
		UNION ALL
		SELECT pm.servings,pmd.recipe_id FROM plan_meals pm
		JOIN plan_meal_dishes pmd ON pmd.plan_meal_id=pm.id WHERE pm.plan_id=$1
	) SELECT ri.name,ri.unit,ri.quantity_milli,d.servings,r.servings
	FROM dishes d JOIN recipes r ON r.id=d.recipe_id JOIN recipe_items ri ON ri.recipe_id=r.id`, plan)
	if e != nil {
		return nil, e
	}
	m := map[string]*demand{}
	for rows.Next() {
		var name, unit string
		var q int64
		var servings, base int
		if e = rows.Scan(&name, &unit, &q, &servings, &base); e != nil {
			rows.Close()
			return nil, e
		}
		scaled, er := core.Scale(q, servings, base)
		if er != nil {
			rows.Close()
			return nil, er
		}
		dim, _, er := core.Dimension(unit)
		if er != nil {
			rows.Close()
			return nil, er
		}
		canonical := unit
		if dim == "mass" {
			canonical = "g"
		} else if dim == "volume" {
			canonical = "ml"
		}
		scaled, er = core.Convert(scaled, unit, canonical)
		if er != nil {
			rows.Close()
			return nil, er
		}
		k := name + "\x00" + canonical
		if m[k] == nil {
			m[k] = &demand{Name: name, Unit: canonical}
		}
		m[k].Needed += scaled
	}
	rows.Close()
	for _, d := range m {
		ir, er := tx.Query(c, "SELECT i.unit,COALESCE(sum(b.quantity_milli) FILTER (WHERE b.condition='normal' AND (b.expires_at IS NULL OR b.expires_at>now())),0) FROM ingredients i LEFT JOIN batches b ON b.ingredient_id=i.id WHERE i.household_id=$1 AND i.name=$2 AND i.archived_at IS NULL GROUP BY i.id", hid(c), d.Name)
		if er != nil {
			return nil, er
		}
		for ir.Next() {
			var unit string
			var q int64
			if er = ir.Scan(&unit, &q); er != nil {
				ir.Close()
				return nil, er
			}
			converted, ce := core.Convert(q, unit, d.Unit)
			if ce != nil {
				d.Uncertain = true
				continue
			}
			d.Available += converted
		}
		ir.Close()
		if d.Available < d.Needed {
			d.Shortage = d.Needed - d.Available
		}
	}
	out := make([]demand, 0, len(m))
	for _, v := range m {
		out = append(out, *v)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name+out[i].Unit < out[j].Name+out[j].Unit })
	return out, nil
}
func (a *App) plan(c *gin.Context) {
	var status string
	var revision int
	e := a.DB.QueryRow(c, "SELECT status,revision FROM plans WHERE id=$1 AND household_id=$2", c.Param("plan"), hid(c)).Scan(&status, &revision)
	if e != nil {
		fail(c, 404, "plan unavailable")
		return
	}
	rows, e := a.DB.Query(c, "SELECT pm.id,pm.day::text,pm.meal,pm.servings,r.id,r.title,r.minutes FROM plan_meals pm JOIN recipes r ON r.id=pm.recipe_id WHERE pm.plan_id=$1 ORDER BY pm.day,pm.meal", c.Param("plan"))
	if e != nil {
		fail(c, 500, "query failed")
		return
	}
	meals := []gin.H{}
	for rows.Next() {
		var id, day, meal, rid, title string
		var servings, minutes int
		if rows.Scan(&id, &day, &meal, &servings, &rid, &title, &minutes) == nil {
			meals = append(meals, gin.H{"id": id, "day": day, "meal": meal, "servings": servings, "recipe_id": rid, "title": title, "minutes": minutes})
		}
	}
	if e = rows.Err(); e != nil {
		rows.Close()
		fail(c, 500, "query failed")
		return
	}
	rows.Close()
	for _, meal := range meals {
		extraRows, err := a.DB.Query(c, `SELECT r.id,r.title,r.minutes FROM plan_meal_dishes pmd
			JOIN recipes r ON r.id=pmd.recipe_id WHERE pmd.plan_meal_id=$1 ORDER BY r.title,r.id`, meal["id"])
		if err != nil {
			fail(c, 500, "query failed")
			return
		}
		extra := []gin.H{}
		for extraRows.Next() {
			var id, title string
			var minutes int
			if err = extraRows.Scan(&id, &title, &minutes); err != nil {
				break
			}
			extra = append(extra, gin.H{"id": id, "title": title, "minutes": minutes})
		}
		if err == nil {
			err = extraRows.Err()
		}
		extraRows.Close()
		if err != nil {
			fail(c, 500, "query failed")
			return
		}
		meal["additional_recipes"] = extra
	}
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "query failed")
		return
	}
	defer tx.Rollback(c)
	ds, e := a.demands(c, tx, c.Param("plan"))
	if e != nil {
		fail(c, 500, "demand calculation failed")
		return
	}
	dout := []gin.H{}
	for _, d := range ds {
		dout = append(dout, gin.H{"name": d.Name, "unit": d.Unit, "needed": core.Format(d.Needed), "available": core.Format(d.Available), "shortage": core.Format(d.Shortage), "conversion_needs_confirmation": d.Uncertain})
	}
	c.JSON(200, gin.H{"id": c.Param("plan"), "status": status, "revision": revision, "meals": meals, "ingredients": dout, "inventory_rechecked_at": time.Now().UTC()})
}
func (a *App) editMeal(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x struct {
		RecipeID            string   `json:"recipe_id"`
		Servings            int      `json:"servings"`
		Cancel              bool     `json:"cancel"`
		AdditionalRecipeIDs []string `json:"additional_recipe_ids"`
	}
	if !input(c, &x) {
		return
	}
	if !x.Cancel && (x.Servings < 1 || x.Servings > 20 || x.RecipeID == "") {
		fail(c, 400, "invalid meal")
		return
	}
	if !x.Cancel && x.AdditionalRecipeIDs != nil {
		if err := validAdditionalRecipes(x.RecipeID, x.AdditionalRecipeIDs); err != nil {
			fail(c, 400, err.Error())
			return
		}
	}
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "transaction failed")
		return
	}
	defer tx.Rollback(c)
	var status string
	e = tx.QueryRow(c, "SELECT status FROM plans WHERE id=$1 AND household_id=$2 FOR UPDATE", c.Param("plan"), hid(c)).Scan(&status)
	if e != nil || status != "draft" {
		fail(c, 409, "draft plan required")
		return
	}
	var tag pgconn.CommandTag
	if x.Cancel {
		tag, e = tx.Exec(c, "DELETE FROM plan_meals WHERE id=$1 AND plan_id=$2", c.Param("meal"), c.Param("plan"))
	} else {
		if x.AdditionalRecipeIDs == nil {
			var duplicate bool
			e = tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM plan_meal_dishes WHERE plan_meal_id=$1 AND recipe_id=$2)", c.Param("meal"), x.RecipeID).Scan(&duplicate)
			if e != nil || duplicate {
				fail(c, 409, "recipe already in meal")
				return
			}
		}
		tag, e = tx.Exec(c, "UPDATE plan_meals SET recipe_id=$1,servings=$2 WHERE id=$3 AND plan_id=$4", x.RecipeID, x.Servings, c.Param("meal"), c.Param("plan"))
	}
	if e != nil {
		fail(c, 400, "invalid recipe")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(c, 404, "meal unavailable")
		return
	}
	if !x.Cancel && x.AdditionalRecipeIDs != nil {
		if _, e = tx.Exec(c, "DELETE FROM plan_meal_dishes WHERE plan_meal_id=$1", c.Param("meal")); e == nil {
			for _, recipeID := range x.AdditionalRecipeIDs {
				_, e = tx.Exec(c, "INSERT INTO plan_meal_dishes(id,plan_meal_id,recipe_id) VALUES($1,$2,$3)", core.ID(), c.Param("meal"), recipeID)
				if e != nil {
					break
				}
			}
		}
		if e != nil {
			fail(c, 400, "invalid additional recipe")
			return
		}
	}
	_, e = tx.Exec(c, "UPDATE plans SET revision=revision+1 WHERE id=$1", c.Param("plan"))
	if e == nil {
		e = tx.Commit(c)
	}
	if e != nil {
		fail(c, 500, "update failed")
		return
	}
	c.Status(204)
}
func (a *App) confirmPlan(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x struct {
		Revision        int  `json:"revision"`
		AcceptUncertain bool `json:"accept_uncertain"`
	}
	if !input(c, &x) {
		return
	}
	a.idem(c, x, func(tx pgx.Tx) (any, error) {
		// Serialize calendar confirmation with multi-meal adoption. NO KEY
		// UPDATE still permits foreign-key checks by stock transactions.
		var householdID string
		if err := tx.QueryRow(c, "SELECT id FROM households WHERE id=$1 FOR NO KEY UPDATE", hid(c)).Scan(&householdID); err != nil {
			return nil, err
		}
		var status string
		var revision int
		e := tx.QueryRow(c, "SELECT status,revision FROM plans WHERE id=$1 AND household_id=$2 FOR UPDATE", c.Param("plan"), hid(c)).Scan(&status, &revision)
		if e != nil {
			return nil, businessError{404, "plan unavailable"}
		}
		if status != "draft" || revision != x.Revision {
			return nil, businessError{409, "plan changed or already confirmed"}
		}
		var occupied bool
		if err := tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM plan_meals wanted JOIN plan_meals other ON other.day=wanted.day AND other.meal=wanted.meal JOIN plans p ON p.id=other.plan_id WHERE wanted.plan_id=$1 AND p.id<>$1 AND p.household_id=$2 AND p.status IN ('confirmed','consumed'))`, c.Param("plan"), hid(c)).Scan(&occupied); err != nil {
			return nil, err
		}
		if occupied {
			return nil, conflict("目标餐次已有已确认菜单")
		}
		ds, e := a.demands(c, tx, c.Param("plan"))
		if e != nil {
			return nil, businessError{500, "calculation failed"}
		}
		for _, d := range ds {
			if d.Uncertain && !x.AcceptUncertain {
				return nil, businessError{409, "unit conversion requires explicit confirmation"}
			}
		}
		var count int
		_ = tx.QueryRow(c, "SELECT count(*) FROM plan_meals WHERE plan_id=$1", c.Param("plan")).Scan(&count)
		if count == 0 {
			return nil, businessError{409, "empty plan"}
		}
		var forbidden int
		e = tx.QueryRow(c, `WITH dishes AS (
		SELECT pm.recipe_id FROM plan_meals pm WHERE pm.plan_id=$1
		UNION ALL SELECT pmd.recipe_id FROM plan_meals pm
		JOIN plan_meal_dishes pmd ON pmd.plan_meal_id=pm.id WHERE pm.plan_id=$1
	) SELECT count(*) FROM dishes d JOIN recipe_items ri ON ri.recipe_id=d.recipe_id
	JOIN plans p ON p.id=$1 JOIN households h ON h.id=p.household_id
	WHERE ri.name=ANY(p.excluded_ingredients||h.excluded_ingredients)`, c.Param("plan")).Scan(&forbidden)
		if e != nil {
			return nil, businessError{500, "dietary check failed"}
		}
		if forbidden > 0 {
			return nil, businessError{409, "recipe contains excluded ingredient"}
		}
		_, e = tx.Exec(c, "UPDATE plans SET status='confirmed',confirmed_at=now() WHERE id=$1", c.Param("plan"))
		listID := core.ID()
		if e == nil {
			_, e = tx.Exec(c, "INSERT INTO shopping_lists(id,household_id,plan_id) VALUES($1,$2,$3)", listID, hid(c), c.Param("plan"))
		}
		for _, d := range ds {
			if e != nil {
				break
			}
			if d.Shortage > 0 {
				_, e = tx.Exec(c, "INSERT INTO shopping_items(id,list_id,name,unit,needed_milli) VALUES($1,$2,$3,$4,$5)", core.ID(), listID, d.Name, d.Unit, d.Shortage)
			}
		}
		if e == nil {
			_, e = tx.Exec(c, "UPDATE jobs SET status='succeeded',updated_at=now() WHERE household_id=$1 AND status='awaiting_confirmation' AND result->>'plan_id'=$2", hid(c), c.Param("plan"))
		}

		if e != nil {
			return nil, businessError{500, "confirmation failed"}
		}
		return gin.H{"shopping_list_id": listID, "shortages_rechecked": true}, nil
	})
}
func (a *App) rejectPlan(c *gin.Context) {
	if !writable(c) {
		return
	}
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "transaction failed")
		return
	}
	defer tx.Rollback(c)
	tag, e := tx.Exec(c, "UPDATE plans SET status='rejected' WHERE id=$1 AND household_id=$2 AND status='draft'", c.Param("plan"), hid(c))
	if e != nil || tag.RowsAffected() == 0 {
		fail(c, 409, "draft plan required")
		return
	}
	_, e = tx.Exec(c, "UPDATE jobs SET status='cancelled',updated_at=now() WHERE household_id=$1 AND status='awaiting_confirmation' AND result->>'plan_id'=$2", hid(c), c.Param("plan"))
	if e == nil {
		e = tx.Commit(c)
	}
	if e != nil {
		fail(c, 500, "reject failed")
		return
	}
	c.Status(204)
}
func (a *App) consumePlan(c *gin.Context) {
	if !writable(c) {
		return
	}
	x := struct{ Plan string }{c.Param("plan")}
	a.idem(c, x, func(tx pgx.Tx) (any, error) {
		var status string
		e := tx.QueryRow(c, "SELECT status FROM plans WHERE id=$1 AND household_id=$2 FOR UPDATE", x.Plan, hid(c)).Scan(&status)
		if e != nil {
			return nil, conflict("plan unavailable")
		}
		if status != "confirmed" {
			return nil, conflict("confirmed plan required")
		}
		mealRows, e := tx.Query(c, "SELECT id,day::text,meal FROM plan_meals WHERE plan_id=$1 ORDER BY day,meal FOR UPDATE", x.Plan)
		if e != nil {
			return nil, e
		}
		type completion struct{ id, day, meal string }
		var completions []completion
		for mealRows.Next() {
			var m completion
			if e = mealRows.Scan(&m.id, &m.day, &m.meal); e != nil {
				mealRows.Close()
				return nil, e
			}
			completions = append(completions, m)
		}
		e = mealRows.Err()
		mealRows.Close()
		if e != nil {
			return nil, e
		}
		for _, m := range completions {
			var inserted string
			e = tx.QueryRow(c, `INSERT INTO meal_completions(id,household_id,plan_meal_id,day,meal,actor_id)
				VALUES($1,$2,$3,$4::date,$5,$6) ON CONFLICT DO NOTHING RETURNING id`, core.ID(), hid(c), m.id, m.day, m.meal, uid(c)).Scan(&inserted)
			if e == pgx.ErrNoRows {
				return nil, conflict("meal already completed")
			}
			if e != nil {
				return nil, e
			}
		}
		for _, meal := range completions {
			needs, err := a.mealNeeds(c, tx, meal.id)
			if err != nil {
				return nil, err
			}
			for _, need := range needs {
				if err := a.consumeNeed(c.Request.Context(), tx, cookingScope{HouseholdID: hid(c), ActorID: uid(c)}, need, meal.id); err != nil {
					return nil, err
				}
			}
			if err := a.consumeSeasonings(c.Request.Context(), tx, cookingScope{HouseholdID: hid(c), ActorID: uid(c)}, meal.id); err != nil {
				return nil, err
			}
		}
		_, e = tx.Exec(c, "UPDATE plans SET status='consumed' WHERE id=$1", x.Plan)
		return gin.H{"plan_id": x.Plan, "status": "consumed"}, e
	})
}
func (a *App) shopping(c *gin.Context) {
	rows, e := a.DB.Query(c, `SELECT l.id,COALESCE(l.plan_id::text,''),i.id,i.name,i.unit,i.needed_milli,i.bought_milli,i.checked,i.stocked,i.origin FROM shopping_lists l JOIN shopping_items i ON i.list_id=l.id WHERE l.household_id=$1 AND i.deleted_at IS NULL ORDER BY l.created_at DESC,i.name LIMIT 200`, hid(c))
	if e != nil {
		fail(c, 500, "query failed")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var list, plan, id, name, unit, origin string
		var needed, bought int64
		var checked, stocked bool
		if rows.Scan(&list, &plan, &id, &name, &unit, &needed, &bought, &checked, &stocked, &origin) == nil {
			out = append(out, gin.H{"list_id": list, "plan_id": plan, "id": id, "name": name, "unit": unit, "needed": core.Format(needed), "bought": core.Format(bought), "checked": checked, "stocked": stocked, "origin": origin})
		}
	}
	c.JSON(200, out)
}
func (a *App) editShopping(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x struct {
		Checked bool   `json:"checked"`
		Bought  string `json:"bought"`
	}
	if !input(c, &x) {
		return
	}
	q := int64(0)
	var e error
	if x.Bought != "" {
		q, e = core.Quantity(x.Bought)
		if e != nil {
			fail(c, 400, e.Error())
			return
		}
	}
	tag, e := a.DB.Exec(c, "UPDATE shopping_items i SET checked=$1,bought_milli=$2 FROM shopping_lists l WHERE i.list_id=l.id AND i.id=$3 AND l.household_id=$4 AND i.stocked=false AND i.deleted_at IS NULL", x.Checked, q, c.Param("item"), hid(c))
	if e != nil {
		fail(c, 500, "update failed")
		return
	}
	if tag.RowsAffected() == 0 {
		fail(c, 409, "item unavailable or already stocked")
		return
	}
	c.Status(204)
}
func (a *App) stockShopping(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x struct{ Location, BoughtOn, ExpiresOn, ExpiryKind, Source string }
	if !input(c, &x) {
		return
	}
	if x.BoughtOn != "" {
		if _, e := time.Parse("2006-01-02", x.BoughtOn); e != nil {
			fail(c, 400, "invalid purchase date")
			return
		}
	}
	if x.ExpiresOn != "" {
		if _, e := time.Parse("2006-01-02", x.ExpiresOn); e != nil {
			fail(c, 400, "invalid expiry date")
			return
		}
		if x.ExpiryKind != "label" && x.ExpiryKind != "user" && x.ExpiryKind != "estimate" {
			fail(c, 400, "expiry kind required")
			return
		}
	} else {
		x.ExpiryKind = "unknown"
	}
	request := struct {
		ID   string
		Data any
	}{c.Param("item"), x}
	a.idem(c, request, func(tx pgx.Tx) (any, error) {
		var name, unit string
		var q int64
		var stocked, checked bool
		e := tx.QueryRow(c, "SELECT i.name,i.unit,i.bought_milli,i.checked,i.stocked FROM shopping_items i JOIN shopping_lists l ON l.id=i.list_id WHERE i.id=$1 AND l.household_id=$2 AND i.deleted_at IS NULL FOR UPDATE OF i", c.Param("item"), hid(c)).Scan(&name, &unit, &q, &checked, &stocked)
		if e != nil {
			return nil, conflict("item unavailable")
		}
		if stocked {
			return nil, conflict("item already stocked")
		}
		if !checked || q <= 0 {
			return nil, conflict("check item and record actual quantity first")
		}
		dim, _, _ := core.Dimension(unit)
		ingredient := core.ID()
		e = tx.QueryRow(c, "INSERT INTO ingredients(id,household_id,name,unit,dimension) VALUES($1,$2,$3,$4,$5) ON CONFLICT(household_id,name,unit) DO UPDATE SET name=EXCLUDED.name,archived_at=NULL RETURNING id", ingredient, hid(c), name, unit, dim).Scan(&ingredient)
		if e != nil {
			return nil, e
		}
		batch := core.ID()
		_, e = tx.Exec(c, "INSERT INTO batches(id,household_id,ingredient_id,quantity_milli,location,bought_on,expires_on,expiry_kind,source) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::date,NULLIF($7,'')::date,$8,$9)", batch, hid(c), ingredient, q, x.Location, x.BoughtOn, x.ExpiresOn, x.ExpiryKind, strings.TrimSpace(x.Source))
		if e != nil {
			return nil, e
		}
		_, e = tx.Exec(c, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,ref_type,ref_id) VALUES($1,$2,$3,$4,'purchase',$5,'shopping_item',$6)", core.ID(), hid(c), batch, q, uid(c), c.Param("item"))
		if e != nil {
			return nil, e
		}
		_, e = tx.Exec(c, "UPDATE shopping_items SET stocked=true WHERE id=$1", c.Param("item"))
		return gin.H{"batch_id": batch, "quantity": core.Format(q)}, e
	})
}
