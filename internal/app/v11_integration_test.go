package app

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"foodflow/internal/core"
)

func TestMealCompletionFEFOIdempotenceAndReplenishment(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Cook"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Kitchen", "servings": 2})
	must(t, code, 201, v)
	root := "/households/" + get(v, "id")
	code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": "番茄", "unit": "g"})
	must(t, code, 201, v)
	tomato := get(v, "id")
	code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": "鸡蛋", "unit": "个", "low": "3"})
	must(t, code, 201, v)
	egg := get(v, "id")
	yesterday := time.Now().AddDate(0, 0, -2).Format("2006-01-02")
	soon := time.Now().AddDate(0, 0, 2).Format("2006-01-02")
	later := time.Now().AddDate(0, 0, 4).Format("2006-01-02")
	stock := func(ingredient, quantity, expires string) string {
		code, v := h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": ingredient, "quantity": quantity, "reason": "purchase", "expires_on": expires, "expiry_kind": "user"})
		must(t, code, 200, v)
		return get(v, "batch_id")
	}
	expired := stock(tomato, "500", yesterday)
	code, _ = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": tomato, "batch_id": expired, "quantity": "1", "reason": "consume"})
	if code != 409 {
		t.Fatalf("expired batch was consumed: %d", code)
	}
	first := stock(tomato, "100", soon)
	second := stock(tomato, "200", later)
	_ = stock(egg, "2", soon)
	code, recommendation := h.call("POST", root+"/clear-fridge", token, "", map[string]any{"selected_ingredient_ids": []string{tomato, egg}, "servings": 2, "max_minutes": 20})
	must(t, code, 200, recommendation)
	if chosen, ok := recommendation["recipes"].([]any); !ok || len(chosen) == 0 {
		t.Fatalf("no clear-fridge recommendation: %+v", recommendation)
	}
	var ledgerBeforePlanning int
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_ledger").Scan(&ledgerBeforePlanning)
	if ledgerBeforePlanning != 4 {
		t.Fatalf("read-only recommendation changed stock ledger: %d", ledgerBeforePlanning)
	}
	code, outsider := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Other"})
	must(t, code, 200, outsider)
	code, _ = h.call("POST", root+"/clear-fridge", get(outsider, "token"), "", map[string]any{"selected_ingredient_ids": []string{tomato}, "servings": 2, "max_minutes": 20})
	if code != 404 {
		t.Fatalf("cross-household clear-fridge access returned %d", code)
	}
	day := time.Now().Format("2006-01-02")
	code, v = h.call("POST", root+"/plans", token, "", map[string]any{"meals": []any{map[string]any{"day": day, "meal": "dinner", "servings": 2, "recipe_id": "10000000-0000-4000-8000-000000000001"}}})
	must(t, code, 201, v)
	plan := get(v, "id")
	code, detail := h.call("GET", root+"/plans/"+plan, token, "", nil)
	must(t, code, 200, detail)
	meals, ok := detail["meals"].([]any)
	if !ok || len(meals) != 1 {
		t.Fatalf("missing plan meal: %+v", detail)
	}
	mealID := get(meals[0].(map[string]any), "id")
	code, v = h.call("POST", root+"/plans/"+plan+"/confirm", token, core.ID(), map[string]any{"revision": 1, "accept_uncertain": false})
	must(t, code, 200, v)
	path := root + "/plans/" + plan + "/meals/" + mealID + "/complete"
	type completionResult struct {
		code int
		key  string
	}
	results := make(chan completionResult, 2)
	for i := 0; i < 2; i++ {
		go func(key string) {
			status, _ := h.call("POST", path, token, key, map[string]any{})
			results <- completionResult{status, key}
		}(core.ID())
	}
	firstResult, secondResult := <-results, <-results
	if (firstResult.code != 200 || secondResult.code != 409) && (firstResult.code != 409 || secondResult.code != 200) {
		t.Fatalf("concurrent completion statuses %d and %d", firstResult.code, secondResult.code)
	}
	key := firstResult.key
	if firstResult.code != 200 {
		key = secondResult.key
	}
	code, replay := h.call("POST", path, token, key, map[string]any{})
	must(t, code, 200, replay)
	code, _ = h.call("POST", path, token, core.ID(), map[string]any{})
	if code != 409 {
		t.Fatalf("second meal completion with new key: %d", code)
	}
	var completionCount, outboundCount int
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM meal_completions WHERE plan_meal_id=$1", mealID).Scan(&completionCount)
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_ledger WHERE ref_type='plan_meal' AND ref_id=$1", mealID).Scan(&outboundCount)
	if completionCount != 1 || outboundCount != 3 {
		t.Fatalf("completion=%d outbound=%d", completionCount, outboundCount)
	}
	var capturedServings int
	var capturedTitle string
	if err := pool.QueryRow(context.Background(), `SELECT s.servings,s.dishes->0->>'title' FROM meal_completion_snapshots s JOIN meal_completions mc ON mc.id=s.completion_id WHERE mc.plan_meal_id=$1`, mealID).Scan(&capturedServings, &capturedTitle); err != nil || capturedServings != 2 || capturedTitle == "" {
		t.Fatalf("completion snapshot: %d %s %v", capturedServings, capturedTitle, err)
	}
	if _, err := pool.Exec(context.Background(), "UPDATE meal_completion_snapshots SET servings=9 WHERE completion_id=(SELECT id FROM meal_completions WHERE plan_meal_id=$1)", mealID); err == nil {
		t.Fatal("completion snapshot was mutable")
	}
	var expiredQty, firstQty, secondQty int64
	_ = pool.QueryRow(context.Background(), "SELECT quantity_milli FROM batches WHERE id=$1", expired).Scan(&expiredQty)
	_ = pool.QueryRow(context.Background(), "SELECT quantity_milli FROM batches WHERE id=$1", first).Scan(&firstQty)
	_ = pool.QueryRow(context.Background(), "SELECT quantity_milli FROM batches WHERE id=$1", second).Scan(&secondQty)
	if expiredQty != 500000 || firstQty != 0 || secondQty != 0 {
		t.Fatalf("FEFO amounts: expired=%d first=%d second=%d", expiredQty, firstQty, secondQty)
	}
	code, shopping := h.list(root+"/shopping", token)
	if code != 200 {
		t.Fatalf("shopping status %d", code)
	}
	foundSuggestion := ""
	for _, item := range shopping {
		if item["origin"] == "system" && item["name"] == "鸡蛋" && item["needed"] == "3.000" {
			foundSuggestion = get(item, "id")
		}
	}
	if foundSuggestion == "" {
		t.Fatalf("missing low-stock suggestion: %+v", shopping)
	}
	code, v = h.call("PATCH", root+"/shopping/"+foundSuggestion, token, "", map[string]any{"checked": true, "bought": "3"})
	must(t, code, 204, v)
	stockKey := core.ID()
	stockBody := map[string]any{"location": "冷藏", "bought_on": day, "expires_on": later, "expiry_kind": "estimate", "source": "补货测试"}
	code, v = h.call("POST", root+"/shopping/"+foundSuggestion+"/stock", token, stockKey, stockBody)
	must(t, code, 200, v)
	code, v = h.call("POST", root+"/shopping/"+foundSuggestion+"/stock", token, stockKey, stockBody)
	must(t, code, 200, v)
	var suggestionInbound int
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_ledger WHERE ref_type='shopping_item' AND ref_id=$1", foundSuggestion).Scan(&suggestionInbound)
	if suggestionInbound != 1 {
		t.Fatalf("system suggestion stocked %d times", suggestionInbound)
	}
	// Correction uses an absolute balance with stale-write protection.
	code, _ = h.call("PATCH", root+"/batches/"+expired+"/quantity", token, core.ID(), map[string]any{"expected_quantity": "500", "target_quantity": "0"})
	if code != 200 {
		t.Fatalf("correction status %d", code)
	}
	code, _ = h.call("PATCH", root+"/batches/"+expired+"/quantity", token, core.ID(), map[string]any{"expected_quantity": "500", "target_quantity": "100"})
	if code != 409 {
		t.Fatalf("stale correction accepted: %d", code)
	}
}

