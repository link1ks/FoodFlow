package app

import (
	"bytes"
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"strings"
)

func (a *App) uploadIngredientImage(c *gin.Context) {
	if !writable(c) {
		return
	}
	store, e := imageStore()
	if e != nil {
		fail(c, 503, "image storage unavailable")
		return
	}
	data, mime, ext, e := readUploadedImage(c)
	if e != nil {
		fail(c, 400, e.Error())
		return
	}
	key := hid(c) + "/ingredients/" + c.Param("ingredient") + "/" + core.ID() + ext
	if e = store.Put(c, key, bytes.NewReader(data), mime); e != nil {
		fail(c, 500, "image storage failed")
		return
	}
	committed := false
	defer func() {
		if !committed {
			_ = store.Delete(c, key)
		}
	}()
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "transaction failed")
		return
	}
	defer tx.Rollback(c)
	var oldKey string
	e = tx.QueryRow(c, "SELECT COALESCE(image_key,'') FROM ingredients WHERE id=$1 AND household_id=$2 AND archived_at IS NULL FOR UPDATE", c.Param("ingredient"), hid(c)).Scan(&oldKey)
	if e == pgx.ErrNoRows {
		fail(c, 404, "ingredient unavailable")
		return
	}
	if e != nil {
		fail(c, 500, "ingredient unavailable")
		return
	}
	_, e = tx.Exec(c, "UPDATE ingredients SET image_key=$1,image_mime=$2,image_version=image_version+1 WHERE id=$3 AND household_id=$4", key, mime, c.Param("ingredient"), hid(c))
	if e == nil {
		e = tx.Commit(c)
	}
	if e != nil {
		fail(c, 500, "image update failed")
		return
	}
	committed = true
	if oldKey != "" {
		_ = store.Delete(c, oldKey)
	}
	c.JSON(200, gin.H{"has_image": true})
}

func (a *App) ingredientImage(c *gin.Context) {
	var key, mime string
	e := a.DB.QueryRow(c, "SELECT COALESCE(image_key,''),COALESCE(image_mime,'') FROM ingredients WHERE id=$1 AND household_id=$2 AND archived_at IS NULL", c.Param("ingredient"), hid(c)).Scan(&key, &mime)
	if e != nil || key == "" || !strings.HasPrefix(key, hid(c)+"/ingredients/") {
		fail(c, 404, "ingredient image unavailable")
		return
	}
	store, e := imageStore()
	if e != nil {
		fail(c, 503, "image storage unavailable")
		return
	}
	r, e := store.Open(c, key)
	if e != nil {
		fail(c, 404, "ingredient image unavailable")
		return
	}
	defer r.Close()
	c.Header("Cache-Control", "no-store")
	c.Header("X-Content-Type-Options", "nosniff")
	c.DataFromReader(200, -1, mime, r, nil)
}

func (a *App) deleteIngredientImage(c *gin.Context) {
	if !writable(c) {
		return
	}
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "transaction failed")
		return
	}
	defer tx.Rollback(c)
	var key string
	e = tx.QueryRow(c, "SELECT COALESCE(image_key,'') FROM ingredients WHERE id=$1 AND household_id=$2 AND archived_at IS NULL FOR UPDATE", c.Param("ingredient"), hid(c)).Scan(&key)
	if e == pgx.ErrNoRows {
		fail(c, 404, "ingredient unavailable")
		return
	}
	if e != nil {
		fail(c, 500, "ingredient unavailable")
		return
	}
	if key != "" {
		_, e = tx.Exec(c, "UPDATE ingredients SET image_key=NULL,image_mime=NULL,image_version=image_version+1 WHERE id=$1 AND household_id=$2", c.Param("ingredient"), hid(c))
		if e != nil {
			fail(c, 500, "image update failed")
			return
		}
	}
	if e = tx.Commit(c); e != nil {
		fail(c, 500, "image update failed")
		return
	}
	if key != "" {
		if store, err := imageStore(); err == nil {
			_ = store.Delete(c, key)
		}
	}
	c.Status(204)
}
