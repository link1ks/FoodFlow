package app

import (
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"time"
)

// Membership middleware scopes this read. Historical names and units come from
// immutable snapshots; missing historical facts are explicitly left unknown.
func (a *App) ledger(c *gin.Context) {
	rows, err := a.DB.Query(c, `SELECT l.id,l.delta_milli,l.reason,l.created_at,COALESCE(s.ingredient_name,''),COALESCE(s.unit,'') FROM stock_ledger l LEFT JOIN stock_ledger_snapshots s ON s.ledger_id=l.id AND s.household_id=l.household_id WHERE l.household_id=$1 ORDER BY l.created_at DESC,l.id DESC LIMIT 100`, hid(c))
	if err != nil {
		fail(c, 503, "库存流水暂不可用")
		return
	}
	defer rows.Close()
	out := []gin.H{}
	for rows.Next() {
		var id, reason, name, unit string
		var delta int64
		var created time.Time
		if err := rows.Scan(&id, &delta, &reason, &created, &name, &unit); err != nil {
			fail(c, 503, "库存流水读取失败")
			return
		}
		out = append(out, gin.H{"id": id, "quantity": core.Format(delta), "reason": reason, "created_at": created, "ingredient": name, "unit": unit})
	}
	if rows.Err() != nil {
		fail(c, 503, "库存流水读取失败")
		return
	}
	c.JSON(200, out)
}
