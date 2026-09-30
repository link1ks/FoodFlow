package cache

import (
	"context"
	"testing"
	"time"
)

func TestUnavailableRedisHasBoundedFallback(t *testing.T) {
	c, err := New("redis://127.0.0.1:1/0")
	if err != nil {
		t.Fatal(err)
	}
	defer c.Client.Close()
	start := time.Now()
	if _, ok := c.Get(context.Background()); ok {
		t.Fatal("unavailable cache returned hit")
	}
	c.Set(context.Background(), []byte("[]"))
	if elapsed := time.Since(start); elapsed > time.Second {
		t.Fatalf("fallback blocked for %v", elapsed)
	}
	if c.Errors.Load() != 2 {
		t.Fatal("cache errors not observable")
	}
}
