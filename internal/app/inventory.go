package app

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"foodflow/internal/core"
	"foodflow/internal/nutrition"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"strings"
	"time"
)

type stockRequest struct {
	IngredientID string `json:"ingredient_id"`
	BatchID      string `json:"batch_id"`
	Quantity     string `json:"quantity"`
	Reason       string `json:"reason"`
	Direction    string `json:"direction"`
	Location     string `json:"location"`
	BoughtOn     string `json:"bought_on"`
	ExpiresOn    string `json:"expires_on"`
	ExpiryKind   string `json:"expiry_kind"`
	Source       string `json:"source"`
	Note         string `json:"note"`
}

func (a *App) addIngredient(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x struct {
		CatalogID string `json:"catalog_id"`
		Name      string `json:"name"`
		Category  string `json:"category"`
		Unit      string `json:"unit"`
		Low       string `json:"low"`
	}
	if !input(c, &x) {
		return
	}
	if x.CatalogID != "" {
		if e := a.DB.QueryRow(c, "SELECT name,category,default_unit FROM ingredient_catalog WHERE id=$1", x.CatalogID).Scan(&x.Name, &x.Category, &x.Unit); e != nil {
			fail(c, 400, "catalog ingredient unavailable")
			return
		}
	}
	x.Name = strings.TrimSpace(x.Name)
	dim, _, e := core.Dimension(x.Unit)
	if e != nil || x.Name == "" {
		fail(c, 400, "name and supported unit required")
		return
	}
	low := int64(0)
	if x.Low != "" {
		low, e = core.Quantity(x.Low)
		if e != nil {
			fail(c, 400, e.Error())
			return
		}
	}
	id := core.ID()
	e = a.DB.QueryRow(c, "INSERT INTO ingredients(id,household_id,name,category,unit,dimension,low_milli,catalog_id) VALUES($1,$2,$3,$4,$5,$6,$7,NULLIF($8,'')::uuid) ON CONFLICT(household_id,name,unit) DO UPDATE SET category=EXCLUDED.category,low_milli=EXCLUDED.low_milli,catalog_id=COALESCE(EXCLUDED.catalog_id,ingredients.catalog_id),archived_at=NULL RETURNING id", id, hid(c), x.Name, x.Category, x.Unit, dim, low, x.CatalogID).Scan(&id)
	if e != nil {
		fail(c, 500, "ingredient failed")
		return
	}
	c.JSON(201, gin.H{"id": id})
}
func (a *App) ingredientCatalog(c *gin.Context) {
	rows, e := a.DB.Query(c, "SELECT id,name,category,default_unit,aliases FROM ingredient_catalog ORDER BY category,name")
	if e != nil {
		fail(c, 500, "catalog unavailable")
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id, name, category, unit string
		var aliases []string
		if e := rows.Scan(&id, &name, &category, &unit, &aliases); e != nil {
			fail(c, 500, "catalog unavailable")
			return
		}
		items = append(items, gin.H{"id": id, "name": name, "category": category, "default_unit": unit, "aliases": aliases, "nutrition": nutrition.Lookup(name)})
	}
	if rows.Err() != nil {
		fail(c, 500, "catalog unavailable")
		return
	}
	c.JSON(200, items)
}

