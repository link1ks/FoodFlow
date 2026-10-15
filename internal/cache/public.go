package cache

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
)

// PublicTTL bounds staleness for rebuildable public reference data. Inventory,
// household permissions, sessions and write confirmations never use this cache.
const PublicTTL = 60 * time.Second

// PublicKey keeps filter boundaries unambiguous and user query text out of keys.
// Callers include every value that changes the serialized response, including
// the Shanghai calendar date for price freshness labels.
func PublicKey(resource string, filters ...string) string {
	parts := append([]string{resource}, filters...)
	raw, _ := json.Marshal(parts)
	digest := sha256.Sum256(raw)
	return "foodflow:public:v1:" + hex.EncodeToString(digest[:])
}

func (c *Catalog) GetPublic(ctx context.Context, key string) ([]byte, bool) {
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	raw, err := c.Client.Get(ctx, key).Bytes()
	if err == redis.Nil {
		c.PublicMisses.Add(1)
		return nil, false
	}
	if err != nil || !json.Valid(raw) {
		c.PublicErrors.Add(1)
		return nil, false
	}
	c.PublicHits.Add(1)
	return raw, true
}

func (c *Catalog) SetPublic(ctx context.Context, key string, raw []byte) {
	if !json.Valid(raw) {
		c.PublicErrors.Add(1)
		return
	}
	ctx, cancel := context.WithTimeout(ctx, operationTimeout)
	defer cancel()
	if c.Client.Set(ctx, key, raw, PublicTTL).Err() != nil {
		c.PublicErrors.Add(1)
	}
}
