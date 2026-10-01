package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"bytes"
	"encoding/json"
	"foodflow/internal/core"
	"github.com/getkin/kin-openapi/openapi3filter"
	"github.com/getkin/kin-openapi/routers/legacy"
	"io"
	"net/http"
)

func TestMonthlyReportCostsCoverageAndScope(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	app := New(pool)
	defer app.Close()
	server := httptest.NewServer(app.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Report-Fixture-2026!", "name": "Report"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Costs", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	code, v = h.call("GET", root+"/monthly-report", token, "", nil)
	must(t, code, 200, v)
	if v["summary"].(map[string]any)["outbound_events"] != float64(0) {
		t.Fatal("empty report invented events")
	}
	var catalog string
	if err := pool.QueryRow(ctx, "SELECT id FROM ingredient_catalog WHERE name='番茄'").Scan(&catalog); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/catalog-stock", token, core.ID(), map[string]any{"catalog_id": catalog, "quantity": "100"})
	must(t, code, 200, v)
	batch := get(v, "batch_id")
	ingredient := get(v, "ingredient_id")
	stock := func(quantity, reason, key string) {
		t.Helper()
		code, v := h.call("POST", root+"/stock", token, key, map[string]any{"ingredient_id": ingredient, "batch_id": batch, "quantity": quantity, "reason": reason})
		must(t, code, 200, v)
	}
	stock("10", "consume", core.ID()) // No historical cost exists.
	code, v = h.call("POST", root+"/batches/"+batch+"/cost", token, core.ID(), map[string]any{"total_cost": "5.00"})
	must(t, code, 200, v)
	stock("20", "consume", core.ID())
	key := core.ID()
	stock("30", "waste", key)
	stock("30", "waste", key)
	code, v = h.call("GET", root+"/monthly-report", token, "", nil)
	must(t, code, 200, v)
	summary := v["summary"].(map[string]any)
	for name, want := range map[string]any{"outbound_events": float64(3), "priced_outbound_events": float64(2), "unknown_outbound_events": float64(1), "known_consumed_cost": "1.00", "known_wasted_cost": "1.50", "recorded_purchase_cost": "5.00", "purchase_records": float64(1)} {
		if summary[name] != want {
			t.Fatalf("%s=%v want=%v", name, summary[name], want)
		}
	}
	if v["historical_outbound_repriced"] != false {
		t.Fatal("report repriced history")
	}
	item := v["items"].([]any)[0].(map[string]any)
	if item["inbound_milli"] != "100000" || item["consumed_milli"] != "30000" || item["wasted_milli"] != "30000" || item["unit"] != "g" {
		t.Fatalf("quantity or unit changed: %+v", item)
	}
	// Validate the actual response, including numeric strings, against OpenAPI.
	doc := loadContract(t)
	doc.Servers = nil
	router, err := legacy.NewRouter(doc)
	if err != nil {
		t.Fatal(err)
	}
	req, _ := http.NewRequest("GET", "http://fixture/households/"+house+"/monthly-report", nil)
	route, params, err := router.FindRoute(req)
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(v)
	input := &openapi3filter.RequestValidationInput{Request: req, PathParams: params, Route: route, Options: &openapi3filter.Options{AuthenticationFunc: openapi3filter.NoopAuthenticationFunc}}
	if err = openapi3filter.ValidateResponse(ctx, &openapi3filter.ResponseValidationInput{RequestValidationInput: input, Status: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(bytes.NewReader(raw))}); err != nil {
		t.Fatal(err)
	}
	// Current catalogue/ingredient names do not replace historical snapshots.
	if _, err = pool.Exec(ctx, "UPDATE ingredients SET name='今日改名' WHERE id=$1", ingredient); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", root+"/monthly-report", token, "", nil)
	must(t, code, 200, v)
	if v["items"].([]any)[0].(map[string]any)["ingredient"] != "番茄" {
		t.Fatal("historical identity changed")
	}
	code, v = h.call("GET", root+"/monthly-report?month=2026-13", token, "", nil)
	must(t, code, 400, v)
	code, v = h.call("GET", root+"/monthly-report?month=0000-01", token, "", nil)
	must(t, code, 400, v)
	code, v = h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Report-Fixture-2026!", "name": "Other"})
	must(t, code, 200, v)
	other := get(v, "token")
	code, v = h.call("GET", root+"/monthly-report", other, "", nil)
	must(t, code, 404, v)
	// The family timezone owns both inclusive start and exclusive end.
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback(ctx)
	for _, at := range []string{"2026-02-28T15:59:59Z", "2026-02-28T16:00:00Z", "2026-03-31T15:59:59Z", "2026-03-31T16:00:00Z"} {
		if _, err = tx.Exec(ctx, "UPDATE batches SET quantity_milli=quantity_milli+1000 WHERE id=$1", batch); err != nil {
			t.Fatal(err)
		}
		if _, err = tx.Exec(ctx, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,created_at) VALUES($1,$2,$3,1000,'correction',$4::timestamptz)", core.ID(), house, batch, at); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", root+"/monthly-report?month=2026-03", token, "", nil)
	must(t, code, 200, v)
	item = v["items"].([]any)[0].(map[string]any)
	if item["adjusted_milli"] != "2000" || v["summary"].(map[string]any)["known_consumed_cost"] != "0.00" {
		t.Fatalf("timezone or correction misclassified: %+v", v)
	}
}
