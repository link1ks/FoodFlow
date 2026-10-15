package app

import (
	"encoding/json"

	"github.com/gin-gonic/gin"
)

func (a *App) publicCacheHit(c *gin.Context, key string) bool {
	if a.catalogCache == nil {
		return false
	}
	raw, ok := a.catalogCache.GetPublic(c.Request.Context(), key)
	if !ok {
		return false
	}
	c.Header("X-Public-Cache", "hit")
	c.Data(200, "application/json", raw)
	return true
}

// Only complete, successfully serialized database reads enter the public cache.
func (a *App) publicCacheJSON(c *gin.Context, key string, value any) {
	raw, err := json.Marshal(value)
	if err != nil {
		fail(c, 500, "public reference unavailable")
		return
	}
	if a.catalogCache != nil {
		a.catalogCache.SetPublic(c.Request.Context(), key, raw)
	}
	c.Header("X-Public-Cache", "database")
	c.Data(200, "application/json", raw)
}
