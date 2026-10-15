package app

import (
	"context"
	"fmt"
	"foodflow/internal/cache"
	"foodflow/internal/core"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

func testRedis(t *testing.T) string {
	t.Helper()
	ctx := context.Background()
	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: testcontainers.ContainerRequest{
			Image:        "redis:7.4-alpine",
			ExposedPorts: []string{"6379/tcp"},
			Cmd:          []string{"redis-server", "--save", "", "--appendonly", "no"},
			WaitingFor:   wait.ForListeningPort("6379/tcp").WithStartupTimeout(30 * time.Second),
		},
		Started: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = container.Terminate(ctx) })
	host, err := container.Host(ctx)
	if err != nil {
		t.Fatal(err)
	}
	port, err := container.MappedPort(ctx, "6379/tcp")
	if err != nil {
		t.Fatal(err)
	}
	return fmt.Sprintf("redis://%s:%s/0", host, port.Port())
}

func publicCacheRequest(t *testing.T, server *httptest.Server, path, token string) (int, string, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, server.URL+"/api"+path, nil)
	if err != nil {
		t.Fatal(err)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	res, err := server.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	validateFixtureResponse(t, req, res, raw)
	return res.StatusCode, res.Header.Get("X-Public-Cache"), string(raw)
}

func TestPublicRedisCachesShareReferencesAndPreserveAuthorization(t *testing.T) {
	db := testDB(t)
	t.Setenv("REDIS_URL", testRedis(t))
	ctx := context.Background()
	a, err := NewAPI(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	b, err := NewAPI(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer b.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	otherServer := httptest.NewServer(b.Router())
	defer otherServer.Close()
	h := testAPI{t, server}
	register := func() string {
		code, account := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "password123", "name": "public cache"})
		must(t, code, 200, account)
		return get(account, "token")
	}
	token, outsider := register(), register()
	code, household := h.call("POST", "/households", token, "", map[string]any{"name": "public cache", "servings": 2})
	must(t, code, 201, household)
	root := "/households/" + get(household, "id") + "/prices"
	for _, path := range []string{
		"/recipes",
		root + "/benchmarks?" + url.Values{"city": {"北京市"}}.Encode(),
		root + "/dashboard?" + url.Values{"city": {"北京市"}, "ingredient_name": {"西红柿"}}.Encode(),
	} {
		code, source, first := publicCacheRequest(t, server, path, token)
		if code != 200 || source != "database" {
			t.Fatalf("public reference miss = %d %q", code, source)
		}
		code, source, second := publicCacheRequest(t, otherServer, path, token)
		if code != 200 || source != "hit" || second != first {
			t.Fatalf("second API did not share exact Redis reference: %d %q", code, source)
		}
		code, source, _ = publicCacheRequest(t, server, path, "")
		if code != 401 || source != "" {
			t.Fatal("cached reference bypassed authentication")
		}
		if path != "/recipes" {
			code, source, _ = publicCacheRequest(t, server, path, outsider)
			if code != 404 || source != "" {
				t.Fatal("cached price reference bypassed household authorization")
			}
		}
	}
	for _, path := range []string{"/recipes?max_minutes=1", "/recipes?tag=redis-fixture", "/recipes?ingredient=redis-fixture", root + "/benchmarks?" + url.Values{"city": {"上海市"}}.Encode(), root + "/benchmarks?" + url.Values{"city": {"北京市"}, "category": {"蔬菜"}}.Encode(), root + "/dashboard?" + url.Values{"city": {"北京市"}, "ingredient_name": {"土豆"}}.Encode()} {
		if code, source, _ := publicCacheRequest(t, server, path, token); code != 200 || source != "database" {
			t.Fatalf("distinct filters reused another cache entry: %d %q", code, source)
		}
	}
	if code, source, _ := publicCacheRequest(t, server, "/recipes?max_minutes=invalid", token); code != 400 || source != "" {
		t.Fatal("recipe cache bypassed filter validation")
	}
	key := cache.PublicKey("recipes", "", "", "")
	ttl, err := a.catalogCache.Client.PTTL(ctx, key).Result()
	if err != nil || ttl <= 0 || ttl > cache.PublicTTL {
		t.Fatalf("public reference TTL = %v, %v", ttl, err)
	}
	if err = a.catalogCache.Client.Set(ctx, key, "{broken", time.Minute).Err(); err != nil {
		t.Fatal(err)
	}
	if code, source, _ := publicCacheRequest(t, server, "/recipes", token); code != 200 || source != "database" {
		t.Fatal("invalid cache JSON was served")
	}
	if err = a.catalogCache.Client.PExpire(ctx, key, time.Millisecond).Err(); err != nil {
		t.Fatal(err)
	}
	time.Sleep(5 * time.Millisecond)
	if code, source, _ := publicCacheRequest(t, server, "/recipes", token); code != 200 || source != "database" {
		t.Fatal("expired public reference did not refresh")
	}
	if a.catalogCache.PublicErrors.Load() == 0 || b.catalogCache.PublicHits.Load() != 3 || a.catalogCache.Hits.Load() != 0 || a.catalogCache.Misses.Load() != 0 || a.catalogCache.Errors.Load() != 0 {
		t.Fatal("public cache metrics changed catalog counters or omitted Redis outcomes")
	}
	a.Close()
	if code, source, _ := publicCacheRequest(t, server, "/recipes", token); code != 200 || source != "database" {
		t.Fatal("public reference outage did not fall back to PostgreSQL")
	}
}

func TestPublicRedisNeverCachesIncompleteRecipeMaterials(t *testing.T) {
	db := testDB(t)
	t.Setenv("REDIS_URL", testRedis(t))
	ctx := context.Background()
	a, err := NewAPI(ctx, db)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, account := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "password123", "name": "incomplete recipe cache"})
	must(t, code, 200, account)
	if _, err = db.Exec(ctx, "ALTER TABLE recipe_items RENAME COLUMN quantity_milli TO redis_fixture_quantity"); err != nil {
		t.Fatal(err)
	}
	defer func() {
		if _, err := db.Exec(ctx, "ALTER TABLE recipe_items RENAME COLUMN redis_fixture_quantity TO quantity_milli"); err != nil {
			t.Error(err)
		}
	}()
	code, source, _ := publicCacheRequest(t, server, "/recipes", get(account, "token"))
	if code != 500 || source != "" {
		t.Fatalf("incomplete recipe materials reported success: %d %q", code, source)
	}
	key := cache.PublicKey("recipes", "", "", "")
	if exists, err := a.catalogCache.Client.Exists(ctx, key).Result(); err != nil || exists != 0 {
		t.Fatalf("incomplete recipe entered Redis: %d %v", exists, err)
	}
}
