package app

import (
	"context"
	"fmt"
	"foodflow/internal/core"
	"net/http/httptest"
	"testing"
)

func TestMalformedQuantityRejectedWithoutInventoryWrites(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	a := New(pool)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Quantity-Fixture-2026!", "name": "Quantity fixture"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Quantity fixture", "servings": 1})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	var catalog string
	if err := pool.QueryRow(ctx, "SELECT id FROM ingredient_catalog WHERE name='番茄'").Scan(&catalog); err != nil {
		t.Fatal(err)
	}
	for _, quantity := range []string{"1.-1", "1.+1", "0.+1", "1.-01", "+1", "1e3", "1.0001"} {
		code, v = h.call("POST", root+"/catalog-stock", token, core.ID(), map[string]any{"catalog_id": catalog, "quantity": quantity})
		must(t, code, 400, v)
	}
	for _, table := range []string{"ingredients", "batches", "stock_ledger", "stock_outbox"} {
		var count int
		if err := pool.QueryRow(ctx, "SELECT count(*) FROM "+table+" WHERE household_id=$1", house).Scan(&count); err != nil || count != 0 {
			t.Fatalf("invalid quantity mutated %s: count=%d, err=%v", table, count, err)
		}
	}
	key := core.ID()
	stock := map[string]any{"catalog_id": catalog, "quantity": "1.250"}
	code, v = h.call("POST", root+"/catalog-stock", token, key, stock)
	must(t, code, 200, v)
	batch := get(v, "batch_id")
	code, v = h.call("POST", root+"/catalog-stock", token, key, stock)
	must(t, code, 200, v)
	if get(v, "batch_id") != batch {
		t.Fatal("valid quantity replay created another batch")
	}
	var quantity, facts int64
	if err := pool.QueryRow(ctx, "SELECT quantity_milli FROM batches WHERE id=$1", batch).Scan(&quantity); err != nil || quantity != 1250 {
		t.Fatalf("precise valid quantity: %d, %v", quantity, err)
	}
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stock_ledger WHERE household_id=$1", house).Scan(&facts); err != nil || facts != 1 {
		t.Fatalf("replay facts=%d, %v", facts, err)
	}
}

func TestClearFridgeReadsCompleteCandidateBOM(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	a := New(pool)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "BOM-Fixture-2026!", "name": "BOM fixture"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "BOM fixture", "servings": 1})
	must(t, code, 201, v)
	root := "/households/" + get(v, "id")
	rows, err := pool.Query(ctx, `SELECT id::text,name FROM ingredient_catalog WHERE default_unit='g' ORDER BY name COLLATE "C" LIMIT 30`)
	if err != nil {
		t.Fatal(err)
	}
	type material struct{ id, name string }
	materials := []material{}
	for rows.Next() {
		var m material
		if err := rows.Scan(&m.id, &m.name); err != nil {
			t.Fatal(err)
		}
		materials = append(materials, m)
	}
	err = rows.Err()
	rows.Close()
	if err != nil || len(materials) != 30 {
		t.Fatalf("catalog fixture: %d items, %v", len(materials), err)
	}
	selected := []string{}
	for _, m := range materials[:29] {
		code, v = h.call("POST", root+"/catalog-stock", token, core.ID(), map[string]any{"catalog_id": m.id, "quantity": "1"})
		must(t, code, 200, v)
		selected = append(selected, get(v, "ingredient_id"))
	}
	target := ""
	// Each recipe uses 30 catalogue materials, the public contribution maximum.
	// The old joined-row LIMIT 400 kept only 10 of the last recipe's 30 needs.
	for i := 0; i < 14; i++ {
		id := fmt.Sprintf("00000000-0000-4000-8000-%012d", i+1)
		minutes, quantity := 1, int64(100000000)
		if i == 13 {
			minutes, quantity, target = 2, 1000, id
		}
		if _, err := pool.Exec(ctx, "INSERT INTO recipes(id,title,servings,minutes,steps,source) VALUES($1,'BOM fixture',1,$2,ARRAY['test'],'fixture')", id, minutes); err != nil {
			t.Fatal(err)
		}
		for _, m := range materials {
			if _, err := pool.Exec(ctx, "INSERT INTO recipe_items(recipe_id,name,quantity_milli,unit,catalog_id) VALUES($1,$2,$3,'g',$4)", id, m.name, quantity, m.id); err != nil {
				t.Fatal(err)
			}
		}
	}
	request := map[string]any{"selected_ingredient_ids": selected, "servings": 1, "max_minutes": 2}
	code, v = h.call("POST", root+"/clear-fridge", token, "", request)
	must(t, code, 200, v)
	if len(v["recipes"].([]any)) != 0 {
		t.Fatal("recipe recommended despite one missing material beyond joined-row limit")
	}
	last := materials[29]
	code, v = h.call("POST", root+"/catalog-stock", token, core.ID(), map[string]any{"catalog_id": last.id, "quantity": "1"})
	must(t, code, 200, v)
	code, v = h.call("POST", root+"/clear-fridge", token, "", request)
	must(t, code, 200, v)
	chosen := v["recipes"].([]any)
	if len(chosen) != 1 || chosen[0].(map[string]any)["id"] != target || len(v["batch_uses"].([]any)) != 30 {
		t.Fatal("fully stocked recipe must allocate all 30 materials")
	}
}
