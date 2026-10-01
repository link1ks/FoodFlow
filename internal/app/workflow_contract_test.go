package app

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"foodflow/internal/core"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
)

// Validation runs only in this isolated test. Authentication and household
// authorization remain the real API middleware; no validator is added at runtime.
func TestKitchenWorkflowContract(t *testing.T) {
	doc := loadContract(t)
	doc.Servers = nil
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	pool := testDB(t)
	a := New(pool)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	token := ""
	call := func(method, path string, body any, expected int) any {
		t.Helper()
		var raw []byte
		if body != nil {
			raw, err = json.Marshal(body)
			if err != nil {
				t.Fatal(err)
			}
		}
		req, e := http.NewRequest(method, server.URL+"/api"+path, bytes.NewReader(raw))
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Content-Type", "application/json")
		if token != "" {
			req.Header.Set("Authorization", "Bearer "+token)
		}
		req.Header.Set("Idempotency-Key", core.ID())
		validationRequest := req.Clone(context.Background())
		validationRequest.URL.Path = path
		route, params, e := router.FindRoute(validationRequest)
		if e != nil {
			t.Fatalf("%s %s: %v", method, path, e)
		}
		input := &openapi3filter.RequestValidationInput{Request: validationRequest, PathParams: params, Route: route, Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc, IncludeResponseStatus: true}}
		if e = openapi3filter.ValidateRequest(context.Background(), input); e != nil {
			t.Fatalf("request %s %s: %v", method, path, e)
		}
		req.Body = io.NopCloser(bytes.NewReader(raw))
		res, e := server.Client().Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer res.Body.Close()
		payload, e := io.ReadAll(res.Body)
		if e != nil {
			t.Fatal(e)
		}
		if res.StatusCode != expected {
			t.Fatalf("%s %s: expected %d, got %d", method, path, expected, res.StatusCode)
		}
		responseInput := &openapi3filter.ResponseValidationInput{RequestValidationInput: input, Status: res.StatusCode, Header: res.Header, Body: io.NopCloser(bytes.NewReader(payload))}
		if e = openapi3filter.ValidateResponse(context.Background(), responseInput); e != nil {
			t.Fatalf("response %s %s: %v", method, path, e)
		}
		if len(payload) == 0 {
			return nil
		}
		var value any
		if e = json.Unmarshal(payload, &value); e != nil {
			t.Fatal(e)
		}
		return value
	}
	u := call("POST", "/register", map[string]any{"email": core.ID() + "@example.test", "password": "Contract-Fixture-2026!", "name": "Contract"}, 200).(map[string]any)
	token = u["token"].(string)
	h := call("POST", "/households", map[string]any{"name": "Contract kitchen", "servings": 2}, 201).(map[string]any)
	root := "/households/" + h["id"].(string)
	call("GET", "/ingredient-catalog", nil, 200)
	call("GET", "/recipes", nil, 200)
	var catalogID string
	if err = pool.QueryRow(context.Background(), "SELECT id FROM ingredient_catalog WHERE name='生菜'").Scan(&catalogID); err != nil {
		t.Fatal(err)
	}
	call("POST", root+"/catalog-stock", map[string]any{"catalog_id": catalogID, "quantity": "100"}, 200)
	call("GET", root+"/inventory", nil, 200)
	p := call("POST", root+"/plans", map[string]any{"meals": []any{map[string]any{"day": time.Now().Format("2006-01-02"), "meal": "dinner", "servings": 2, "recipe_id": "10000000-0000-4000-8000-000000000004"}}}, 201).(map[string]any)
	planPath := root + "/plans/" + p["id"].(string)
	draft := call("GET", planPath, nil, 200).(map[string]any)
	call("POST", planPath+"/confirm", map[string]any{"revision": draft["revision"], "accept_uncertain": false}, 200)
	shopping := call("GET", root+"/shopping", nil, 200).([]any)
	for _, value := range shopping {
		item := value.(map[string]any)
		path := root + "/shopping/" + item["id"].(string)
		call("PATCH", path, map[string]any{"checked": true, "bought": item["needed"]}, 204)
		call("POST", path+"/stock", map[string]any{"location": "冷藏", "expiry_kind": "unknown"}, 200)
	}
	meals := draft["meals"].([]any)
	complete := planPath + "/meals/" + meals[0].(map[string]any)["id"].(string) + "/complete"
	call("POST", complete, map[string]any{}, 200)
	call("POST", complete, map[string]any{}, 409)
	inventory := call("GET", root+"/inventory", nil, 200).(map[string]any)
	for _, item := range inventory["items"].([]any) {
		if item.(map[string]any)["quantity"] != "0.000" {
			t.Fatal("workflow failed to conserve purchased and consumed stock")
		}
	}
	ledger := call("GET", root+"/ledger", nil, 200).([]any)
	if len(ledger) != 6 {
		t.Fatalf("expected 3 inbound and 3 FEFO batch deductions, got %d", len(ledger))
	}
}
