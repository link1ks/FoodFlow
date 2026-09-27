package app

import (
	"context"
	"foodflow/internal/core"
	"net/http/httptest"
	"testing"
)

func TestShoppingArchive(t *testing.T) {
	db := testDB(t)
	server := httptest.NewServer(New(db).Router())
	defer server.Close()
	h := testAPI{t, server}
	home := func() (string, string) {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "test"})
		must(t, code, 200, v)
		token := get(v, "token")
		code, v = h.call("POST", "/households", token, "", map[string]any{"name": "test", "servings": 2})
		must(t, code, 201, v)
		return token, get(v, "id")
	}
	token, house := home()
	other, _ := home()
	list, item := core.ID(), core.ID()
	ctx := context.Background()
	if _, e := db.Exec(ctx, "INSERT INTO shopping_lists(id,household_id) VALUES($1,$2)", list, house); e != nil {
		t.Fatal(e)
	}
	if _, e := db.Exec(ctx, "INSERT INTO shopping_items(id,list_id,name,unit,needed_milli) VALUES($1,$2,'番茄','g',1000)", item, list); e != nil {
		t.Fatal(e)
	}
	path := "/households/" + house + "/shopping/" + item
	code, v := h.call("DELETE", path, other, "", nil)
	must(t, code, 404, v)
	for range 2 {
		code, v = h.call("DELETE", path, token, "", nil)
		must(t, code, 204, v)
	}
	code, v = h.call("PATCH", path, token, "", map[string]any{"checked": true, "bought": "1"})
	must(t, code, 409, v)
	code, v = h.call("POST", path+"/stock", token, core.ID(), map[string]any{})
	must(t, code, 409, v)
	var archived bool
	if e := db.QueryRow(ctx, "SELECT deleted_at IS NOT NULL FROM shopping_items WHERE id=$1", item).Scan(&archived); e != nil || !archived {
		t.Fatal("archive missing", e)
	}
	var batches int
	if e := db.QueryRow(ctx, "SELECT count(*) FROM batches WHERE household_id=$1", house).Scan(&batches); e != nil || batches != 0 {
		t.Fatal("archive created stock", e)
	}
}
