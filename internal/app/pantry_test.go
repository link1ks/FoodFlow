package app

import (
	"context"
	"foodflow/internal/core"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVirtualPantryCookingCalibrationAndReplenishment(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Pantry"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Pantry", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	batch := ""
	for _, item := range []struct{ name, unit, quantity string }{{"生菜", "g", "900"}, {"蒜", "g", "60"}, {"食用油", "ml", "40"}} {
		code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": item.name, "unit": item.unit})
		must(t, code, 201, v)
		ingredient := get(v, "id")
		code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": ingredient, "quantity": item.quantity, "reason": "purchase"})
		must(t, code, 200, v)
		batch = get(v, "batch_id")
	}
	enable := map[string]any{"batch_id": batch, "capacity": "100", "threshold": 15, "accept_estimates": false}
	code, v = h.call("POST", root+"/pantry", token, core.ID(), enable)
	must(t, code, 400, v)
	enable["accept_estimates"] = true
	key := core.ID()
	code, v = h.call("POST", root+"/pantry", token, key, enable)
	must(t, code, 200, v)
	pantry := get(v, "id")
	code, v = h.call("POST", root+"/pantry", token, key, enable)
	must(t, code, 200, v)
	code, other := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Other"})
	must(t, code, 200, other)
	code, v = h.call("GET", root+"/pantry", get(other, "token"), "", nil)
	must(t, code, 404, v)
	today := time.Now().Format("2006-01-02")
	tomorrow := time.Now().AddDate(0, 0, 1).Format("2006-01-02")
	code, v = h.call("POST", root+"/plans", token, "", map[string]any{"meals": []any{map[string]any{"day": today, "meal": "lunch", "servings": 2, "recipe_id": "10000000-0000-4000-8000-000000000004"}, map[string]any{"day": today, "meal": "dinner", "servings": 2, "recipe_id": "10000000-0000-4000-8000-000000000004"}, map[string]any{"day": tomorrow, "meal": "lunch", "servings": 2, "recipe_id": "10000000-0000-4000-8000-000000000004"}}})
	must(t, code, 201, v)
	plan := get(v, "id")
	code, v = h.call("POST", root+"/plans/"+plan+"/confirm", token, core.ID(), map[string]any{"revision": 1})
	must(t, code, 200, v)
	rows, err := pool.Query(ctx, "SELECT id FROM plan_meals WHERE plan_id=$1 ORDER BY day,meal", plan)
	if err != nil {
		t.Fatal(err)
	}
	meals := []string{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}
		meals = append(meals, id)
	}
	rows.Close()
	type outcome struct {
		code int
		body map[string]any
	}
	done := make(chan outcome, 2)
	for _, meal := range meals[:2] {
		go func(id string) {
			c, v := h.call("POST", root+"/plans/"+plan+"/meals/"+id+"/complete", token, core.ID(), map[string]any{})
			done <- outcome{c, v}
		}(meal)
	}
	for i := 0; i < 2; i++ {
		r := <-done
		must(t, r.code, 200, r.body)
	}
	var quantity int64
	if err = pool.QueryRow(ctx, "SELECT quantity_milli FROM batches WHERE id=$1", batch).Scan(&quantity); err != nil || quantity != 10000 {
		t.Fatal("wrong concurrent depletion", quantity, err)
	}
	var n int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM stock_ledger WHERE batch_id=$1 AND is_estimated AND reason='consume'", batch).Scan(&n); err != nil || n != 2 {
		t.Fatal("missing estimated ledger", n, err)
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM shopping_items i JOIN shopping_lists l ON l.id=i.list_id WHERE l.household_id=$1 AND i.name='食用油' AND i.origin='system'", house).Scan(&n); err != nil || n != 1 {
		t.Fatal("replenishment missing or duplicated", n, err)
	}
	path := root + "/pantry/" + pantry
	code, v = h.call("POST", path, token, core.ID(), map[string]any{"action": "calibrate", "expected_quantity": "40.000", "percent": 2})
	must(t, code, 409, v)
	key = core.ID()
	body := map[string]any{"action": "calibrate", "expected_quantity": "10.000", "percent": 2}
	code, v = h.call("POST", path, token, key, body)
	must(t, code, 200, v)
	code, v = h.call("POST", path, token, key, body)
	must(t, code, 200, v)
	if _, err = pool.Exec(ctx, "UPDATE batches SET expires_at=now()-interval '1 hour' WHERE id=$1", batch); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/plans/"+plan+"/meals/"+meals[2]+"/complete", token, core.ID(), map[string]any{})
	must(t, code, 409, v)
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM meal_completions WHERE plan_meal_id=$1", meals[2]).Scan(&n); err != nil || n != 0 {
		t.Fatal("expired seasoning left completion")
	}
	var lettuce int64
	if err = pool.QueryRow(ctx, "SELECT sum(b.quantity_milli) FROM batches b JOIN ingredients i ON i.id=b.ingredient_id WHERE i.household_id=$1 AND i.name='生菜'", house).Scan(&lettuce); err != nil || lettuce != 300000 {
		t.Fatal("food deduction did not roll back", lettuce, err)
	}
	if _, err = pool.Exec(ctx, "UPDATE batches SET expires_at=NULL WHERE id=$1", batch); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, "UPDATE households SET excluded_ingredients=ARRAY['食用油'] WHERE id=$1", house); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/plans/"+plan+"/meals/"+meals[2]+"/complete", token, core.ID(), map[string]any{})
	must(t, code, 409, v)
	if _, err = pool.Exec(ctx, "UPDATE households SET excluded_ingredients='{}' WHERE id=$1", house); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/plans/"+plan+"/meals/"+meals[2]+"/complete", token, core.ID(), map[string]any{})
	must(t, code, 200, v)
	var requested, deducted int64
	if err = pool.QueryRow(ctx, "SELECT requested_milli,deducted_milli FROM pantry_consumptions WHERE pantry_id=$1 AND plan_meal_id=$2", pantry, meals[2]).Scan(&requested, &deducted); err != nil || requested != 15000 || deducted != 2000 {
		t.Fatal("shortfall not retained", requested, deducted, err)
	}
	if err = pool.QueryRow(ctx, "SELECT quantity_milli FROM batches WHERE id=$1", batch).Scan(&quantity); err != nil || quantity != 0 {
		t.Fatal("negative balance", quantity, err)
	}
	if _, err = pool.Exec(ctx, "DELETE FROM pantry_consumptions WHERE pantry_id=$1", pantry); err == nil {
		t.Fatal("mutable audit")
	}
	code, v = h.call("POST", path, token, core.ID(), map[string]any{"action": "disable"})
	must(t, code, 200, v)
	code, v = h.call("POST", path, token, core.ID(), map[string]any{"action": "calibrate", "expected_quantity": "0.000", "percent": 50})
	must(t, code, 409, v)
	code, v = h.call("POST", root+"/pantry", token, core.ID(), enable)
	must(t, code, 200, v)
	if get(v, "id") != pantry {
		t.Fatal("reactivation lost bottle identity")
	}
}
