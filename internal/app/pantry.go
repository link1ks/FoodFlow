package app

import (
	"encoding/json"
	"foodflow/internal/core"
	"foodflow/internal/dbgen"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"time"
)

func (a *App) pantry(c *gin.Context) {
	var items, options []byte
	err := a.DB.QueryRow(c, `SELECT COALESCE(jsonb_agg(jsonb_build_object('id',p.id,'batch_id',p.batch_id,'name',p.name,'unit',i.unit,'capacity',round(p.capacity_milli/1000.0,3)::text,'quantity',round(b.quantity_milli/1000.0,3)::text,'threshold',p.alert_threshold_pct,'active',p.active,'usable',b.condition='normal' AND (b.expires_at IS NULL OR b.expires_at>now())) ORDER BY p.name),'[]'::jsonb)
 FROM virtual_pantry p JOIN batches b ON b.id=p.batch_id JOIN ingredients i ON i.id=b.ingredient_id WHERE p.household_id=$1 AND p.active`, hid(c)).Scan(&items)
	if err == nil {
		err = a.DB.QueryRow(c, `SELECT COALESCE(jsonb_agg(v ORDER BY v->>'name'),'[]'::jsonb) FROM (SELECT jsonb_build_object('name',name,'unit',unit,'basis',jsonb_agg(source ORDER BY dose_unit)) v FROM seasoning_conversions GROUP BY name,unit) s`).Scan(&options)
	}
	if err != nil {
		fail(c, 500, "调味品读取失败")
		return
	}
	c.JSON(200, gin.H{"items": json.RawMessage(items), "options": json.RawMessage(options)})
}
func (a *App) enablePantry(c *gin.Context) {
	if !writable(c) {
		return
	}
	var req struct {
		BatchID   string `json:"batch_id"`
		Capacity  string `json:"capacity"`
		Threshold int    `json:"threshold"`
		Accept    bool   `json:"accept_estimates"`
	}
	if !input(c, &req) {
		return
	}
	capacity, err := core.Quantity(req.Capacity)
	if err != nil || capacity > 1000000000 || req.Threshold < 1 || req.Threshold > 50 || !req.Accept {
		fail(c, 400, "请确认估算规则，并填写有效容量与 1–50% 补货线")
		return
	}
	a.idem(c, req, func(tx pgx.Tx) (any, error) {
		var name, unit, condition string
		var quantity int64
		var expiry *time.Time
		err := tx.QueryRow(c, `SELECT i.name,i.unit,b.quantity_milli,b.condition,b.expires_at FROM batches b JOIN ingredients i ON i.id=b.ingredient_id AND i.household_id=b.household_id WHERE b.id=$1 AND b.household_id=$2 AND i.archived_at IS NULL FOR UPDATE OF b`, req.BatchID, hid(c)).Scan(&name, &unit, &quantity, &condition, &expiry)
		if err != nil {
			return nil, businessError{404, "批次不可用"}
		}
		if quantity > capacity || condition != "normal" || (expiry != nil && !expiry.After(time.Now())) {
			return nil, conflict("容量不能低于当前余量，且批次须正常未过期")
		}
		var supported, existing bool
		err = tx.QueryRow(c, `SELECT EXISTS(SELECT 1 FROM seasoning_conversions WHERE name=$1 AND unit=$2),EXISTS(SELECT 1 FROM virtual_pantry WHERE (batch_id=$3 AND active) OR (household_id=$4 AND name=$1 AND active))`, name, unit, req.BatchID, hid(c)).Scan(&supported, &existing)
		if err != nil {
			return nil, err
		}
		if !supported {
			return nil, bad("暂不支持该调味品或单位，请使用目录标准名称及 g/ml")
		}
		if existing {
			return nil, conflict("本批次已启用，或同名调味品已有瓶子；请先停用旧瓶")
		}
		id := core.ID()
		err = tx.QueryRow(c, `INSERT INTO virtual_pantry(id,household_id,batch_id,name,capacity_milli,alert_threshold_pct,accepted_by) VALUES($1,$2,$3,$4,$5,$6,$7) ON CONFLICT(batch_id) DO UPDATE SET active=true,capacity_milli=EXCLUDED.capacity_milli,alert_threshold_pct=EXCLUDED.alert_threshold_pct,accepted_by=EXCLUDED.accepted_by,accepted_at=now() RETURNING id`, id, hid(c), req.BatchID, name, capacity, req.Threshold, uid(c)).Scan(&id)
		if err != nil {
			return nil, err
		}
		if err = a.pantryReplenishment(c.Request.Context(), tx, cookingScope{HouseholdID: hid(c), ActorID: uid(c)}, name, unit, capacity, quantity, req.Threshold); err != nil {
			return nil, err
		}
		return gin.H{"id": id}, nil
	})
}
func (a *App) updatePantry(c *gin.Context) {
	if !writable(c) {
		return
	}
	var req struct {
		Action    string `json:"action"`
		Expected  string `json:"expected_quantity"`
		Percent   int    `json:"percent"`
		Threshold int    `json:"threshold"`
	}
	if !input(c, &req) {
		return
	}
	if req.Action != "calibrate" && req.Action != "disable" && req.Action != "threshold" {
		fail(c, 400, "无效操作")
		return
	}
	a.idem(c, req, func(tx pgx.Tx) (any, error) {
		var id, house pgtype.UUID
		if id.Scan(c.Param("pantry")) != nil || house.Scan(hid(c)) != nil {
			return nil, bad("无效编号")
		}
		p, err := dbgen.New(tx).LockPantryBatch(c, dbgen.LockPantryBatchParams{ID: id, HouseholdID: house})
		if err != nil {
			return nil, businessError{404, "调味品不可用"}
		}
		if !p.Active {
			return nil, conflict("已停用")
		}
		switch req.Action {
		case "disable":
			_, err = tx.Exec(c, `UPDATE virtual_pantry SET active=false WHERE id=$1`, id)
		case "threshold":
			if req.Threshold < 1 || req.Threshold > 50 {
				return nil, bad("补货线应为 1–50%")
			}
			_, err = tx.Exec(c, `UPDATE virtual_pantry SET alert_threshold_pct=$2 WHERE id=$1`, id, req.Threshold)
			if err == nil {
				err = a.pantryReplenishment(c.Request.Context(), tx, cookingScope{HouseholdID: hid(c), ActorID: uid(c)}, p.Name, p.Unit, p.CapacityMilli, p.QuantityMilli, req.Threshold)
			}
		case "calibrate":
			if req.Expected != core.Format(p.QuantityMilli) {
				return nil, conflict("余量已变化，请刷新后重新校准")
			}
			if req.Percent < 0 || req.Percent > 100 {
				return nil, bad("余量须在 0–100% 之间")
			}
			target := (p.CapacityMilli*int64(req.Percent) + 50) / 100 // nearest 0.001 unit, half up
			delta := target - p.QuantityMilli
			if delta != 0 {
				_, err = tx.Exec(c, `UPDATE batches SET quantity_milli=$2 WHERE id=$1`, p.BatchID, target)
				if err == nil {
					_, err = tx.Exec(c, `INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,ref_type,ref_id,note,is_estimated) VALUES($1,$2,$3,$4,'correction',$5,'pantry_calibration',$6,'用户滑动校准估计余量',true)`, core.ID(), hid(c), p.BatchID, delta, uid(c), id)
				}
			}
			if err == nil {
				err = a.pantryReplenishment(c.Request.Context(), tx, cookingScope{HouseholdID: hid(c), ActorID: uid(c)}, p.Name, p.Unit, p.CapacityMilli, target, int(p.AlertThresholdPct))
			}
		}
		return gin.H{"status": "ok"}, err
	})
}
