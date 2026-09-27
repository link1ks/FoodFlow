package app

import (
	"context"
	"foodflow/internal/core"
	engine "foodflow/internal/engine/multimeal"
	"net/http/httptest"
	"testing"
	"time"
)

func TestMultiMealScalingUnitsAndExpiryDisplay(t *testing.T) {
	now := time.Date(2026, 9, 27, 0, 0, 0, 0, time.UTC)
	expiry := now.Add(22 * time.Hour)
	s := multiSnapshot{Timezone: "UTC", Recipes: []multiRecipe{{ID: "r", Title: "示例", Servings: 2, Minutes: 10, Items: []multiItem{{Key: "tomato", Name: "番茄", Unit: "g", Quantity: 300000}}}}, Batches: []multiBatch{{ID: "b", Key: "tomato", Name: "番茄", Unit: "kg", Quantity: 1000, Expires: &expiry}}}
	req := multiRequest{Slots: []multiSlot{{"2026-09-27", "dinner"}, {"2026-09-28", "lunch"}}, Servings: 4, MaxMinutes: 30}
	in, err := multiInput(s, req, now)
	if err != nil {
		t.Fatal(err)
	}
	result, err := engine.Plan(in)
	if err != nil {
		t.Fatal(err)
	}
	out := multiOutput(s, req, in, result)
	if out.Meals[0].Uses[0].Quantity != "600.000" || len(out.Meals[0].AtRisk) != 1 || out.Meals[0].AtRisk[0].Quantity != "400.000" || len(out.Meals[0].Rollover) != 0 {
		t.Fatalf("wrong scaling or expiry display %+v", out)
	}
	s.Batches[0].Unit = "个"
	in, err = multiInput(s, req, now)
	if err != nil {
		t.Fatal(err)
	}
	result, err = engine.Plan(in)
	if err != nil || result.Meals[0].Recipe != -1 {
		t.Fatal("count was converted to mass")
	}
}

func TestMultiMealRolloverAdoptionAndFences(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	ctx := context.Background()
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Multi"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Multi", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	var batch string
	for _, item := range []struct{ name, unit, quantity string }{{"番茄", "g", "600"}, {"鸡蛋", "个", "4"}} {
		code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": item.name, "unit": item.unit})
		must(t, code, 201, v)
		code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": item.quantity, "reason": "purchase"})
		must(t, code, 200, v)
		batch = get(v, "batch_id")
	}
	if _, err := pool.Exec(ctx, "UPDATE batches SET expires_at=now()+interval '47 hours' WHERE household_id=$1", house); err != nil {
		t.Fatal(err)
	}
	loc, _ := time.LoadLocation("Asia/Shanghai")
	today := time.Now().In(loc)
	body := map[string]any{"servings": 2, "max_minutes": 30, "slots": []map[string]any{{"day": today.Format("2006-01-02"), "meal": "dinner"}, {"day": today.AddDate(0, 0, 1).Format("2006-01-02"), "meal": "lunch"}}}
	key := core.ID()
	code, v = h.call("POST", root+"/multi-meal", token, key, body)
	must(t, code, 200, v)
	proposal := get(v, "id")
	result := v["result"].(map[string]any)
	meals := result["meals"].([]any)
	if len(meals) != 2 || meals[0].(map[string]any)["recipe_id"] == "" || meals[1].(map[string]any)["recipe_id"] == "" {
		t.Fatalf("missing rollover %+v", result)
	}
	first := meals[0].(map[string]any)
	if len(first["rollover"].([]any)) != 2 {
		t.Fatal("lost leftover batches")
	}
	code, v = h.call("POST", root+"/multi-meal", token, key, body)
	must(t, code, 200, v)
	if get(v, "id") != proposal {
		t.Fatal("duplicate preview")
	}
	code, outsider := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Other"})
	must(t, code, 200, outsider)
	action := root + "/multi-meal/" + proposal
	code, v = h.call("POST", action, get(outsider, "token"), core.ID(), map[string]any{"action": "adopt"})
	must(t, code, 404, v)
	// Another household member may change stock between preview and adoption.
	if _, err := pool.Exec(ctx, "UPDATE batches SET quantity_milli=quantity_milli-1000 WHERE id=$1", batch); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", action, token, core.ID(), map[string]any{"action": "adopt"})
	must(t, code, 409, v)
	var plans int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM plans WHERE household_id=$1", house).Scan(&plans); err != nil || plans != 0 {
		t.Fatal("partial adoption")
	}
	code, v = h.call("POST", action, token, core.ID(), map[string]any{"action": "cancel"})
	must(t, code, 200, v)
	code, v = h.call("POST", action, token, core.ID(), map[string]any{"action": "adopt"})
	must(t, code, 409, v)
	if _, err := pool.Exec(ctx, "UPDATE batches SET quantity_milli=quantity_milli+1000 WHERE id=$1", batch); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/multi-meal", token, core.ID(), body)
	must(t, code, 200, v)
	proposal = get(v, "id")
	action = root + "/multi-meal/" + proposal
	key = core.ID()
	code, v = h.call("POST", action, token, key, map[string]any{"action": "adopt"})
	must(t, code, 200, v)
	plan := get(v, "plan_id")
	code, v = h.call("POST", action, token, key, map[string]any{"action": "adopt"})
	must(t, code, 200, v)
	if get(v, "plan_id") != plan {
		t.Fatal("duplicate plan")
	}
	var n int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM plan_meals WHERE plan_id=$1", plan).Scan(&n); err != nil || n != 2 {
		t.Fatal("not atomic", n, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stock_ledger WHERE household_id=$1 AND delta_milli<0", house).Scan(&n); err != nil || n != 0 {
		t.Fatal("adoption deducted stock")
	}
	// A separate proposal cannot overwrite already confirmed calendar slots.
	code, v = h.call("POST", root+"/multi-meal", token, core.ID(), body)
	must(t, code, 200, v)
	code, v = h.call("POST", root+"/multi-meal/"+get(v, "id"), token, core.ID(), map[string]any{"action": "adopt"})
	must(t, code, 409, v)
	// Full cooking closure uses the existing exact FEFO consumption transaction.
	var mealID string
	if err := pool.QueryRow(ctx, "SELECT id FROM plan_meals WHERE plan_id=$1 ORDER BY day,meal LIMIT 1", plan).Scan(&mealID); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/plans/"+plan+"/meals/"+mealID+"/complete", token, core.ID(), map[string]any{})
	must(t, code, 200, v)
	var eggs int64
	if err := pool.QueryRow(ctx, "SELECT quantity_milli FROM batches WHERE id=$1", batch).Scan(&eggs); err != nil || eggs != 2000 {
		t.Fatal("wrong remaining quantity", eggs, err)
	}
}
