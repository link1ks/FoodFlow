package app

import (
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"time"
)

func (a *App) batchNutrition(c *gin.Context) {
	var exists bool
	if err := a.DB.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM batches WHERE id::text=$1 AND household_id=$2)", c.Param("batch"), hid(c)).Scan(&exists); err != nil {
		fail(c, 503, "营养换算暂不可用")
		return
	}
	if !exists {
		fail(c, 404, "批次不可访问")
		return
	}
	var revision int
	var quantity, grams int64
	var classification string
	var at time.Time
	err := a.DB.QueryRow(c, "SELECT revision,quantity_milli,edible_grams_milli,classification,confirmed_at FROM batch_nutrition_confirmations WHERE batch_id::text=$1 AND household_id=$2 ORDER BY revision DESC LIMIT 1", c.Param("batch"), hid(c)).Scan(&revision, &quantity, &grams, &classification, &at)
	if err == pgx.ErrNoRows {
		c.JSON(200, gin.H{"recorded": false, "revision": 0})
		return
	}
	if err != nil {
		fail(c, 503, "营养换算暂不可用")
		return
	}
	c.JSON(200, gin.H{"recorded": true, "revision": revision, "quantity": core.Format(quantity), "edible_grams": core.Format(grams), "classification": classification, "confirmed_at": at})
}

func (a *App) confirmBatchNutrition(c *gin.Context) {
	if !writable(c) {
		return
	}
	var in struct {
		Quantity       string `json:"quantity"`
		Grams          string `json:"edible_grams"`
		Classification string `json:"classification"`
		Revision       int    `json:"expected_revision"`
		Confirm        bool   `json:"confirm"`
	}
	if !input(c, &in) {
		return
	}
	quantity, err := core.Quantity(in.Quantity)
	grams, gramErr := core.Quantity(in.Grams)
	if err != nil || gramErr != nil || quantity <= 0 || grams <= 0 || in.Revision < 0 || !in.Confirm {
		fail(c, 400, "须确认称重依据并提供正数量、可食克重与当前版本")
		return
	}
	if in.Classification != "catalog" && in.Classification != "dark" && in.Classification != "other" && in.Classification != "unknown" {
		fail(c, 400, "蔬菜分类无效")
		return
	}
	a.idem(c, in, func(tx pgx.Tx) (any, error) {
		var unit string
		if err := tx.QueryRow(c, "SELECT i.unit FROM batches b JOIN ingredients i ON i.id=b.ingredient_id AND i.household_id=b.household_id WHERE b.id::text=$1 AND b.household_id=$2 AND b.quantity_milli>0 FOR UPDATE OF b", c.Param("batch"), hid(c)).Scan(&unit); err != nil {
			return nil, businessError{404, "批次不可访问或已用完"}
		}
		// For a weighed stock unit, edible mass cannot exceed the sample's gross mass.
		if unit == "g" && grams > quantity || unit == "kg" && (grams/1000 > quantity || grams/1000 == quantity && grams%1000 > 0) {
			return nil, businessError{400, "可食重量不能超过样本原料重量"}
		}
		var revision int
		if err := tx.QueryRow(c, "SELECT COALESCE(max(revision),0) FROM batch_nutrition_confirmations WHERE batch_id::text=$1 AND household_id=$2", c.Param("batch"), hid(c)).Scan(&revision); err != nil {
			return nil, err
		}
		if revision != in.Revision {
			return nil, conflict("换算依据已更新，请刷新后重新确认")
		}
		_, err := tx.Exec(c, "INSERT INTO batch_nutrition_confirmations(id,household_id,batch_id,revision,quantity_milli,edible_grams_milli,classification,actor_id) VALUES($1,$2,$3,$4,$5,$6,$7,$8)", core.ID(), hid(c), c.Param("batch"), revision+1, quantity, grams, in.Classification, uid(c))
		return gin.H{"batch_id": c.Param("batch"), "revision": revision + 1, "historical_recalculated": false}, err
	})
}
