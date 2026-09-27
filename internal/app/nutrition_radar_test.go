package app

import (
	"context"
	"foodflow/internal/core"
	"math"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNutritionRadarSnapshotsCoverageAndScope(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Radar"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Radar", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	code, v = h.call("GET", root+"/nutrition", token, "", nil)
	must(t, code, 200, v)
	for _, x := range v["daily"].([]any) {
		if x.(map[string]any)["energy"] != nil {
			t.Fatal("missing days must not be zero")
		}
	}
	code, v = h.call("GET", root+"/nutrition?days=8", token, "", nil)
	must(t, code, 400, v)
	for _, i := range []struct{ name, unit, q string }{{"番茄", "g", "600"}, {"鸡蛋", "个", "4"}} {
		code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": i.name, "unit": i.unit})
		must(t, code, 201, v)
		code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": i.q, "reason": "purchase"})
		must(t, code, 200, v)
	}
	recipe := core.ID()
	_, err := pool.Exec(ctx, `INSERT INTO recipes(id,title,servings,minutes,steps,source) VALUES($1,'Radar fixture',2,10,ARRAY['cook'],'test');`, recipe)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO recipe_items(recipe_id,name,quantity_milli,unit) VALUES($1,'番茄',300000,'g'),($1,'鸡蛋',2000,'个')`, recipe)
	if err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/plans", token, "", map[string]any{"meals": []any{map[string]any{"day": time.Now().Format("2006-01-02"), "meal": "lunch", "servings": 2, "recipe_id": recipe}}})
	must(t, code, 201, v)
	plan := get(v, "id")
	code, v = h.call("POST", root+"/plans/"+plan+"/confirm", token, core.ID(), map[string]any{"revision": 1})
	must(t, code, 200, v)
	var meal string
	if err = pool.QueryRow(ctx, "SELECT id FROM plan_meals WHERE plan_id=$1", plan).Scan(&meal); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/plans/"+plan+"/meals/"+meal+"/complete", token, core.ID(), map[string]any{})
	must(t, code, 200, v)
	check := func() {
		t.Helper()
		code, v = h.call("GET", root+"/nutrition?days=30", token, "", nil)
		must(t, code, 200, v)
		ds := v["daily"].([]any)
		if len(ds) != 30 {
			t.Fatal("wrong period")
		}
		found := false
		for _, x := range ds {
			d := x.(map[string]any)
			if d["meals"].(float64) > 0 {
				found = true
				if math.Abs(d["energy"].(float64)-27) > 0.0001 || d["lines"] != float64(2) || d["known_lines"] != float64(1) || d["protein_low_streak"] != false {
					t.Fatal("bad nutrition", d)
				}
			}
		}
		if !found || len(v["excluded"].([]any)) != 1 {
			t.Fatal("missing coverage", v)
		}
	}
	check()
	// Later metadata and household edits must not rewrite what was cooked.
	_, err = pool.Exec(ctx, `UPDATE ingredient_nutrition_profiles SET profile=jsonb_set(profile,'{nutrients,energy_kcal}','"999"') WHERE catalog_id IN(SELECT id FROM ingredient_catalog WHERE name='番茄')`)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `UPDATE households SET servings=8 WHERE id=$1`, house)
	if err != nil {
		t.Fatal(err)
	}
	check()
	code, v = h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Other"})
	must(t, code, 200, v)
	code, v = h.call("GET", root+"/nutrition", get(v, "token"), "", nil)
	must(t, code, 404, v)
	// The actual completion date, rather than the scheduled meal day, controls the window.
	_, err = pool.Exec(ctx, `UPDATE meal_completions SET completed_at=now()-interval '31 days' WHERE household_id=$1`, house)
	if err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", root+"/nutrition?days=30", token, "", nil)
	must(t, code, 200, v)
	for _, x := range v["daily"].([]any) {
		if x.(map[string]any)["meals"] != float64(0) {
			t.Fatal("old completion included")
		}
	}
}

func TestNutritionRadarThreeDayWindowRequiresCompleteRecords(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Window"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Window", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": "番茄", "unit": "g"})
	must(t, code, 201, v)
	code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": "3000", "reason": "purchase"})
	must(t, code, 200, v)
	batch := get(v, "batch_id")
	var owner string
	if err := pool.QueryRow(ctx, "SELECT owner_id FROM households WHERE id=$1", house).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	plan := core.ID()
	_, err := pool.Exec(ctx, "INSERT INTO plans(id,household_id,status,created_by) VALUES($1,$2,'consumed',$3)", plan, house, owner)
	if err != nil {
		t.Fatal(err)
	}
	// Persisted historical fixtures exercise the SQL window over actual completion days.
	for day := 0; day < 3; day++ {
		for _, slot := range []string{"breakfast", "lunch", "dinner"} {
			meal := core.ID()
			_, err = pool.Exec(ctx, `INSERT INTO plan_meals(id,plan_id,day,meal,servings,recipe_id) VALUES($1,$2,(now() AT TIME ZONE 'Asia/Shanghai')::date-$3::int,$4,2,'10000000-0000-4000-8000-000000000001')`, meal, plan, day, slot)
			if err != nil {
				t.Fatal(err)
			}
			_, err = pool.Exec(ctx, `INSERT INTO meal_completions(id,household_id,plan_meal_id,day,meal,actor_id,completed_at) SELECT $1,$2,id,day,meal,$4,now()-make_interval(days=>$5::int) FROM plan_meals WHERE id=$3`, core.ID(), house, meal, owner, day)
			if err != nil {
				t.Fatal(err)
			}
			_, err = pool.Exec(ctx, `INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,ref_type,ref_id) VALUES($1,$2,$3,-300000,'consume',$4,'plan_meal',$5)`, core.ID(), house, batch, owner, meal)
			if err != nil {
				t.Fatal(err)
			}
		}
	}
	assertFlag := func(want bool) {
		t.Helper()
		code, v = h.call("GET", root+"/nutrition", token, "", nil)
		must(t, code, 200, v)
		ds := v["daily"].([]any)
		if ds[len(ds)-1].(map[string]any)["protein_low_streak"] != want {
			t.Fatal("wrong three-day rule", v)
		}
	}
	assertFlag(true)
	// A missing middle day must break the streak, even with three other recorded days.
	_, err = pool.Exec(ctx, `UPDATE meal_completions SET completed_at=completed_at-interval '5 days' WHERE household_id=$1 AND (completed_at AT TIME ZONE 'Asia/Shanghai')::date=(now() AT TIME ZONE 'Asia/Shanghai')::date-1`, house)
	if err != nil {
		t.Fatal(err)
	}
	assertFlag(false)
}