func (a *App) catalogStock(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x struct {
		CatalogID string `json:"catalog_id"`
		Quantity  string `json:"quantity"`
		Low       string `json:"low"`
		Location  string `json:"location"`
		BoughtOn  string `json:"bought_on"`
		ExpiresOn string `json:"expires_on"`
	}
	if !input(c, &x) {
		return
	}
	if x.CatalogID == "" {
		fail(c, 400, "catalog_id required")
		return
	}
	quantity, e := core.Quantity(x.Quantity)
	if e != nil {
		fail(c, 400, e.Error())
		return
	}
	low := int64(0)
	if x.Low != "" {
		low, e = core.Quantity(x.Low)
		if e != nil {
			fail(c, 400, e.Error())
			return
		}
	}
	for _, date := range []string{x.BoughtOn, x.ExpiresOn} {
		if date != "" {
			if _, e = time.Parse("2006-01-02", date); e != nil {
				fail(c, 400, "invalid date")
				return
			}
		}
	}
	a.idem(c, x, func(tx pgx.Tx) (any, error) {
		var name, category, unit string
		if e := tx.QueryRow(c, "SELECT name,category,default_unit FROM ingredient_catalog WHERE id=$1", x.CatalogID).Scan(&name, &category, &unit); e != nil {
			return nil, bad("catalog ingredient unavailable")
		}
		dim, _, e := core.Dimension(unit)
		if e != nil {
			return nil, e
		}
		ingredientID := core.ID()
		e = tx.QueryRow(c, "INSERT INTO ingredients(id,household_id,name,category,unit,dimension,low_milli,catalog_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8) ON CONFLICT(household_id,name,unit) DO UPDATE SET category=EXCLUDED.category,low_milli=CASE WHEN $9 THEN EXCLUDED.low_milli ELSE ingredients.low_milli END,catalog_id=EXCLUDED.catalog_id,archived_at=NULL RETURNING id", ingredientID, hid(c), name, category, unit, dim, low, x.CatalogID, x.Low != "").Scan(&ingredientID)
		if e != nil {
			return nil, e
		}
		batchID := core.ID()
		kind := "unknown"
		if x.ExpiresOn != "" {
			kind = "user"
		}
		_, e = tx.Exec(c, "INSERT INTO batches(id,household_id,ingredient_id,quantity_milli,location,bought_on,expires_on,expiry_kind,source) VALUES($1,$2,$3,$4,$5,NULLIF($6,'')::date,NULLIF($7,'')::date,$8,$9)", batchID, hid(c), ingredientID, quantity, x.Location, x.BoughtOn, x.ExpiresOn, kind, "目录录入")
		if e != nil {
			return nil, e
		}
		_, e = tx.Exec(c, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id) VALUES($1,$2,$3,$4,'manual',$5)", core.ID(), hid(c), batchID, quantity, uid(c))
		return gin.H{"ingredient_id": ingredientID, "batch_id": batchID}, e
	})
}
func (a *App) inventory(c *gin.Context) {
	rows, e := a.DB.Query(c, `SELECT i.id,i.name,i.category,i.unit,i.dimension,i.low_milli,COALESCE(sum(b.quantity_milli) FILTER (WHERE b.condition='normal' AND (b.expires_at IS NULL OR b.expires_at>now())),0),i.image_key IS NOT NULL,i.image_version FROM ingredients i LEFT JOIN batches b ON b.ingredient_id=i.id AND b.household_id=i.household_id WHERE i.household_id=$1 AND i.archived_at IS NULL GROUP BY i.id ORDER BY i.name`, hid(c))
	if e != nil {
		fail(c, 500, "inventory unavailable")
		return
	}
	defer rows.Close()
	items := []gin.H{}
	for rows.Next() {
		var id, name, cat, unit, dim string
		var low, total, imageVersion int64
		var hasImage bool
		if rows.Scan(&id, &name, &cat, &unit, &dim, &low, &total, &hasImage, &imageVersion) == nil {
			items = append(items, gin.H{"id": id, "name": name, "category": cat, "unit": unit, "dimension": dim, "low": core.Format(low), "quantity": core.Format(total), "low_stock": total <= low, "has_image": hasImage, "image_version": imageVersion, "nutrition": nutrition.Lookup(name)})
		}
	}
	rows.Close()
	br, e := a.DB.Query(c, `SELECT b.id,b.ingredient_id,b.quantity_milli,b.location,COALESCE(b.bought_on::text,''),
		COALESCE(b.expires_on::text,''),COALESCE(to_char(b.expires_at AT TIME ZONE 'UTC','YYYY-MM-DD"T"HH24:MI:SS"Z"'),''),
		b.expiry_kind,b.source,b.condition FROM batches b JOIN ingredients i ON i.id=b.ingredient_id
		WHERE b.household_id=$1 AND i.archived_at IS NULL ORDER BY b.expires_at NULLS LAST,b.created_at`, hid(c))
	if e != nil {
		fail(c, 500, "batches unavailable")
		return
	}
	defer br.Close()
	batches := []gin.H{}
	for br.Next() {
		var id, ingredient, location, bought, expires, expiresAt, kind, source, condition string
		var q int64
		if br.Scan(&id, &ingredient, &q, &location, &bought, &expires, &expiresAt, &kind, &source, &condition) == nil {
			batches = append(batches, gin.H{"id": id, "ingredient_id": ingredient, "quantity": core.Format(q), "location": location, "bought_on": bought, "expires_on": expires, "expires_at": expiresAt, "expiry_kind": kind, "source": source, "condition": condition})
		}
	}
	c.JSON(200, gin.H{"items": items, "batches": batches})
}
func (a *App) expiring(c *gin.Context) {
	rows, e := a.DB.Query(c, "SELECT b.id,i.name,b.quantity_milli,i.unit,b.expires_on::text,b.expiry_kind FROM batches b JOIN ingredients i ON i.id=b.ingredient_id WHERE b.household_id=$1 AND i.archived_at IS NULL AND b.quantity_milli>0 AND b.expires_on<=current_date+interval '7 days' ORDER BY b.expires_on", hid(c))
	if e != nil {
		fail(c, 500, "query failed")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, name, unit, date, kind string
		var q int64
		if rows.Scan(&id, &name, &q, &unit, &date, &kind) == nil {
			out = append(out, gin.H{"batch_id": id, "name": name, "quantity": core.Format(q), "unit": unit, "expires_on": date, "expiry_kind": kind})
		}
	}
	c.JSON(200, out)
}