func TestMultiDishMealCombinesDemandAndRollsBackOnSharedShortage(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Combo"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Combo home", "servings": 2})
	must(t, code, 201, v)
	root := "/households/" + get(v, "id")
	for _, ingredient := range []struct{ name, unit, quantity string }{{"番茄", "g", "300"}, {"鸡蛋", "个", "3"}} {
		code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": ingredient.name, "unit": ingredient.unit})
		must(t, code, 201, v)
		code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": ingredient.quantity, "reason": "purchase"})
		must(t, code, 200, v)
	}
	extraID := core.ID()
	_, err := pool.Exec(context.Background(), "INSERT INTO recipes(id,title,servings,minutes,steps,source) VALUES($1,'蛋花汤',2,8,ARRAY['煮熟'],'测试菜谱')", extraID)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), "INSERT INTO recipe_items(recipe_id,name,quantity_milli,unit) VALUES($1,'鸡蛋',2000,'个')", extraID)
	if err != nil {
		t.Fatal(err)
	}
	day := time.Now().Format("2006-01-02")
	primaryID := "10000000-0000-4000-8000-000000000001"
	code, v = h.call("POST", root+"/plans", token, "", map[string]any{"meals": []any{map[string]any{"day": day, "meal": "dinner", "servings": 2, "recipe_id": primaryID, "additional_recipe_ids": []string{extraID}}}})
	must(t, code, 201, v)
	planID := get(v, "id")
	readPlan := func() (string, map[string]any) {
		code, detail := h.call("GET", root+"/plans/"+planID, token, "", nil)
		must(t, code, 200, detail)
		meals := detail["meals"].([]any)
		if len(meals) != 1 || len(meals[0].(map[string]any)["additional_recipes"].([]any)) != 1 {
			t.Fatalf("additional dish missing: %+v", detail)
		}
		return get(meals[0].(map[string]any), "id"), detail
	}
	mealID, detail := readPlan()
	checkEgg := func(detail map[string]any, needed, shortage string) {
		t.Helper()
		for _, raw := range detail["ingredients"].([]any) {
			d := raw.(map[string]any)
			if d["name"] == "鸡蛋" {
				if d["needed"] != needed || d["shortage"] != shortage {
					t.Fatalf("egg demand: %+v", d)
				}
				return
			}
		}
		t.Fatal("egg demand missing")
	}
	checkEgg(detail, "4.000", "1.000")
	code, _ = h.call("PATCH", root+"/plans/"+planID+"/meals/"+mealID, token, "", map[string]any{"recipe_id": primaryID, "servings": 4})
	if code != 204 {
		t.Fatalf("servings update: %d", code)
	}
	_, detail = readPlan()
	checkEgg(detail, "8.000", "5.000")
	code, _ = h.call("PATCH", root+"/plans/"+planID+"/meals/"+mealID, token, "", map[string]any{"recipe_id": primaryID, "servings": 2})
	if code != 204 {
		t.Fatalf("servings reset: %d", code)
	}
	code, v = h.call("POST", root+"/plans/"+planID+"/confirm", token, core.ID(), map[string]any{"revision": 3, "accept_uncertain": false})
	must(t, code, 200, v)
	path := root + "/plans/" + planID + "/meals/" + mealID + "/complete"
	code, _ = h.call("POST", path, token, core.ID(), map[string]any{})
	if code != 409 {
		t.Fatalf("shared egg shortage should block entire meal: %d", code)
	}
	var outbound, completed int
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_ledger WHERE ref_type='plan_meal' AND ref_id=$1", mealID).Scan(&outbound)
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM meal_completions WHERE plan_meal_id=$1", mealID).Scan(&completed)
	if outbound != 0 || completed != 0 {
		t.Fatalf("partial completion persisted: ledger=%d completion=%d", outbound, completed)
	}
	var eggID string
	_ = pool.QueryRow(context.Background(), "SELECT id FROM ingredients WHERE household_id=$1 AND name='鸡蛋'", root[len("/households/"):]).Scan(&eggID)
	code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": eggID, "quantity": "1", "reason": "purchase"})
	must(t, code, 200, v)
	code, v = h.call("POST", path, token, core.ID(), map[string]any{})
	must(t, code, 200, v)
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_ledger WHERE ref_type='plan_meal' AND ref_id=$1", mealID).Scan(&outbound)
	if outbound != 3 {
		t.Fatalf("combined dish deductions: %d", outbound)
	}
	code, today := h.list(root+"/today?day="+day, token)
	if code != 200 || len(today) == 0 || today[0]["title"] != "番茄炒蛋 + 蛋花汤" {
		t.Fatalf("today combo missing: %d %+v", code, today)
	}
}
