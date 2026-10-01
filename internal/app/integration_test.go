package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"foodflow/internal/core"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"github.com/testcontainers/testcontainers-go/modules/postgres"
	"image"
	"image/color"
	"image/png"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestImageRecognitionConfirmation(t *testing.T) {
	pool := testDB(t)
	model := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"{\"name\":\"番茄\",\"category\":\"蔬菜\"}"}}]}`))
	}))
	defer model.Close()
	t.Setenv("MODEL_ENDPOINT", model.URL)
	t.Setenv("MODEL_API_KEY", "test-key")
	t.Setenv("VISION_MODEL_NAME", "test-vision")
	imageDir := t.TempDir()
	t.Setenv("IMAGE_STORAGE_DIR", imageDir)
	t.Setenv("STORAGE_BACKEND", "local")
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Alice"})
	must(t, code, 200, v)
	user := get(v, "token")
	code, v = h.call("POST", "/households", user, "", map[string]any{"name": "Image household", "servings": 2})
	must(t, code, 201, v)
	root := "/households/" + get(v, "id")
	otherCode, other := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Other"})
	must(t, otherCode, 200, other)
	outsider := get(other, "token")
	upload := func(token string) (int, map[string]any) {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, e := writer.CreateFormFile("image", "tomato.png")
		if e != nil {
			t.Fatal(e)
		}
		_, _ = part.Write([]byte{137, 'P', 'N', 'G', 13, 10, 26, 10, 0})
		_ = writer.Close()
		req, e := http.NewRequest("POST", server.URL+"/api"+root+"/jobs/image", &body)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		out := map[string]any{}
		_ = json.NewDecoder(response.Body).Decode(&out)
		return response.StatusCode, out
	}
	code, v = upload(outsider)
	must(t, code, 404, v)
	code, v = upload(user)
	must(t, code, 202, v)
	jobID := get(v, "id")
	j, e := New(pool).claim(context.Background(), "test-worker")
	if e != nil || j.ID != jobID || j.Kind != "image" {
		t.Fatalf("claim %+v: %v", j, e)
	}
	New(pool).runJob(context.Background(), j, "test-worker")
	code, v = h.call("GET", root+"/jobs/"+jobID, user, "", nil)
	must(t, code, 200, v)
	if get(v, "status") != "awaiting_confirmation" || v["result"].(map[string]any)["name"] != "番茄" {
		t.Fatalf("recognition: %+v", v)
	}
	preview := func(token string) int {
		req, e := http.NewRequest("GET", server.URL+"/api"+root+"/jobs/"+jobID+"/image", nil)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		resp, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer resp.Body.Close()
		return resp.StatusCode
	}
	if preview(outsider) != 404 || preview(user) != 200 {
		t.Fatal("image preview permission or availability incorrect")
	}
	key := core.ID()
	confirmation := map[string]any{"name": "番茄", "category": "蔬菜", "unit": "个", "quantity": "2", "expires_on": ""}
	code, v = h.call("POST", root+"/jobs/"+jobID+"/image/confirm", outsider, key, confirmation)
	must(t, code, 404, v)
	code, v = h.call("POST", root+"/jobs/"+jobID+"/image/confirm", user, key, confirmation)
	must(t, code, 200, v)
	batch := get(v, "batch_id")
	code, v = h.call("POST", root+"/jobs/"+jobID+"/image/confirm", user, key, confirmation)
	must(t, code, 200, v)
	if get(v, "batch_id") != batch {
		t.Fatal("same key created another batch")
	}
	if preview(user) != 404 {
		t.Fatal("confirmed image remains visible")
	}
	if _, e := os.Stat(filepath.Join(imageDir, strings.TrimPrefix(root, "/households/"), jobID+".png")); e == nil {
		t.Fatal("confirmed image retained")
	}
	code, v = h.call("POST", root+"/jobs/"+jobID+"/image/confirm", user, core.ID(), confirmation)
	must(t, code, 409, v)
	code, v = h.call("GET", root+"/inventory", user, "", nil)
	must(t, code, 200, v)
	items := v["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["quantity"] != "2.000" {
		t.Fatalf("inventory: %+v", v)
	}
	code, v = upload(user)
	must(t, code, 202, v)
	rejected := get(v, "id")
	j, e = New(pool).claim(context.Background(), "test-worker")
	if e != nil || j.ID != rejected {
		t.Fatalf("claim reject: %v", e)
	}
	New(pool).runJob(context.Background(), j, "test-worker")
	code, v = h.call("POST", root+"/jobs/"+rejected+"/cancel", user, "", map[string]any{})
	must(t, code, 204, v)
	code, v = h.call("POST", root+"/jobs/"+rejected+"/image/confirm", user, core.ID(), confirmation)
	must(t, code, 409, v)
}

func TestIngredientCatalogSelection(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Catalog test"})
	must(t, code, 200, v)
	token := get(v, "token")
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "Catalog home", "servings": 2})
	must(t, code, 201, v)
	root := "/households/" + get(v, "id")
	code, _ = h.call("GET", "/ingredient-catalog", "", "", nil)
	if code != 401 {
		t.Fatalf("unauthenticated catalog status: %d", code)
	}
	code, catalog := h.list("/ingredient-catalog", token)
	if code != 200 {
		t.Fatalf("catalog status: %d", code)
	}
	var tomatoID string
	for _, item := range catalog {
		if item["name"] == "番茄" {
			tomatoID = item["id"].(string)
			if item["category"] != "蔬菜" || item["default_unit"] != "g" || item["aliases"].([]any)[0] != "西红柿" {
				t.Fatalf("catalog tomato: %+v", item)
			}
		}
	}
	if tomatoID == "" {
		t.Fatal("tomato missing from catalog")
	}
	code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"catalog_id": core.ID(), "low": "100"})
	must(t, code, 400, v)
	code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"catalog_id": tomatoID, "name": "伪造名称", "category": "错误分类", "unit": "个", "low": "100"})
	must(t, code, 201, v)
	id := get(v, "id")
	code, v = h.call("POST", root+"/ingredients", token, "", map[string]any{"catalog_id": tomatoID, "low": "200"})
	must(t, code, 201, v)
	if get(v, "id") != id {
		t.Fatal("catalog selection created duplicate ingredient")
	}
	code, v = h.call("GET", root+"/inventory", token, "", nil)
	must(t, code, 200, v)
	items := v["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["name"] != "番茄" || items[0].(map[string]any)["category"] != "蔬菜" || items[0].(map[string]any)["unit"] != "g" || items[0].(map[string]any)["low"] != "200.000" {
		t.Fatalf("catalog selection not canonical: %+v", items)
	}
	key := core.ID()
	stock := map[string]any{"catalog_id": tomatoID, "quantity": "300", "location": "冷藏", "expires_on": "2026-10-01"}
	code, v = h.call("POST", root+"/catalog-stock", token, key, stock)
	must(t, code, 200, v)
	batchID := get(v, "batch_id")
	code, v = h.call("POST", root+"/catalog-stock", token, key, stock)
	must(t, code, 200, v)
	if get(v, "batch_id") != batchID {
		t.Fatal("catalog stock retry created a second batch")
	}
	code, v = h.call("POST", root+"/catalog-stock", token, key, map[string]any{"catalog_id": tomatoID, "quantity": "301"})
	must(t, code, 409, v)
	code, v = h.call("GET", root+"/inventory", token, "", nil)
	must(t, code, 200, v)
	items = v["items"].([]any)
	if items[0].(map[string]any)["quantity"] != "300.000" || len(v["batches"].([]any)) != 1 {
		t.Fatalf("catalog stock was duplicated: %+v", v)
	}
}

func TestIngredientArchiveKeepsLedgerAndCanBeRestocked(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	register := func() string {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Archive test"})
		must(t, code, 200, v)
		return get(v, "token")
	}
	owner, outsider := register(), register()
	code, v := h.call("POST", "/households", owner, "", map[string]any{"name": "Archive home", "servings": 2})
	must(t, code, 201, v)
	root := "/households/" + get(v, "id")
	code, catalog := h.list("/ingredient-catalog", owner)
	if code != 200 {
		t.Fatalf("catalog status %d", code)
	}
	var tomato string
	for _, item := range catalog {
		if item["name"] == "番茄" {
			tomato = item["id"].(string)
		}
	}
	if tomato == "" {
		t.Fatal("tomato missing")
	}
	var ingredient string
	for _, quantity := range []string{"200", "100"} {
		code, v = h.call("POST", root+"/catalog-stock", owner, core.ID(), map[string]any{"catalog_id": tomato, "quantity": quantity})
		must(t, code, 200, v)
		ingredient = get(v, "ingredient_id")
	}
	path := root + "/ingredients/" + ingredient
	code, v = h.call("DELETE", path, outsider, core.ID(), nil)
	must(t, code, 404, v)
	key := core.ID()
	code, v = h.call("DELETE", path, owner, key, nil)
	must(t, code, 200, v)
	if v["quantity_removed"] != "300.000" {
		t.Fatalf("removed quantity: %+v", v)
	}
	code, v = h.call("DELETE", path, owner, key, nil)
	must(t, code, 200, v)
	if v["quantity_removed"] != "300.000" {
		t.Fatalf("idempotent archive: %+v", v)
	}
	code, v = h.call("GET", root+"/inventory", owner, "", nil)
	must(t, code, 200, v)
	if len(v["items"].([]any)) != 0 {
		t.Fatalf("archived ingredient visible: %+v", v)
	}
	code, v = h.call("POST", root+"/stock", owner, core.ID(), map[string]any{"ingredient_id": ingredient, "quantity": "1", "reason": "manual"})
	must(t, code, 409, v)
	var entries int
	var removed int64
	if e := pool.QueryRow(context.Background(), "SELECT count(*),COALESCE(sum(-delta_milli),0) FROM stock_ledger WHERE ref_type='ingredient_archive' AND ref_id=$1", ingredient).Scan(&entries, &removed); e != nil {
		t.Fatal(e)
	}
	if entries != 2 || removed != 300000 {
		t.Fatalf("archive ledger entries=%d removed=%d", entries, removed)
	}
	code, v = h.call("POST", root+"/catalog-stock", owner, core.ID(), map[string]any{"catalog_id": tomato, "quantity": "50"})
	must(t, code, 200, v)
	if get(v, "ingredient_id") != ingredient {
		t.Fatal("restock did not reactivate ingredient")
	}
	code, v = h.call("GET", root+"/inventory", owner, "", nil)
	must(t, code, 200, v)
	items := v["items"].([]any)
	if len(items) != 1 || items[0].(map[string]any)["quantity"] != "50.000" {
		t.Fatalf("restocked inventory: %+v", v)
	}
}

func TestIngredientPhotosAndRecipeOptions(t *testing.T) {
	pool := testDB(t)
	t.Setenv("STORAGE_BACKEND", "local")
	t.Setenv("IMAGE_STORAGE_DIR", t.TempDir())
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	register := func() string {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "Photo test"})
		must(t, code, 200, v)
		return get(v, "token")
	}
	owner, outsider := register(), register()
	code, v := h.call("POST", "/households", owner, "", map[string]any{"name": "Photo home", "servings": 2})
	must(t, code, 201, v)
	root := "/households/" + get(v, "id")
	code, v = h.call("POST", "/households", outsider, "", map[string]any{"name": "Other home", "servings": 2})
	must(t, code, 201, v)
	otherRoot := "/households/" + get(v, "id")
	code, v = h.call("POST", root+"/ingredients", owner, "", map[string]any{"name": "番茄", "category": "蔬菜", "unit": "g"})
	must(t, code, 201, v)
	tomato := get(v, "id")
	code, v = h.call("POST", root+"/ingredients", owner, "", map[string]any{"name": "鸡蛋", "category": "蛋类", "unit": "个"})
	must(t, code, 201, v)
	eggs := get(v, "id")
	photo := image.NewRGBA(image.Rect(0, 0, 1, 1))
	photo.Set(0, 0, color.RGBA{255, 0, 0, 255})
	var pngData bytes.Buffer
	if e := png.Encode(&pngData, photo); e != nil {
		t.Fatal(e)
	}
	upload := func(token, path string) int {
		var body bytes.Buffer
		writer := multipart.NewWriter(&body)
		part, e := writer.CreateFormFile("image", "tomato.png")
		if e != nil {
			t.Fatal(e)
		}
		_, _ = part.Write(pngData.Bytes())
		_ = writer.Close()
		req, e := http.NewRequest("POST", server.URL+"/api"+path, &body)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", writer.FormDataContentType())
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		return response.StatusCode
	}
	photoPath := root + "/ingredients/" + tomato + "/image"
	if upload(outsider, photoPath) != 404 || upload(owner, photoPath) != 200 {
		t.Fatal("photo upload permission or result incorrect")
	}
	readPhoto := func(token string) int {
		req, e := http.NewRequest("GET", server.URL+"/api"+photoPath, nil)
		if e != nil {
			t.Fatal(e)
		}
		req.Header.Set("Authorization", "Bearer "+token)
		response, e := http.DefaultClient.Do(req)
		if e != nil {
			t.Fatal(e)
		}
		defer response.Body.Close()
		if response.StatusCode == 200 && response.Header.Get("Content-Type") != "image/png" {
			t.Fatal("wrong image type")
		}
		return response.StatusCode
	}
	if readPhoto(outsider) != 404 || readPhoto(owner) != 200 {
		t.Fatal("photo read permission incorrect")
	}
	code, v = h.call("GET", root+"/inventory", owner, "", nil)
	must(t, code, 200, v)
	foundPhoto := false
	for _, raw := range v["items"].([]any) {
		item := raw.(map[string]any)
		if item["id"] == tomato && item["has_image"] == true {
			foundPhoto = true
		}
	}
	if !foundPhoto {
		t.Fatal("inventory lost image state")
	}
	code, v = h.call("DELETE", photoPath, owner, "", nil)
	must(t, code, 204, v)
	if readPhoto(owner) != 404 {
		t.Fatal("deleted image still served")
	}
	for _, item := range []struct{ id, quantity string }{{tomato, "300"}, {eggs, "2"}} {
		code, v = h.call("POST", root+"/stock", owner, core.ID(), map[string]any{"ingredient_id": item.id, "quantity": item.quantity, "reason": "manual"})
		must(t, code, 200, v)
	}
	request := func(ids []string, servings int) (int, []map[string]any) {
		status, all := h.listRequest("POST", root+"/recipe-options", owner, map[string]any{"selected_ingredient_ids": ids, "servings": servings, "max_minutes": 30})
		// These assertions exercise the original tomato/egg BOM. Other valid
		// recipes may now also match; their presence must not alter this fixture.
		original := []map[string]any{}
		for _, option := range all {
			if option["recipe_id"] == "10000000-0000-4000-8000-000000000001" {
				original = append(original, option)
			}
		}
		return status, original
	}
	code, options := request([]string{tomato}, 2)
	if code != 200 || len(options) != 1 || options[0]["status"] != "one_missing" {
		t.Fatalf("partial options: %d %+v", code, options)
	}
	code, options = request([]string{tomato, eggs}, 2)
	if code != 200 || len(options) != 1 || options[0]["status"] != "ready" {
		t.Fatalf("ready options: %d %+v", code, options)
	}
	code, options = request([]string{tomato, eggs}, 4)
	if code != 200 || options[0]["status"] != "two_missing" {
		t.Fatalf("servings not reflected: %d %+v", code, options)
	}
	code, _ = h.listRequest("POST", otherRoot+"/recipe-options", outsider, map[string]any{"selected_ingredient_ids": []string{tomato}, "servings": 2, "max_minutes": 30})
	if code != 404 {
		t.Fatalf("cross-household selection returned %d", code)
	}
	day := time.Now().Format("2006-01-02")
	code, v = h.call("POST", root+"/plans", owner, "", map[string]any{"meals": []any{map[string]any{"day": day, "meal": "dinner", "servings": 2, "recipe_id": options[0]["recipe_id"]}}})
	must(t, code, 201, v)
	code, v = h.call("POST", root+"/plans/"+get(v, "id")+"/confirm", owner, core.ID(), map[string]any{"revision": 1, "accept_uncertain": false})
	must(t, code, 200, v)
	code, v = h.call("POST", root+"/ingredients", owner, "", map[string]any{"name": "鸡蛋", "category": "蛋类", "unit": "g"})
	must(t, code, 201, v)
	code, v = h.call("POST", root+"/stock", owner, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": "100", "reason": "manual"})
	must(t, code, 200, v)
	code, options = request([]string{tomato, eggs}, 2)
	if code != 200 || options[0]["status"] != "unit_confirmation" {
		t.Fatalf("unit conflict not surfaced: %d %+v", code, options)
	}
}

func testDB(t *testing.T) *pgxpool.Pool {
	t.Setenv("JWT_SECRET", "test-only-secret-with-at-least-32-bytes")
	t.Helper()
	ctx := context.Background()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		if testing.Short() {
			t.Skip("short mode")
		}
		if e := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").Run(); e != nil {
			if os.Getenv("REQUIRE_INTEGRATION") == "true" {
				t.Fatalf("required Docker engine unavailable: %v", e)
			}
			t.Skipf("Docker engine unavailable: %v", e)
		}
		container, e := postgres.Run(ctx, "postgres:16-alpine", postgres.WithDatabase("foodflow"), postgres.WithUsername("foodflow"), postgres.WithPassword("foodflow"), postgres.BasicWaitStrategies())
		if e != nil {
			if os.Getenv("REQUIRE_INTEGRATION") == "true" {
				t.Fatalf("required PostgreSQL container unavailable: %v", e)
			}
			t.Skipf("Docker unavailable: %v", e)
		}
		t.Cleanup(func() { _ = container.Terminate(ctx) })
		url, e = container.ConnectionString(ctx, "sslmode=disable")
		if e != nil {
			t.Fatal(e)
		}
	}
	db, e := sql.Open("pgx", url)
	if e != nil {
		t.Fatal(e)
	}
	defer db.Close()
	if e = goose.SetDialect("postgres"); e != nil {
		t.Fatal(e)
	}
	if e = goose.Up(db, filepath.Join("..", "..", "sql", "schema")); e != nil {
		t.Fatal(e)
	}
	pool, e := pgxpool.New(ctx, url)
	if e != nil {
		t.Fatal(e)
	}
	t.Cleanup(pool.Close)
	return pool
}

type testAPI struct {
	t      *testing.T
	server *httptest.Server
}

func (h testAPI) list(path, token string) (int, []map[string]any) {
	return h.listRequest("GET", path, token, nil)
}

func (h testAPI) listRequest(method, path, token string, body any) (int, []map[string]any) {
	h.t.Helper()
	var payload []byte
	if body != nil {
		payload = core.JSON(body)
	}
	req, e := http.NewRequest(method, h.server.URL+"/api"+path, bytes.NewReader(payload))
	if e != nil {
		h.t.Fatal(e)
	}
	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		h.t.Fatal(e)
	}
	defer resp.Body.Close()
	out := []map[string]any{}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	return resp.StatusCode, out
}

func (h testAPI) call(method, path, token, key string, body any) (int, map[string]any) {
	h.t.Helper()
	var data []byte
	if body != nil {
		data = core.JSON(body)
	}
	req, e := http.NewRequest(method, h.server.URL+"/api"+path, bytes.NewReader(data))
	if e != nil {
		h.t.Fatal(e)
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	if key != "" {
		req.Header.Set("Idempotency-Key", key)
	}
	resp, e := http.DefaultClient.Do(req)
	if e != nil {
		h.t.Fatal(e)
	}
	defer resp.Body.Close()
	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		h.t.Fatal(err)
	}
	validateFixtureResponse(h.t, req, resp, payload)
	out := map[string]any{}
	_ = json.Unmarshal(payload, &out)
	return resp.StatusCode, out
}
func must(t *testing.T, code, want int, v map[string]any) {
	t.Helper()
	if code != want {
		t.Fatalf("status %d want %d: %+v", code, want, v)
	}
}
func get(v map[string]any, k string) string {
	if s, ok := v[k].(string); ok {
		return s
	}
	return ""
}
func TestWorkflowAndInvariants(t *testing.T) {
	pool := testDB(t)
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	h := testAPI{t, server}
	suffix := core.ID()
	reg := func(name string) string {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": name + suffix + "@example.com", "password": "password123", "name": name})
		must(t, code, 200, v)
		return get(v, "token")
	}
	alice, bob, charlie := reg("alice"), reg("bob"), reg("charlie")
	code, v := h.call("POST", "/households", alice, "", map[string]any{"name": "A", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	code, v = h.call("POST", "/households", charlie, "", map[string]any{"name": "B", "servings": 2})
	must(t, code, 201, v)
	code, v = h.call("GET", root+"/inventory", charlie, "", nil)
	must(t, code, 404, v)
	code, v = h.call("POST", root+"/invites", alice, "", map[string]any{"email": "bob" + suffix + "@example.com", "role": "editor"})
	must(t, code, 201, v)
	code, v = h.call("POST", "/invites/accept", bob, "", map[string]any{"code": get(v, "code")})
	must(t, code, 200, v)
	var bobID string
	_ = pool.QueryRow(context.Background(), "SELECT user_id FROM sessions WHERE token_hash=$1", core.Hash(bob)).Scan(&bobID)
	code, v = h.call("PATCH", root+"/members/"+bobID, charlie, "", map[string]any{"role": "viewer"})
	must(t, code, 404, v)
	code, v = h.call("PATCH", root+"/members/"+bobID, alice, "", map[string]any{"role": "viewer"})
	must(t, code, 204, v)
	code, v = h.call("POST", root+"/ingredients", bob, "", map[string]any{"name": "不应创建", "unit": "g"})
	must(t, code, 403, v)
	code, v = h.call("PATCH", root+"/members/"+bobID, alice, "", map[string]any{"role": "editor"})
	must(t, code, 204, v)
	code, v = h.call("POST", root+"/ingredients", alice, "", map[string]any{"name": "生菜", "unit": "g", "low": "0.1"})
	must(t, code, 201, v)
	ingredient := get(v, "id")
	key := core.ID()
	purchase := map[string]any{"ingredient_id": ingredient, "quantity": "1", "reason": "manual"}
	code, v = h.call("POST", root+"/stock", alice, key, purchase)
	must(t, code, 200, v)
	batch := get(v, "batch_id")
	code, v = h.call("POST", root+"/stock", alice, key, purchase)
	must(t, code, 200, v)
	if batch != get(v, "batch_id") {
		t.Fatal("idempotency changed batch")
	}
	code, v = h.call("POST", root+"/stock", alice, key, map[string]any{"ingredient_id": ingredient, "quantity": "2", "reason": "manual"})
	must(t, code, 409, v)
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			actor := alice
			if i == 1 {
				actor = bob
			}
			codes[i], _ = h.call("POST", root+"/stock", actor, core.ID(), map[string]any{"ingredient_id": ingredient, "batch_id": batch, "quantity": "0.750", "reason": "consume"})
		}(i)
	}
	wg.Wait()
	if !((codes[0] == 200 && codes[1] == 409) || (codes[1] == 200 && codes[0] == 409)) {
		t.Fatalf("concurrent deductions %+v", codes)
	}
	code, v = h.call("GET", root+"/inventory", alice, "", nil)
	must(t, code, 200, v)
	items := v["items"].([]any)
	if items[0].(map[string]any)["quantity"] != "0.250" {
		t.Fatalf("stock lost update: %+v", items)
	}
	// Changing servings must scale ingredient demand before confirmation.
	recipe := "10000000-0000-4000-8000-000000000001"
	day := time.Now().Format("2006-01-02")
	code, v = h.call("POST", root+"/plans", alice, "", map[string]any{"meals": []any{map[string]any{"day": day, "meal": "lunch", "servings": 2, "recipe_id": recipe}}})
	must(t, code, 201, v)
	plan := get(v, "id")
	code, v = h.call("GET", root+"/plans/"+plan, alice, "", nil)
	must(t, code, 200, v)
	meal := v["meals"].([]any)[0].(map[string]any)
	code, v = h.call("PATCH", root+"/plans/"+plan+"/meals/"+get(meal, "id"), alice, "", map[string]any{"recipe_id": recipe, "servings": 4})
	must(t, code, 204, v)
	code, v = h.call("GET", root+"/plans/"+plan, alice, "", nil)
	must(t, code, 200, v)
	demands := v["ingredients"].([]any)
	found := false
	for _, raw := range demands {
		d := raw.(map[string]any)
		if d["name"] == "鸡蛋" {
			found = true
			if d["needed"] != "4.000" {
				t.Fatalf("servings not scaled: %+v", d)
			}
		}
	}
	if !found {
		t.Fatal("egg demand absent")
	}
	// A gram inventory entry cannot satisfy a recipe measured in pieces.
	code, v = h.call("POST", root+"/ingredients", alice, "", map[string]any{"name": "鸡蛋", "unit": "g"})
	must(t, code, 201, v)
	egg := get(v, "id")
	code, v = h.call("POST", root+"/stock", alice, core.ID(), map[string]any{"ingredient_id": egg, "quantity": "100", "reason": "manual"})
	must(t, code, 200, v)
	code, v = h.call("GET", root+"/plans/"+plan, alice, "", nil)
	must(t, code, 200, v)
	for _, raw := range v["ingredients"].([]any) {
		d := raw.(map[string]any)
		if d["name"] == "鸡蛋" && d["conversion_needs_confirmation"] != true {
			t.Fatal("incompatible units not flagged")
		}
	}
	code, v = h.call("POST", root+"/plans/"+plan+"/confirm", alice, core.ID(), map[string]any{"revision": 2, "accept_uncertain": false})
	must(t, code, 409, v)
	code, v = h.call("POST", root+"/plans/"+plan+"/reject", alice, "", map[string]any{})
	must(t, code, 204, v)
	// A cancelled job is never claimed; an expired lease can be reclaimed with a new token.
	payload := planJob{Day: day, Meal: "dinner", Servings: 2, MaxMinutes: 10}
	job := core.ID()
	_, e := pool.Exec(context.Background(), "INSERT INTO jobs(id,household_id,kind,status,payload,created_by) VALUES($1,$2,'plan','queued',$3,$4)", job, house, core.JSON(payload), core.ID())
	if e == nil {
		t.Fatal("invalid creator accepted")
	}
	var userID string
	_ = pool.QueryRow(context.Background(), "SELECT user_id FROM sessions WHERE token_hash=$1", core.Hash(alice)).Scan(&userID)
	_, e = pool.Exec(context.Background(), "INSERT INTO jobs(id,household_id,kind,status,payload,created_by) VALUES($1,$2,'plan','queued',$3,$4)", job, house, core.JSON(payload), userID)
	if e != nil {
		t.Fatal(e)
	}
	a := New(pool)
	j1, e := a.claim(context.Background(), "worker-one")
	if e != nil {
		t.Fatal(e)
	}
	if j1.ID != job {
		t.Fatalf("unexpected job %s", j1.ID)
	}
	_, _ = pool.Exec(context.Background(), "UPDATE jobs SET lease_until=now()-interval '1 second' WHERE id=$1", job)
	j2, e := a.claim(context.Background(), "worker-two")
	if e != nil {
		t.Fatal(e)
	}
	if j1.Token == j2.Token {
		t.Fatal("lease token reused")
	}
	a.runJob(context.Background(), j1, "worker-one")
	a.runJob(context.Background(), j2, "worker-two")
	var count int
	_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM plans WHERE household_id=$1 AND created_by=$2 AND status='draft'", house, userID).Scan(&count)
	if count != 1 {
		t.Fatalf("lease fencing made %d plans", count)
	}
	var planned string
	_ = pool.QueryRow(context.Background(), "SELECT result->>'plan_id' FROM jobs WHERE id=$1", job).Scan(&planned)
	todayCode, todayMeals := h.list(root+"/today?day="+day, alice)
	if todayCode != 200 || len(todayMeals) != 0 {
		t.Fatalf("draft leaked into today: %d %+v", todayCode, todayMeals)
	}
	code, v = h.call("POST", root+"/plans/"+planned+"/confirm", alice, core.ID(), map[string]any{"revision": 1, "accept_uncertain": false})
	must(t, code, 200, v)
	todayCode, todayMeals = h.list(root+"/today?day="+day, alice)
	if todayCode != 200 || len(todayMeals) != 1 || todayMeals[0]["plan_id"] != planned {
		t.Fatalf("confirmed meal missing today: %d %+v", todayCode, todayMeals)
	}
	var listID string
	_ = pool.QueryRow(context.Background(), "SELECT id FROM shopping_lists WHERE plan_id=$1", planned).Scan(&listID)
	rows, e := pool.Query(context.Background(), "SELECT id,needed_milli FROM shopping_items WHERE list_id=$1", listID)
	if e != nil {
		t.Fatal(e)
	}
	type shopItem struct {
		id string
		q  int64
	}
	shopping := []shopItem{}
	for rows.Next() {
		var item shopItem
		if e = rows.Scan(&item.id, &item.q); e != nil {
			t.Fatal(e)
		}
		shopping = append(shopping, item)
	}
	rows.Close()
	for _, item := range shopping {
		code, v = h.call("PATCH", root+"/shopping/"+item.id, bob, "", map[string]any{"checked": true, "bought": core.Format(item.q)})
		must(t, code, 204, v)
		key = core.ID()
		body := map[string]any{"source": "test purchase"}
		code, v = h.call("POST", root+"/shopping/"+item.id+"/stock", bob, key, body)
		must(t, code, 200, v)
		code, v = h.call("POST", root+"/shopping/"+item.id+"/stock", bob, key, body)
		must(t, code, 200, v)
		_ = pool.QueryRow(context.Background(), "SELECT count(*) FROM stock_ledger WHERE ref_type='shopping_item' AND ref_id=$1", item.id).Scan(&count)
		if count != 1 {
			t.Fatal("shopping item stocked twice")
		}
		code, v = h.call("POST", root+"/shopping/"+item.id+"/stock", bob, core.ID(), body)
		must(t, code, 409, v)
	}
	key = core.ID()
	code, v = h.call("POST", root+"/plans/"+planned+"/consume", alice, key, map[string]any{})
	must(t, code, 200, v)
	code, v = h.call("POST", root+"/plans/"+planned+"/consume", alice, key, map[string]any{})
	must(t, code, 200, v)
	code, v = h.call("POST", root+"/plans/"+planned+"/consume", alice, core.ID(), map[string]any{})
	must(t, code, 409, v)
	code, v = h.call("POST", root+"/jobs/plan", alice, "", map[string]any{"day": day, "meal": "lunch", "servings": 2, "max_minutes": 10})
	must(t, code, 202, v)
	cancel := get(v, "id")
	code, v = h.call("POST", root+"/jobs/"+cancel+"/cancel", alice, "", map[string]any{})
	must(t, code, 204, v)
	code, v = h.call("GET", root+"/jobs/"+cancel, alice, "", nil)
	must(t, code, 200, v)
	if v["status"] != "cancelled" {
		t.Fatal("cancel not persisted")
	}
	// Tool-level authorization is checked again when a worker runs.
	var otherID string
	_ = pool.QueryRow(context.Background(), "SELECT user_id FROM sessions WHERE token_hash=$1", core.Hash(charlie)).Scan(&otherID)
	_, _, e = a.chooseRecipe(context.Background(), claimed{ID: job, Household: house, Creator: otherID}, payload)
	if e == nil {
		t.Fatal("cross-household agent tool allowed")
	}
	allowed, _, e := a.chooseRecipe(context.Background(), claimed{ID: job, Household: house, Creator: userID}, planJob{Day: day, Meal: "dinner", Servings: 2, MaxMinutes: 10, ExcludedIngredients: []string{"生菜"}})
	if e != nil {
		t.Fatal("allowed alternative was rejected", e)
	}
	var prohibited int
	if e = pool.QueryRow(context.Background(), "SELECT count(*) FROM recipe_items WHERE recipe_id=$1 AND name='生菜'", allowed).Scan(&prohibited); e != nil || prohibited != 0 {
		t.Fatal("agent selected an explicitly excluded ingredient", prohibited, e)
	}
	_, _, e = a.chooseRecipe(context.Background(), claimed{ID: job, Household: house, Creator: userID}, planJob{Day: day, Meal: "dinner", Servings: 2, MaxMinutes: 10, ExcludedIngredients: []string{"生菜", "菠菜"}})
	if e == nil {
		t.Fatal("agent must reject when all eligible recipes are excluded")
	}
	code, v = h.call("PATCH", root, alice, "", map[string]any{"name": "A", "servings": 2, "preferences": "", "excluded_ingredients": []string{"鸡蛋"}})
	must(t, code, 204, v)
	code, v = h.call("POST", root+"/plans", alice, "", map[string]any{"meals": []any{map[string]any{"day": day, "meal": "lunch", "servings": 2, "recipe_id": recipe}}})
	must(t, code, 201, v)
	code, v = h.call("POST", root+"/plans/"+get(v, "id")+"/confirm", alice, core.ID(), map[string]any{"revision": 1, "accept_uncertain": true})
	must(t, code, 409, v)
	fmt.Println("integration invariants verified")
}
