package app

import (
	"foodflow/internal/core"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

func quantityOrZero(value string) (int64, error) {
	if value == "0" || value == "0.000" {
		return 0, nil
	}
	return core.Quantity(value)
}

// correctBatchQuantity treats the typed number as an absolute target. The
// expected balance prevents a stale mobile form from overwriting a newer edit.
func (a *App) correctBatchQuantity(c *gin.Context) {
	if !writable(c) {
		return
	}
	var request struct {
		Expected string `json:"expected_quantity"`
		Target   string `json:"target_quantity"`
	}
	if !input(c, &request) {
		return
	}
	expected, err := quantityOrZero(request.Expected)
	if err != nil {
		fail(c, 400, "invalid expected quantity")
		return
	}
	target, err := quantityOrZero(request.Target)
	if err != nil {
		fail(c, 400, "invalid target quantity")
		return
	}
	key := struct {
		Batch string
		Body  any
	}{c.Param("batch"), request}
	a.idem(c, key, func(tx pgx.Tx) (any, error) {
		var current int64
		err := tx.QueryRow(c, `SELECT b.quantity_milli FROM batches b JOIN ingredients i ON i.id=b.ingredient_id
			WHERE b.id=$1 AND b.household_id=$2 AND i.archived_at IS NULL FOR UPDATE OF b`, c.Param("batch"), hid(c)).Scan(&current)
		if err != nil {
			return nil, conflict("batch unavailable")
		}
		if current != expected {
			return nil, conflict("batch changed; refresh and retry")
		}
		if target != current {
			_, err = tx.Exec(c, "UPDATE batches SET quantity_milli=$1,condition='normal' WHERE id=$2", target, c.Param("batch"))
			if err != nil {
				return nil, err
			}
			_, err = tx.Exec(c, `INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,note)
				VALUES($1,$2,$3,$4,'correction',$5,'absolute_balance')`, core.ID(), hid(c), c.Param("batch"), target-current, uid(c))
			if err != nil {
				return nil, err
			}
		}
		return gin.H{"batch_id": c.Param("batch"), "quantity": core.Format(target)}, nil
	})
}
