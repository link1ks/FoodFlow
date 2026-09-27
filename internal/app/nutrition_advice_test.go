package app

import (
	"context"
	"encoding/json"
	"foodflow/internal/core"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestNutritionAdviceFlow(t *testing.T) {
	pool := testDB(t)
	a := New(pool)
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	ctx := context.Background()
	home := func() (string, string, string) {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "nutrition"})
		must(t, code, 200, v)
		token := get(v, "token")
		code, v = h.call("POST", "/households", token, "", map[string]any{"name": "nutrition", "servings": 2})
		must(t, code, 201, v)
		id := get(v, "id")
		return token, "/households/" + id, id
	}
	token, root, house := home()
	other, _, _ := home()
	code, catalog := h.list("/ingredient-catalog", token)
	if code != 200 || len(catalog) != 91 {
		t.Fatal(code, len(catalog))
	}
	var tomato string
	for _, r := range catalog {
		if r["nutrition"] == nil {
			t.Fatal("missing profile")
		}
		if r["name"] == "番茄" {
			tomato = r["id"].(string)
		}
	}
	code, v := h.call("POST", root+"/catalog-stock", token, core.ID(), map[string]any{"catalog_id": tomato, "quantity": "500"})
	must(t, code, 200, v)
	code, v = h.call("GET", root+"/inventory", token, "", nil)
	must(t, code, 200, v)
	item := v["items"].([]any)[0].(map[string]any)
	id := item["id"].(string)
	if item["nutrition"].(map[string]any)["status"] != "reference" {
		t.Fatal(item)
	}
	body := map[string]any{"selected_ingredient_ids": []string{id}, "servings": 2, "max_minutes": 60, "goal": "搭配清淡一点"}
	code, v = h.call("POST", root+"/jobs/advice", other, core.ID(), body)
	must(t, code, 404, v)
	bad := map[string]any{"selected_ingredient_ids": []string{core.ID()}, "servings": 2, "max_minutes": 60}
	code, v = h.call("POST", root+"/jobs/advice", token, core.ID(), bad)
	must(t, code, 400, v)
	calls := 0
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		var payload struct {
			Messages []struct{ Role, Content string }
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatal(err)
		}
		var input struct {
			Stock   []struct{ Name string } `json:"selected_inventory"`
			Recipes []struct{ ID string }   `json:"allowed_recipes"`
		}
		if err := json.Unmarshal([]byte(payload.Messages[1].Content), &input); err != nil {
			t.Fatal(err)
		}
		if len(input.Stock) != 1 || input.Stock[0].Name != "番茄" {
			t.Errorf("selection not isolated: %+v", input)
		}
		ids := []string{}
		if len(input.Recipes) > 0 {
			ids = append(ids, input.Recipes[0].ID)
		}
		answer := core.JSON(map[string]any{"summary": "根据所选番茄搭配蛋白质食材。", "tips": []string{"核对菜谱中额外需要的食材。"}, "recipe_ids": ids})
		_ = json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"role": "assistant", "content": string(answer)}}}})
	}))
	defer model.Close()
	t.Setenv("MODEL_ENDPOINT", model.URL)
	t.Setenv("MODEL_API_KEY", "test")
	t.Setenv("MODEL_NAME", "test")
	key := core.ID()
	code, v = h.call("POST", root+"/jobs/advice", token, key, body)
	must(t, code, 202, v)
	job := get(v, "id")
	code, v = h.call("POST", root+"/jobs/advice", token, key, body)
	must(t, code, 202, v)
	if get(v, "id") != job {
		t.Fatal("duplicate task")
	}
	body["goal"] = "changed"
	code, v = h.call("POST", root+"/jobs/advice", token, key, body)
	must(t, code, 409, v)
	j, err := a.claim(ctx, "nutrition-test")
	if err != nil {
		t.Fatal(err)
	}
	a.runJob(ctx, j, "nutrition-test")
	code, v = h.call("GET", root+"/jobs/"+job, token, "", nil)
	must(t, code, 200, v)
	if v["status"] != "succeeded" || calls != 1 {
		t.Fatal(v, calls)
	}
	result := v["result"].(map[string]any)
	if result["mode"] != "model" {
		t.Fatal(result)
	}
	var quantity int64
	_ = pool.QueryRow(ctx, "SELECT sum(quantity_milli) FROM batches WHERE household_id=$1", house).Scan(&quantity)
	if quantity != 500000 {
		t.Fatal("advice changed inventory")
	}
	var plans int
	_ = pool.QueryRow(ctx, "SELECT count(*) FROM plans WHERE household_id=$1", house).Scan(&plans)
	if plans != 0 {
		t.Fatal("advice created plan without confirmation")
	}
	code, v = h.call("POST", root+"/jobs/advice", token, core.ID(), body)
	must(t, code, 202, v)
	cancelID := get(v, "id")
	j, err = a.claim(ctx, "nutrition-test")
	if err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/jobs/"+cancelID+"/cancel", token, "", map[string]any{})
	must(t, code, 204, v)
	a.runJob(ctx, j, "nutrition-test")
	if calls != 1 {
		t.Fatal("cancelled job called model")
	}
	code, v = h.call("GET", root+"/jobs/"+cancelID, token, "", nil)
	must(t, code, 200, v)
	if v["status"] != "cancelled" || v["result"] != nil {
		t.Fatal(v)
	}
	_, err = pool.Exec(ctx, "UPDATE batches SET condition='spoiled' WHERE household_id=$1", house)
	if err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/jobs/advice", token, core.ID(), body)
	must(t, code, 400, v)
}
