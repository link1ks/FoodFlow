package cache

import (
	"context"
	"strings"
	"testing"
	"time"
)

func TestPublicKeysSeparateResourcesFiltersAndDates(t *testing.T) {
	keys := map[string]bool{}
	for _, parts := range [][]string{
		{"recipes", "a:b", "c"},
		{"recipes", "a", "b:c"},
		{"price-benchmarks", "a:b", "c"},
		{"price-dashboard", "2026-10-15", "上海市", "番茄"},
		{"price-dashboard", "2026-10-16", "上海市", "番茄"},
		{"price-dashboard", "2026-10-15", "北京市", "番茄"},
		{"price-dashboard", "2026-10-15", "上海市", "土豆"},
	} {
		key := PublicKey(parts[0], parts[1:]...)
		if keys[key] || len(key) != len("foodflow:public:v1:")+64 || strings.Contains(key, "番茄") || strings.Contains(key, "2026-10-15") {
			t.Fatalf("public key collision or raw filter disclosure: %q", key)
		}
		keys[key] = true
		if key != PublicKey(parts[0], parts[1:]...) {
			t.Fatal("public key is not stable")
		}
	}
}

func TestPublicCacheUnavailableHasSeparateBoundedMetrics(t *testing.T) {
	c, err := New("redis://127.0.0.1:1/0")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	start := time.Now()
	key := PublicKey("recipes", "", "", "")
	if _, ok := c.GetPublic(context.Background(), key); ok {
		t.Fatal("unavailable public cache returned a hit")
	}
	c.SetPublic(context.Background(), key, []byte("[]"))
	if time.Since(start) > time.Second {
		t.Fatal("public cache outage did not remain bounded")
	}
	if c.PublicErrors.Load() != 2 || c.Hits.Load() != 0 || c.Misses.Load() != 0 || c.Errors.Load() != 0 {
		t.Fatal("public operations changed catalog metrics or lost errors")
	}
	c.SetPublic(context.Background(), key, []byte("invalid JSON"))
	if c.PublicErrors.Load() != 3 {
		t.Fatal("invalid serialized data was accepted")
	}
}
