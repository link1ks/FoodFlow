package app

import (
	"context"
	"foodflow/internal/core"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestPersonalKitchenProvisionIsolationAndLegacyFamily(t *testing.T) {
	db := testDB(t)
	a := New(db)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	register := func(name string) (string, string, string) {
		email := core.ID() + "@example.test"
		status, v := h.call("POST", "/register", "", "", map[string]any{"email": email, "password": "Personal-Fixture-2026!", "name": name})
		must(t, status, 200, v)
		return get(v, "token"), get(v, "user_id"), email
	}
	user, userID, _ := register("个人验收")
	other, otherID, email := register("共享成员")
	status, v := h.call("PUT", "/me/personal-kitchen", "", "", nil)
	must(t, status, 401, v)
	status, v = h.call("POST", "/households", user, "", map[string]any{"name": "已有家庭", "servings": 3})
	must(t, status, 201, v)
	family := get(v, "id")
	status, v = h.call("POST", "/households/"+family+"/ingredients", user, "", map[string]any{"name": "番茄", "unit": "g"})
	must(t, status, 201, v)
	status, v = h.call("POST", "/households/"+family+"/stock", user, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": "300.125", "reason": "manual"})
	must(t, status, 200, v)
	// Concurrent sessions all provision the same default and commit its membership.
	ids := make(chan string, 6)
	var wg sync.WaitGroup
	for range 6 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, result := h.call("PUT", "/me/personal-kitchen", user, "", nil)
			must(t, code, 200, result)
			if result["kind"] != "personal" || result["servings"] != float64(1) || result["role"] != "owner" {
				t.Error("default private kitchen identity/servings incorrect")
			}
			ids <- get(result, "id")
		}()
	}
	wg.Wait()
	close(ids)
	personal := ""
	for id := range ids {
		if personal != "" && id != personal {
			t.Fatal("concurrent requests created separate kitchens")
		}
		personal = id
	}
	if personal == "" || personal == family {
		t.Fatal("existing shared family became a personal kitchen")
	}
	root := "/households/" + personal
	status, v = h.call("GET", root+"/inventory", user, "", nil)
	must(t, status, 200, v)
	if len(v["items"].([]any)) != 0 {
		t.Fatal("family stock copied into private space")
	}
	status, v = h.call("GET", "/households/"+family, user, "", nil)
	must(t, status, 200, v)
	if v["kind"] != "family" || v["servings"] != float64(3) {
		t.Fatal("legacy family settings changed")
	}
	status, v = h.call("GET", "/households/"+family+"/inventory", user, "", nil)
	must(t, status, 200, v)
	if get(v["items"].([]any)[0].(map[string]any), "quantity") != "300.125" {
		t.Fatal("legacy stock changed")
	}
	status, v = h.call("POST", root+"/invites", user, "", map[string]any{"email": email, "role": "editor"})
	must(t, status, 403, v)
	status, v = h.call("PATCH", root+"/members/"+otherID, user, "", map[string]any{"role": "viewer"})
	must(t, status, 403, v)
	ctx := context.Background()
	// Even a stale/erroneous member row cannot authorize another account.
	if _, err := db.Exec(ctx, "INSERT INTO members(household_id,user_id,role) VALUES($1,$2,'editor')", personal, otherID); err != nil {
		t.Fatal(err)
	}
	status, v = h.call("GET", root+"/inventory", other, "", nil)
	must(t, status, 404, v)
	if _, _, err := a.chooseRecipe(ctx, claimed{Household: personal, Creator: otherID}, planJob{MaxMinutes: 30}); err == nil {
		t.Fatal("worker accepted a foreign member of a private kitchen")
	}
	status, v = h.call("POST", root+"/ingredients", user, "", map[string]any{"name": "私有临期食材", "unit": "g"})
	must(t, status, 201, v)
	status, v = h.call("POST", root+"/stock", user, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": "10", "reason": "manual"})
	must(t, status, 200, v)
	if _, err := db.Exec(ctx, "UPDATE batches SET expires_on=(now() AT TIME ZONE 'Asia/Shanghai')::date WHERE household_id=$1", personal); err != nil {
		t.Fatal(err)
	}
	a.makeReminders(ctx)
	var reminders int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM reminders WHERE household_id=$1 AND user_id=$2", personal, otherID).Scan(&reminders); err != nil || reminders != 0 {
		t.Fatal("private expiry reminder leaked to another member", reminders, err)
	}
	if err := db.QueryRow(ctx, "SELECT count(*) FROM reminders WHERE household_id=$1 AND user_id=$2", personal, userID).Scan(&reminders); err != nil || reminders != 1 {
		t.Fatal("owner expiry reminder missing", reminders, err)
	}
	status, list := h.list("/households", other)
	if status != 200 || len(list) != 0 {
		t.Fatal("private space listed for another user")
	}
	// Historical or manually seeded personal invite is rejected before consumption.
	code := token()
	if _, err := db.Exec(ctx, "INSERT INTO invites(id,household_id,email,role,code_hash,expires_at) VALUES($1,$2,$3,'editor',$4,now()+interval '1 day')", core.ID(), personal, email, core.Hash(code)); err != nil {
		t.Fatal(err)
	}
	status, v = h.call("POST", "/invites/accept", other, "", map[string]any{"code": code})
	must(t, status, 403, v)
	status, v = h.call("PUT", "/me/personal-kitchen", other, "", map[string]any{"user_id": userID, "household_id": personal})
	must(t, status, 200, v)
	if get(v, "id") == personal {
		t.Fatal("client-selected owner bypassed isolation")
	}
	status, list = h.list("/households", user)
	if status != 200 || len(list) != 2 || list[0]["kind"] != "personal" {
		t.Fatal("personal space not first; family lost")
	}
	var count int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM households WHERE owner_id=$1 AND kind='personal'", userID).Scan(&count); err != nil || count != 1 {
		t.Fatal("duplicate private kitchen", count, err)
	}
}
