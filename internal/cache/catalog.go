// Package cache caches public catalog data only, with bounded fallback latency.
package cache

import (
	"context"
	"github.com/redis/go-redis/v9"
	"sync/atomic"
	"time"
)

const CatalogKey = "foodflow:catalog:v1"

type Catalog struct {
	Client               *redis.Client
	Hits, Misses, Errors atomic.Uint64
}

func New(url string) (*Catalog, error) {
	opts, err := redis.ParseURL(url)
	if err != nil {
		return nil, err
	}
	opts.DialTimeout = 100 * time.Millisecond
	opts.ReadTimeout = 100 * time.Millisecond
	opts.WriteTimeout = 100 * time.Millisecond
	opts.MaxRetries = -1
	opts.ContextTimeoutEnabled = true
	return &Catalog{Client: redis.NewClient(opts)}, nil
}
func (c *Catalog) Get(ctx context.Context) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Millisecond)
	defer cancel()
	raw, err := c.Client.Get(ctx, CatalogKey).Bytes()
	if err == redis.Nil {
		c.Misses.Add(1)
		return nil, false
	}
	if err != nil {
		c.Errors.Add(1)
		return nil, false
	}
	c.Hits.Add(1)
	return raw, true
}
func (c *Catalog) Set(ctx context.Context, raw []byte) {
	ctx, cancel := context.WithTimeout(ctx, 120*time.Millisecond)
	defer cancel()
	if err := c.Client.Set(ctx, CatalogKey, raw, 60*time.Second).Err(); err != nil {
		c.Errors.Add(1)
	}
}
