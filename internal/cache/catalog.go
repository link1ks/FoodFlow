// Package cache caches public reference data only, with bounded fallback latency.
package cache

import (
	"context"
	"errors"
	"github.com/redis/go-redis/v9"
	"strings"
	"sync/atomic"
	"time"
)

const CatalogKey = "foodflow:catalog:v1"

const operationTimeout = 120 * time.Millisecond

type Catalog struct {
	Client                                 *redis.Client
	Hits, Misses, Errors                   atomic.Uint64
	PublicHits, PublicMisses, PublicErrors atomic.Uint64
}

func New(url string) (*Catalog, error) {
	if strings.TrimSpace(url) == "" {
		return nil, errors.New("REDIS_URL is required")
	}
	opts, err := redis.ParseURL(url)
	if err != nil {
		// Parse errors can contain the configured URL, including credentials.
		return nil, errors.New("invalid REDIS_URL")
	}
	opts.DialTimeout = 100 * time.Millisecond
	opts.ReadTimeout = 100 * time.Millisecond
	opts.WriteTimeout = 100 * time.Millisecond
	opts.MaxRetries = -1
	opts.ContextTimeoutEnabled = true
	return &Catalog{Client: redis.NewClient(opts)}, nil
}

// Ping checks the required API dependency without exposing connection details.
func (c *Catalog) Ping(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if c.Client.Ping(ctx).Err() != nil {
		return errors.New("redis unavailable")
	}
	return nil
}

func (c *Catalog) Close() error { return c.Client.Close() }

func (c *Catalog) Get(ctx context.Context) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
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
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if err := c.Client.Set(ctx, CatalogKey, raw, 60*time.Second).Err(); err != nil {
		c.Errors.Add(1)
	}
}
