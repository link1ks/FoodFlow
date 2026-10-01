package app

import (
	"context"
	"fmt"
	"net/http/httptest"
	"testing"
	"time"

	"foodflow/internal/core"
)

func TestFamilyRecipesBOMAndScheduling(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	app := New(pool)
	defer app.Close()
	server := httptest.NewServer(app.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Recipe-fixture-2026!", "name": "Recipe"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Recipes", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	expectedItems := []int{2, 2, 3, 3, 2, 2, 2, 2, 3, 4, 3, 4}
	for i, wantItems := range expectedItems {
		recipe := fmt.Sprintf("20000000-0000-4000-8000-%012d", i+1)
		t.Run(recipe, func(t *testing.T) {
			var count, invalid, minutes, duration, seasonings int
			if err := pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER(WHERE c.id IS NULL OR ri.name<>c.name OR ri.unit<>c.default_unit OR ri.quantity_milli<=0)
     FROM recipe_items ri LEFT JOIN ingredient_catalog c ON c.id=ri.catalog_id WHERE ri.recipe_id=$1`, recipe).Scan(&count, &invalid); err != nil || count != wantItems || invalid != 0 {
				t.Fatalf("BOM items=%d invalid=%d err=%v", count, invalid, err)
			}
			if err := pool.QueryRow(ctx, `SELECT r.minutes,sum(s.duration_seconds), (SELECT count(*) FROM recipe_seasonings WHERE recipe_id=r.id)
     FROM recipes r JOIN recipe_steps s ON s.recipe_id=r.id WHERE r.id=$1 GROUP BY r.id`, recipe).Scan(&minutes, &duration, &seasonings); err != nil || duration != minutes*60 {
				t.Fatalf("step duration disagrees with advertised estimate: %d/%d %v", duration, minutes, err)
			}
			wantSeasonings := 2
			if i == 10 {
				wantSeasonings = 0
			}
			if seasonings != wantSeasonings {
				t.Fatalf("seasoning coverage=%d want=%d", seasonings, wantSeasonings)
			}
			code, v := h.call("POST", root+"/plans", token, "", map[string]any{"meals": []any{map[string]any{"day": time.Now().AddDate(0, 0, i).Format("2006-01-02"), "meal": "dinner", "servings": 3, "recipe_id": recipe}}})
			must(t, code, 201, v)
			plan := root + "/plans/" + get(v, "id")
			code, v = h.call("GET", plan, token, "", nil)
			must(t, code, 200, v)
			meal := v["meals"].([]any)[0].(map[string]any)["id"].(string)
			code, v = h.call("POST", plan+"/confirm", token, core.ID(), map[string]any{"revision": v["revision"], "accept_uncertain": false})
			must(t, code, 200, v)
			path := root + "/meals/" + meal + "/pipeline"
			body := map[string]any{"target_at": time.Now().Add(2 * time.Hour), "second_stove": false}
			code, v = h.call("POST", path, token, core.ID(), body)
			must(t, code, 200, v)
			code, v = h.call("GET", path, token, "", nil)
			must(t, code, 200, v)
			schedule := v["session"].(map[string]any)["schedule"].(map[string]any)
			if schedule["deadline_met"] != true || int(schedule["duration"].(float64)) != duration {
				t.Fatalf("invalid schedule: %+v", schedule)
			}
			steps := schedule["steps"].([]any)
			previousEnd := 0
			for _, raw := range steps {
				step := raw.(map[string]any)
				if int(step["start"].(float64)) < previousEnd {
					t.Fatal("dependent steps overlap")
				}
				previousEnd = int(step["end"].(float64))
			}
		})
	}
	var ledger int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stock_ledger WHERE household_id=$1", house).Scan(&ledger); err != nil || ledger != 0 {
		t.Fatalf("planning or pipeline deducted stock: %d %v", ledger, err)
	}
}