type businessError struct {
	status int
	msg    string
}

func (b businessError) Error() string { return b.msg }
func conflict(msg string) error       { return businessError{409, msg} }
func bad(msg string) error            { return businessError{400, msg} }
func (a *App) idem(c *gin.Context, request any, run func(pgx.Tx) (any, error)) {
	key := strings.TrimSpace(c.GetHeader("Idempotency-Key"))
	if len(key) < 8 || len(key) > 120 {
		fail(c, 400, "Idempotency-Key of 8-120 characters required")
		return
	}
	// Bind the key to actor, operation and concrete resource, not just body.
	// Different endpoints can legitimately accept identical JSON bodies.
	digest := core.Hash(string(core.JSON(struct {
		Actor, Method, Path string
		Request             any
	}{uid(c), c.Request.Method, c.Request.URL.Path, request})))
	tx, e := a.DB.Begin(c)
	if e != nil {
		fail(c, 500, "transaction failed")
		return
	}
	defer tx.Rollback(c)
	_, e = tx.Exec(c, "INSERT INTO idempotency(household_id,key,request_hash) VALUES($1,$2,$3) ON CONFLICT DO NOTHING", hid(c), key, digest)
	if e != nil {
		fail(c, 500, "idempotency failed")
		return
	}
	var stored string
	var response []byte
	e = tx.QueryRow(c, "SELECT request_hash,response FROM idempotency WHERE household_id=$1 AND key=$2 FOR UPDATE", hid(c), key).Scan(&stored, &response)
	if e != nil {
		fail(c, 500, "idempotency failed")
		return
	}
	if stored != digest {
		fail(c, 409, "idempotency key used with different request")
		return
	}
	if response != nil {
		var v any
		_ = json.Unmarshal(response, &v)
		c.JSON(200, v)
		return
	}
	result, e := run(tx)
	if e != nil {
		var b businessError
		if errors.As(e, &b) {
			fail(c, b.status, b.msg)
		} else {
			fail(c, 500, "operation failed")
		}
		return
	}
	_, e = tx.Exec(c, "UPDATE idempotency SET response=$1 WHERE household_id=$2 AND key=$3", core.JSON(result), hid(c), key)
	if e == nil {
		e = tx.Commit(c)
	}
	if e != nil {
		fail(c, 500, "commit failed")
		return
	}
	c.JSON(200, result)
}
func (a *App) stock(c *gin.Context) {
	if !writable(c) {
		return
	}
	var x stockRequest
	if !input(c, &x) {
		return
	}
	q, e := core.Quantity(x.Quantity)
	if e != nil {
		fail(c, 400, e.Error())
		return
	}
	if x.Reason != "purchase" && x.Reason != "consume" && x.Reason != "waste" && x.Reason != "correction" && x.Reason != "manual" {
		fail(c, 400, "invalid reason")
		return
	}
	if x.ExpiresOn != "" {
		if _, e = time.Parse("2006-01-02", x.ExpiresOn); e != nil {
			fail(c, 400, "invalid expiry date")
			return
		}
		if x.ExpiryKind != "label" && x.ExpiryKind != "user" && x.ExpiryKind != "estimate" {
			fail(c, 400, "expiry kind required")
			return
		}
	} else {
		x.ExpiryKind = "unknown"
	}
	if x.BoughtOn != "" {
		if _, e = time.Parse("2006-01-02", x.BoughtOn); e != nil {
			fail(c, 400, "invalid purchase date")
			return
		}
	}
	if x.Direction != "" && x.Direction != "increase" && x.Direction != "decrease" {
		fail(c, 400, "invalid direction")
		return
	}
	if x.Direction != "" && x.Reason != "correction" {
		fail(c, 400, "direction only applies to correction")
		return
	}
	if x.Note != "" && (x.Reason != "waste" || (x.Note != "spoiled" && x.Note != "expired")) {
		fail(c, 400, "invalid waste note")
		return
	}
	a.idem(c, x, func(tx pgx.Tx) (any, error) {
		var exists string
		e := tx.QueryRow(c, "SELECT id FROM ingredients WHERE id=$1 AND household_id=$2 AND archived_at IS NULL FOR UPDATE", x.IngredientID, hid(c)).Scan(&exists)
		if e != nil {
			return nil, conflict("ingredient unavailable")
		}
		batchID := x.BatchID
		delta := q
		if x.Reason == "consume" || x.Reason == "waste" || x.Direction == "decrease" {
			delta = -q
		}
		if batchID == "" {
			if delta < 0 {
				return nil, bad("batch_id required for deduction")
			}
			batchID = core.ID()
			_, e = tx.Exec(c, "INSERT INTO batches(id,household_id,ingredient_id,quantity_milli,location,bought_on,expires_on,expiry_kind,source) VALUES($1,$2,$3,0,$4,NULLIF($5,'')::date,NULLIF($6,'')::date,$7,$8)", batchID, hid(c), x.IngredientID, x.Location, x.BoughtOn, x.ExpiresOn, x.ExpiryKind, x.Source)
			if e != nil {
				return nil, e
			}
		}
		if delta > 0 {
			// Serialize replenishment with recording a purchase cost basis.
			var locked string
			if err := tx.QueryRow(c, "SELECT id FROM batches WHERE id=$1 AND household_id=$2 AND ingredient_id=$3 FOR UPDATE", batchID, hid(c), x.IngredientID).Scan(&locked); err != nil {
				return nil, conflict("batch unavailable")
			}
			if x.Reason == "purchase" || x.Reason == "manual" {
				var costRecorded bool
				if err := tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM batch_purchase_costs WHERE batch_id=$1)", batchID).Scan(&costRecorded); err != nil {
					return nil, err
				}
				if costRecorded {
					return nil, conflict("priced batch cannot receive new purchases; create a new batch")
				}
			}
			tag, er := tx.Exec(c, "UPDATE batches SET quantity_milli=quantity_milli+$1 WHERE id=$2 AND household_id=$3 AND ingredient_id=$4", delta, batchID, hid(c), x.IngredientID)
			e = er
			if e == nil && tag.RowsAffected() == 0 {
				return nil, conflict("batch unavailable")
			}
		} else {
			query := "UPDATE batches SET quantity_milli=quantity_milli+$1 WHERE id=$2 AND household_id=$3 AND ingredient_id=$4 AND quantity_milli >= $5"
			if x.Reason == "consume" {
				query += " AND condition='normal' AND (expires_at IS NULL OR expires_at>now())"
			}
			tag, er := tx.Exec(c, query, delta, batchID, hid(c), x.IngredientID, q)
			e = er
			if e == nil && tag.RowsAffected() == 0 {
				return nil, conflict("insufficient stock or batch unavailable")
			}
		}
		if e != nil {
			return nil, e
		}
		_, e = tx.Exec(c, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,note) VALUES($1,$2,$3,$4,$5,$6,$7)", core.ID(), hid(c), batchID, delta, x.Reason, uid(c), x.Note)
		if e == nil && x.Reason == "waste" && x.Note == "spoiled" {
			_, e = tx.Exec(c, "UPDATE batches SET condition='spoiled' WHERE id=$1 AND household_id=$2 AND quantity_milli=0", batchID, hid(c))
		}
		return gin.H{"batch_id": batchID, "quantity_delta": core.Format(delta)}, e
	})
}
func reserve(ctx context.Context, tx pgx.Tx, house, ingredient string, q int64, actor, reason, refType, refID string) error {
	rows, e := tx.Query(ctx, "SELECT id,quantity_milli FROM batches WHERE household_id=$1 AND ingredient_id=$2 AND quantity_milli>0 AND condition='normal' AND (expires_at IS NULL OR expires_at>now()) ORDER BY expires_at NULLS LAST,bought_on NULLS LAST,created_at,id FOR UPDATE", house, ingredient)
	if e != nil {
		return e
	}
	type lot struct {
		id string
		q  int64
	}
	lots := []lot{}
	for rows.Next() {
		var v lot
		if e = rows.Scan(&v.id, &v.q); e != nil {
			rows.Close()
			return e
		}
		lots = append(lots, v)
	}
	rows.Close()
	available := int64(0)
	for _, l := range lots {
		available += l.q
	}
	if available < q {
		return conflict("insufficient stock")
	}
	for _, l := range lots {
		if q == 0 {
			break
		}
		use := q
		if l.q < use {
			use = l.q
		}
		tag, e := tx.Exec(ctx, "UPDATE batches SET quantity_milli=quantity_milli-$1 WHERE id=$2 AND household_id=$3 AND quantity_milli >= $1", use, l.id, house)
		if e != nil {
			return e
		}
		if tag.RowsAffected() != 1 {
			return conflict("stock changed")
		}
		_, e = tx.Exec(ctx, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,ref_type,ref_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", core.ID(), house, l.id, -use, reason, actor, refType, refID)
		if e != nil {
			return e
		}
		q -= use
	}
	return nil
}
func needKey(c *gin.Context) error {
	if len(c.GetHeader("Idempotency-Key")) < 8 {
		return fmt.Errorf("idempotency key required")
	}
	return nil
}
