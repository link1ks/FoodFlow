package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"foodflow/internal/core"
)

func TestV2LedgerSnapshotAndOperationIdempotency(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Snapshot"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Snapshot kitchen", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	var catalog string
	if err := pool.QueryRow(ctx, "SELECT id FROM ingredient_catalog WHERE name='番茄'").Scan(&catalog); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/catalog-stock", token, core.ID(), map[string]any{"catalog_id": catalog, "quantity": "100"})
	must(t, code, 200, v)
	batch, ingredient := get(v, "batch_id"), get(v, "ingredient_id")
	key := core.ID()
	payload := map[string]any{"ingredient_id": ingredient, "batch_id": batch, "quantity": "10", "reason": "consume"}
	code, v = h.call("POST", root+"/stock", token, key, payload)
	must(t, code, 200, v)
	code, v = h.call("POST", root+"/stock", token, key, payload)
	must(t, code, 200, v)
	var ledger string
	var energy string
	if err := pool.QueryRow(ctx, `SELECT s.ledger_id,s.nutrition_profile->'nutrients'->>'energy_kcal' FROM stock_ledger_snapshots s JOIN stock_ledger l ON l.id=s.ledger_id WHERE s.household_id=$1 AND l.reason='consume'`, house).Scan(&ledger, &energy); err != nil {
		t.Fatal(err)
	}
	if energy != "18" {
		t.Fatalf("missing reference snapshot %q", energy)
	}
	if _, err := pool.Exec(ctx, "UPDATE stock_ledger_snapshots SET ingredient_name='changed' WHERE ledger_id=$1", ledger); err == nil {
		t.Fatal("snapshot was mutable")
	}
	if _, err := pool.Exec(ctx, "DELETE FROM stock_ledger_snapshots WHERE ledger_id=$1", ledger); err == nil {
		t.Fatal("snapshot was deletable")
	}
	// Same household/body/key sent to a different resource is not a replay.
	if _, err := pool.Exec(ctx, "UPDATE ingredients SET name='改名番茄' WHERE id=$1", ingredient); err != nil {
		t.Fatal(err)
	}
	var name string
	if err := pool.QueryRow(ctx, "SELECT ingredient_name FROM stock_ledger_snapshots WHERE ledger_id=$1", ledger).Scan(&name); err != nil || name != "番茄" {
		t.Fatalf("historical name changed %s %v", name, err)
	}
	var count int
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stock_ledger WHERE household_id=$1 AND reason='consume'", house).Scan(&count); err != nil || count != 1 {
		t.Fatalf("duplicate writes: %d %v", count, err)
	}
	// Failed writes roll back snapshot rows along with ledger and stock changes.
	code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": ingredient, "batch_id": batch, "quantity": "1000", "reason": "consume"})
	must(t, code, 409, v)
	if err := pool.QueryRow(ctx, "SELECT count(*) FROM stock_ledger_snapshots WHERE household_id=$1", house).Scan(&count); err != nil || count != 2 {
		t.Fatalf("failed operation left snapshot: %d %v", count, err)
	}
	costPath := root + "/batches/" + batch + "/cost"
	code, v = h.call("GET", costPath, token, "", nil)
	must(t, code, 200, v)
	if v["recorded"] != false {
		t.Fatal("missing price represented as known")
	}
	code, other := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Other"})
	must(t, code, 200, other)
	code, v = h.call("GET", costPath, get(other, "token"), "", nil)
	must(t, code, 404, v)
	code, v = h.call("POST", costPath, get(other, "token"), core.ID(), map[string]any{"total_cost": "5.00"})
	must(t, code, 404, v)
	costKey := core.ID()
	code, v = h.call("POST", costPath, token, "", map[string]any{"total_cost": "5.00"})
	must(t, code, 400, v)
	code, v = h.call("POST", costPath, token, costKey, map[string]any{"total_cost": "5.00"})
	must(t, code, 200, v)
	code, v = h.call("GET", costPath, token, "", nil)
	must(t, code, 200, v)
	if v["recorded"] != true || v["total_cost"] != "5.00" {
		t.Fatalf("cost retrieval: %+v", v)
	}
	code, v = h.call("POST", costPath, token, costKey, map[string]any{"total_cost": "5.00"})
	must(t, code, 200, v)
	code, v = h.call("POST", costPath, token, costKey, map[string]any{"total_cost": "6.00"})
	must(t, code, 409, v)
	code, v = h.call("POST", root+"/batches/"+core.ID()+"/cost", token, costKey, map[string]any{"total_cost": "5.00"})
	must(t, code, 409, v)
	code, v = h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": ingredient, "batch_id": batch, "quantity": "20", "reason": "consume"})
	must(t, code, 200, v)
	var cost string
	if err := pool.QueryRow(ctx, "SELECT allocated_cost::text FROM stock_cost_snapshots WHERE household_id=$1", house).Scan(&cost); err != nil || cost != "1.00000000" {
		t.Fatalf("incorrect cost %s %v", cost, err)
	}
	if _, err := pool.Exec(ctx, "UPDATE batch_purchase_costs SET total_cost=0 WHERE batch_id=$1", batch); err == nil {
		t.Fatal("purchase fact was mutable")
	}
}
