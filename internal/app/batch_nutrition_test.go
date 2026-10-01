package app

import (
	"context"
	"foodflow/internal/core"
	"math"
	"net/http/httptest"
	"testing"
)

func TestConfirmedNutritionBasisPreservesHistoryAndScope(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	a := New(db)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Nutrition-fixture-2026!", "name": "Nutrition"})
	must(t, code, 200, v)
	token, user := get(v, "token"), get(v, "user_id")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Nutrition", "servings": 1})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	batches := map[string]string{}
	for _, item := range []struct{ name, unit, quantity string }{{"鸡蛋", "个", "20"}, {"牛奶", "ml", "1000"}} {
		code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": item.name, "unit": item.unit})
		must(t, code, 201, v)
		code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": item.quantity, "reason": "purchase"})
		must(t, code, 200, v)
		batches[item.name] = get(v, "batch_id")
	}
	recipe := core.ID()
	if _, err := db.Exec(ctx, "INSERT INTO recipes(id,title,servings,minutes,steps,source) VALUES($1,'营养 fixture',1,10,ARRAY['cook'],'fixture')", recipe); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(ctx, "INSERT INTO recipe_items(recipe_id,name,unit,quantity_milli) VALUES($1,'鸡蛋','个',2000),($1,'牛奶','ml',100000)", recipe); err != nil {
		t.Fatal(err)
	}
	var day string
	if err := db.QueryRow(ctx, "SELECT (now() AT TIME ZONE 'Asia/Shanghai')::date::text").Scan(&day); err != nil {
		t.Fatal(err)
	}
	complete := func(slot string) {
		t.Helper()
		code, v = h.call("POST", root+"/plans", token, "", map[string]any{"meals": []any{map[string]any{"day": day, "meal": slot, "servings": 1, "recipe_id": recipe}}})
		must(t, code, 201, v)
		plan := get(v, "id")
		code, v = h.call("POST", root+"/plans/"+plan+"/confirm", token, core.ID(), map[string]any{"revision": 1})
		must(t, code, 200, v)
		var meal string
		if err := db.QueryRow(ctx, "SELECT id FROM plan_meals WHERE plan_id=$1", plan).Scan(&meal); err != nil {
			t.Fatal(err)
		}
		code, v = h.call("POST", root+"/plans/"+plan+"/meals/"+meal+"/complete", token, core.ID(), map[string]any{})
		must(t, code, 200, v)
	}
	complete("breakfast")
	assertReport := func(known, confirmed float64, want float64) {
		t.Helper()
		code, v = h.call("GET", root+"/nutrition", token, "", nil)
		must(t, code, 200, v)
		for _, x := range v["daily"].([]any) {
			d := x.(map[string]any)
			if d["meals"].(float64) > 0 {
				if d["known_lines"] != known || d["confirmed_mass_lines"] != confirmed {
					t.Fatal("wrong historical coverage", d)
				}
				if known == 0 {
					if d["energy"] != nil {
						t.Fatal("unknown mass became zero")
					}
				} else if math.Abs(d["energy"].(float64)-want) > 0.001 {
					t.Fatal("wrong exact mass conversion", d)
				}
				return
			}
		}
		t.Fatal("no completion")
	}
	assertReport(0, 0, 0)
	confirm := func(name, q, g string, revision int) {
		t.Helper()
		path := root + "/batches/" + batches[name] + "/nutrition-basis"
		body := map[string]any{"quantity": q, "edible_grams": g, "classification": "catalog", "expected_revision": revision, "confirm": true}
		key := core.ID()
		code, v = h.call("POST", path, token, key, body)
		must(t, code, 200, v)
		code, v = h.call("POST", path, token, key, body)
		must(t, code, 200, v)
		code, v = h.call("POST", path, token, core.ID(), body)
		must(t, code, 409, v)
	}
	path := root + "/batches/" + batches["鸡蛋"] + "/nutrition-basis"
	code, v = h.call("POST", path, token, core.ID(), map[string]any{"quantity": "2", "edible_grams": "100", "classification": "catalog", "expected_revision": 0, "confirm": false})
	must(t, code, 400, v)
	confirm("鸡蛋", "2", "100", 0)
	confirm("牛奶", "250", "260", 0)
	assertReport(0, 0, 0)
	complete("lunch")
	assertReport(2, 2, 206.44)
	confirm("鸡蛋", "2", "80", 1)
	assertReport(2, 2, 206.44)
	complete("dinner")
	assertReport(4, 4, 384.28)
	var count int
	var sum int64
	if err := db.QueryRow(ctx, "SELECT count(*),sum(quantity_milli)::bigint FROM batch_nutrition_confirmations WHERE household_id=$1", house).Scan(&count, &sum); err != nil || count != 3 {
		t.Fatal("idempotency added confirmations", count, sum, err)
	}
	if _, err := db.Exec(ctx, "UPDATE batch_nutrition_confirmations SET edible_grams_milli=1 WHERE household_id=$1", house); err == nil {
		t.Fatal("historical basis was mutable")
	}
	code, v = h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Nutrition-fixture-2026!", "name": "Other"})
	must(t, code, 200, v)
	other := get(v, "token")
	code, v = h.call("GET", path, other, "", nil)
	must(t, code, 404, v)
	if _, err := db.Exec(ctx, "INSERT INTO members(household_id,user_id,role) SELECT $1,id,'viewer' FROM users WHERE id<>$2", house, user); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", path, other, core.ID(), map[string]any{"quantity": "2", "edible_grams": "80", "classification": "unknown", "expected_revision": 2, "confirm": true})
	must(t, code, 403, v)
	// Classification and edible fraction are frozen with each future outbound.
	code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"name": "小白菜", "unit": "g"})
	must(t, code, 201, v)
	vegetable := get(v, "id")
	code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": vegetable, "quantity": "300", "reason": "purchase"})
	must(t, code, 200, v)
	vegetableBatch := get(v, "batch_id")
	vegetablePath := root + "/batches/" + vegetableBatch + "/nutrition-basis"
	for revision, classification := range []string{"dark", "unknown"} {
		code, v = h.call("POST", vegetablePath, token, core.ID(), map[string]any{"quantity": "100", "edible_grams": "50", "classification": classification, "expected_revision": revision, "confirm": true})
		must(t, code, 200, v)
		code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": vegetable, "batch_id": vegetableBatch, "quantity": "50", "reason": "consume"})
		must(t, code, 200, v)
	}
	var classified, unknown int
	if err := db.QueryRow(ctx, `SELECT count(*) FILTER(WHERE s.is_dark_vegetable=true),count(*) FILTER(WHERE s.is_dark_vegetable IS NULL)
 FROM stock_ledger l JOIN stock_ledger_snapshots s ON s.ledger_id=l.id
 WHERE l.batch_id=$1 AND l.delta_milli<0 AND s.classification_source='household-confirmed' AND s.confirmed_edible_grams_milli=50000`, vegetableBatch).Scan(&classified, &unknown); err != nil || classified != 1 || unknown != 1 {
		t.Fatal("later classification rewrote earlier snapshot", classified, unknown, err)
	}
	code, v = h.call("POST", vegetablePath, token, core.ID(), map[string]any{"quantity": "100", "edible_grams": "101", "classification": "catalog", "expected_revision": 2, "confirm": true})
	must(t, code, 400, v)
}
