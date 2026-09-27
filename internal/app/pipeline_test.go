package app

import (
	"context"
	"fmt"
	"foodflow/internal/core"
	"net/http/httptest"
	"testing"
	"time"
)

func TestKitchenPipelineLifecycle(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Cook"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Kitchen", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	plan, meal := core.ID(), core.ID()
	_, err := pool.Exec(context.Background(), `INSERT INTO plans(id,household_id,status,created_by) SELECT $1,$2,'confirmed',user_id FROM members WHERE household_id=$2 LIMIT 1`, plan, house)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO plan_meals(id,plan_id,day,meal,servings,recipe_id) VALUES($1,$2,current_date,'dinner',2,'10000000-0000-4000-8000-000000000001');`, meal, plan)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(context.Background(), `INSERT INTO plan_meal_dishes(id,plan_meal_id,recipe_id) VALUES(gen_random_uuid(),$1,'10000000-0000-4000-8000-000000000002')`, meal)
	if err != nil {
		t.Fatal(err)
	}
	path := root + "/meals/" + meal + "/pipeline"
	key := core.ID()
	body := map[string]any{"target_at": time.Now().Add(time.Hour), "second_stove": true}
	code, v = h.call("POST", path, token, "", body)
	must(t, code, 400, v)
	code, v = h.call("POST", path, token, key, body)
	must(t, code, 200, v)
	session := get(v, "id")
	code, v = h.call("POST", path, token, key, body)
	must(t, code, 200, v)
	if get(v, "id") != session {
		t.Fatal("duplicate session")
	}
	code, v = h.call("POST", path, token, core.ID(), body)
	must(t, code, 409, v)
	code, other := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Other"})
	must(t, code, 200, other)
	code, v = h.call("GET", path, get(other, "token"), "", nil)
	must(t, code, 404, v)
	action := func(i int, a string, want int) {
		t.Helper()
		code, v := h.call("POST", fmt.Sprintf("%s/pipelines/%s/steps/%d", root, session, i), token, core.ID(), map[string]any{"action": a})
		must(t, code, want, v)
	}
	action(1, "start", 409) // dependency not completed
	action(0, "complete", 409)
	action(0, "start", 200)
	action(3, "start", 409) // independent recipe, same hands and board
	action(0, "complete", 200)
	for i := 1; i < 6; i++ {
		action(i, "start", 200)
		action(i, "complete", 200)
	}
	code, v = h.call("GET", path, token, "", nil)
	must(t, code, 200, v)
	if v["session"].(map[string]any)["status"] != "finished" {
		t.Fatal("not finished")
	}
	action(0, "start", 409)
	var n int
	if err = pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_ledger WHERE household_id=$1", house).Scan(&n); err != nil || n != 0 {
		t.Fatal("step wrote inventory", err, n)
	}
	// A second meal verifies terminal cancellation and revision fencing.
	second := core.ID()
	_, err = pool.Exec(context.Background(), `INSERT INTO plan_meals(id,plan_id,day,meal,servings,recipe_id) VALUES($1,$2,current_date,'lunch',2,'10000000-0000-4000-8000-000000000001')`, second, plan)
	if err != nil {
		t.Fatal(err)
	}
	path = root + "/meals/" + second + "/pipeline"
	code, v = h.call("POST", path, token, core.ID(), body)
	must(t, code, 200, v)
	session = get(v, "id")
	_, err = pool.Exec(context.Background(), "UPDATE plans SET revision=revision+1 WHERE id=$1", plan)
	if err != nil {
		t.Fatal(err)
	}
	action(0, "start", 409)
	action(0, "cancel", 200)
	action(0, "start", 409)
	code, v = h.call("GET", path, token, "", nil)
	must(t, code, 200, v)
	if v["session"] != nil {
		t.Fatal("cancelled session still current")
	}
}
