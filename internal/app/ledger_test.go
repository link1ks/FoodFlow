package app

import (
	"context"
	"foodflow/internal/core"
	"net/http/httptest"
	"testing"
)

func TestLedgerHistoryUsesSnapshotsAndHouseholdAuthorization(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, user := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "password123", "name": "History"})
	must(t, code, 200, user)
	token := get(user, "token")
	code, house := h.call("POST", "/households", token, "", map[string]any{"name": "History", "servings": 2})
	must(t, code, 201, house)
	root := "/households/" + get(house, "id")
	code, ingredient := h.call("POST", root+"/ingredients", token, "", map[string]any{"name": "历史番茄", "unit": "g", "category": "蔬菜"})
	must(t, code, 201, ingredient)
	code, stocked := h.call("POST", root+"/stock", token, core.ID(), map[string]any{"ingredient_id": get(ingredient, "id"), "quantity": "123.456", "reason": "manual"})
	must(t, code, 200, stocked)
	if _, err := pool.Exec(context.Background(), "UPDATE ingredients SET name='今日名称' WHERE id=$1", get(ingredient, "id")); err != nil {
		t.Fatal(err)
	}
	code, rows := h.list(root+"/ledger", token)
	if code != 200 || len(rows) != 1 || rows[0]["ingredient"] != "历史番茄" || rows[0]["quantity"] != "123.456" || rows[0]["unit"] != "g" {
		t.Fatalf("historical ledger mismatch: %d %+v", code, rows)
	}
	code, _ = h.call("GET", root+"/ledger", "", "", nil)
	if code != 401 {
		t.Fatalf("unauthenticated history returned %d", code)
	}
	code, outsider := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "password123", "name": "Other"})
	must(t, code, 200, outsider)
	code, _ = h.call("GET", root+"/ledger", get(outsider, "token"), "", nil)
	if code != 404 {
		t.Fatalf("cross-household history returned %d", code)
	}
}
