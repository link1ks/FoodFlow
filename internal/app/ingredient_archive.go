package app

import (
	"foodflow/internal/core"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
)

// archiveIngredient keeps batches and their append-only ledger, recording a
// compensating entry for every remaining batch before hiding the ingredient.
func (a *App) archiveIngredient(c *gin.Context) {
	if !writable(c) {
		return
	}
	id := c.Param("ingredient")
	a.idem(c, gin.H{"ingredient_id": id, "operation": "archive"}, func(tx pgx.Tx) (any, error) {
		var archived bool
		if e := tx.QueryRow(c, "SELECT archived_at IS NOT NULL FROM ingredients WHERE id=$1 AND household_id=$2 FOR UPDATE", id, hid(c)).Scan(&archived); e != nil {
			return nil, businessError{404, "ingredient unavailable"}
		}
		if archived {
			return gin.H{"archived": true, "quantity_removed": "0.000"}, nil
		}
		rows, e := tx.Query(c, "SELECT id,quantity_milli FROM batches WHERE household_id=$1 AND ingredient_id=$2 AND quantity_milli>0 ORDER BY id FOR UPDATE", hid(c), id)
		if e != nil {
			return nil, e
		}
		type batch struct {
			id       string
			quantity int64
		}
		batches := []batch{}
		for rows.Next() {
			var b batch
			if e = rows.Scan(&b.id, &b.quantity); e != nil {
				rows.Close()
				return nil, e
			}
			batches = append(batches, b)
		}
		e = rows.Err()
		rows.Close()
		if e != nil {
			return nil, e
		}
		var removed int64
		for _, b := range batches {
			tag, er := tx.Exec(c, "UPDATE batches SET quantity_milli=0 WHERE id=$1 AND household_id=$2 AND quantity_milli=$3", b.id, hid(c), b.quantity)
			if er != nil {
				return nil, er
			}
			if tag.RowsAffected() != 1 {
				return nil, conflict("stock changed")
			}
			if _, er = tx.Exec(c, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,ref_type,ref_id) VALUES($1,$2,$3,$4,'correction',$5,'ingredient_archive',$6)", core.ID(), hid(c), b.id, -b.quantity, uid(c), id); er != nil {
				return nil, er
			}
			removed += b.quantity
		}
		if _, e = tx.Exec(c, "UPDATE ingredients SET archived_at=now() WHERE id=$1 AND household_id=$2", id, hid(c)); e != nil {
			return nil, e
		}
		return gin.H{"archived": true, "quantity_removed": core.Format(removed)}, nil
	})
}
