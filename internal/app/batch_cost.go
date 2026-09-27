package app

import (
	"encoding/json"
	"errors"
	"foodflow/internal/core"
	"foodflow/internal/dbgen"
	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"regexp"
)

func (a *App) batchCost(c *gin.Context) {
	var batch, house pgtype.UUID
	if batch.Scan(c.Param("batch")) != nil || house.Scan(hid(c)) != nil {
		fail(c, 404, "batch unavailable")
		return
	}
	var exists bool
	if err := a.DB.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM batches WHERE id=$1 AND household_id=$2)", batch, house).Scan(&exists); err != nil {
		fail(c, 500, "cost unavailable")
		return
	}
	if !exists {
		fail(c, 404, "batch unavailable")
		return
	}
	cost, err := dbgen.New(a.DB).GetBatchCost(c, dbgen.GetBatchCostParams{BatchID: batch, HouseholdID: house})
	if errors.Is(err, pgx.ErrNoRows) {
		c.JSON(200, gin.H{"recorded": false})
		return
	}
	if err != nil {
		fail(c, 500, "cost unavailable")
		return
	}
	c.JSON(200, gin.H{"recorded": true, "total_cost": cost.TotalCost, "currency": cost.Currency, "purchase_quantity": core.Format(cost.PurchaseQuantityMilli), "recorded_at": cost.RecordedAt.Time})
}

func (a *App) mealRecord(c *gin.Context) {
	var meal, house pgtype.UUID
	if meal.Scan(c.Param("meal")) != nil || house.Scan(hid(c)) != nil {
		fail(c, 404, "record unavailable")
		return
	}
	snapshot, err := dbgen.New(a.DB).GetMealCompletionSnapshot(c, dbgen.GetMealCompletionSnapshotParams{HouseholdID: house, PlanMealID: meal})
	if errors.Is(err, pgx.ErrNoRows) {
		fail(c, 404, "record unavailable or predates snapshot support")
		return
	}
	if err != nil {
		fail(c, 500, "record unavailable")
		return
	}
	c.JSON(200, gin.H{"servings": snapshot.Servings, "timezone": snapshot.Timezone, "dishes": json.RawMessage(snapshot.Dishes), "captured_at": snapshot.CapturedAt.Time})
}

var costPattern = regexp.MustCompile(`^(0|[1-9][0-9]{0,9})(\.[0-9]{1,2})?$`)

// A cost basis is an immutable user-confirmed purchase fact. Missing costs
// remain unknown; official market prices are never substituted for receipts.
func (a *App) recordBatchCost(c *gin.Context) {
	if !writable(c) {
		return
	}
	var inputValue struct {
		TotalCost string `json:"total_cost"`
	}
	if !input(c, &inputValue) {
		return
	}
	if !costPattern.MatchString(inputValue.TotalCost) {
		fail(c, 400, "total_cost must be a nonnegative CNY amount with at most two decimal places")
		return
	}
	a.idem(c, inputValue, func(tx pgx.Tx) (any, error) {
		var batchID, houseID pgtype.UUID
		if batchID.Scan(c.Param("batch")) != nil || houseID.Scan(hid(c)) != nil {
			return nil, businessError{404, "batch unavailable"}
		}
		batch, err := dbgen.New(tx).LockBatchForCost(c, dbgen.LockBatchForCostParams{ID: batchID, HouseholdID: houseID})
		if err != nil {
			return nil, businessError{404, "batch unavailable"}
		}
		var exists bool
		if err := tx.QueryRow(c, "SELECT EXISTS(SELECT 1 FROM batch_purchase_costs WHERE batch_id=$1)", batch).Scan(&exists); err != nil {
			return nil, err
		}
		if exists {
			return nil, conflict("purchase cost already recorded")
		}
		var quantity int64
		if err := tx.QueryRow(c, "SELECT COALESCE(sum(delta_milli),0)::bigint FROM stock_ledger WHERE batch_id=$1 AND household_id=$2 AND delta_milli>0 AND reason IN ('purchase','manual')", batch, hid(c)).Scan(&quantity); err != nil {
			return nil, err
		}
		if quantity <= 0 {
			return nil, conflict("batch has no purchase or initial stock quantity")
		}
		_, err = tx.Exec(c, "INSERT INTO batch_purchase_costs(batch_id,household_id,purchase_quantity_milli,total_cost,actor_id) VALUES($1,$2,$3,$4::numeric,$5)", batch, hid(c), quantity, inputValue.TotalCost, uid(c))
		return gin.H{"batch_id": c.Param("batch"), "total_cost": inputValue.TotalCost, "currency": "CNY", "historical_outbound_repriced": false}, err
	})
}
