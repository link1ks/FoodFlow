package app

import (
	"context"
	"foodflow/internal/core"
	"github.com/jackc/pgx/v5"
	"time"
)

func (a *App) pantryReplenishment(ctx context.Context, tx pgx.Tx, scope cookingScope, name, unit string, capacity, quantity int64, threshold int) error {
	if quantity*100 >= capacity*int64(threshold) {
		return nil
	}
	var list string
	err := tx.QueryRow(ctx, `INSERT INTO shopping_lists(id,household_id,source) VALUES($1,$2,'system') ON CONFLICT(household_id) WHERE source='system' DO UPDATE SET source='system' RETURNING id`, core.ID(), scope.HouseholdID).Scan(&list)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO shopping_items(id,list_id,name,unit,needed_milli,origin) VALUES($1,$2,$3,$4,$5,'system') ON CONFLICT(list_id,name,unit) WHERE stocked=false AND deleted_at IS NULL DO UPDATE SET needed_milli=GREATEST(shopping_items.needed_milli,EXCLUDED.needed_milli)`, core.ID(), list, name, unit, capacity)
	return err
}

// Opt-in virtual consumption shares the meal's transaction and physical ledger.
// No separate inventory balance exists; uncertainty is retained in an immutable
// calculation record even when the estimated bottle has already reached zero.
func (a *App) consumeSeasonings(ctx context.Context, tx pgx.Tx, scope cookingScope, mealID string) error {
	rows, err := tx.Query(ctx, `WITH dishes AS(SELECT servings,recipe_id FROM plan_meals WHERE id=$1 UNION ALL SELECT m.servings,d.recipe_id FROM plan_meals m JOIN plan_meal_dishes d ON d.plan_meal_id=m.id WHERE m.id=$1)
 SELECT s.name,s.dose_milli,s.dose_unit,v.per_dose_milli,v.unit,v.source,d.servings,r.servings,r.title FROM dishes d JOIN recipes r ON r.id=d.recipe_id JOIN recipe_seasonings s ON s.recipe_id=r.id JOIN seasoning_conversions v ON v.name=s.name AND v.dose_unit=s.dose_unit
 WHERE NOT EXISTS(SELECT 1 FROM recipe_items ri WHERE ri.recipe_id=r.id AND ri.name=s.name) ORDER BY s.name,r.id`, mealID)
	if err != nil {
		return err
	}
	type need struct {
		name, unit string
		quantity   int64
		basis      []map[string]any
	}
	needs := []need{}
	for rows.Next() {
		var name, doseUnit, unit, source, title string
		var dose, per int64
		var servings, base int
		if err = rows.Scan(&name, &dose, &doseUnit, &per, &unit, &source, &servings, &base, &title); err != nil {
			break
		}
		quantity := (dose*per + 999) / 1000
		quantity, err = core.Scale(quantity, servings, base)
		if err != nil {
			break
		}
		if len(needs) == 0 || needs[len(needs)-1].name != name {
			needs = append(needs, need{name: name, unit: unit})
		}
		n := &needs[len(needs)-1]
		n.quantity += quantity
		n.basis = append(n.basis, map[string]any{"recipe": title, "dose": core.Format(dose), "dose_unit": doseUnit, "per_dose": core.Format(per), "unit": unit, "source": source, "servings": servings, "base_servings": base})
	}
	rows.Close()
	if err != nil {
		return err
	}
	if err = rows.Err(); err != nil {
		return err
	}
	// Lock in stable name order, matching ordinary meal requirement ordering.
	for _, n := range needs {
		var id, batch, unit, condition string
		var capacity, quantity int64
		var threshold int
		var expiry *time.Time
		err = tx.QueryRow(ctx, `SELECT p.id,p.batch_id,i.unit,p.capacity_milli,b.quantity_milli,p.alert_threshold_pct,b.condition,b.expires_at FROM virtual_pantry p JOIN batches b ON b.id=p.batch_id AND b.household_id=p.household_id JOIN ingredients i ON i.id=b.ingredient_id WHERE p.household_id=$1 AND p.name=$2 AND p.active FOR UPDATE OF b,p`, scope.HouseholdID, n.name).Scan(&id, &batch, &unit, &capacity, &quantity, &threshold, &condition, &expiry)
		if err == pgx.ErrNoRows {
			continue
		}
		if err != nil {
			return err
		}
		var forbidden bool
		if err = tx.QueryRow(ctx, `SELECT $2=ANY(h.excluded_ingredients||p.excluded_ingredients) FROM plan_meals m JOIN plans p ON p.id=m.plan_id JOIN households h ON h.id=p.household_id WHERE m.id=$1 AND p.household_id=$3`, mealID, n.name, scope.HouseholdID).Scan(&forbidden); err != nil {
			return err
		}
		if forbidden {
			return conflict(n.name + "属于家庭或本次菜单忌口，请停用该调味品或调整菜单")
		}
		if unit != n.unit {
			return conflict("调味品单位与估算依据不兼容")
		}
		if condition != "normal" || (expiry != nil && !expiry.After(time.Now())) {
			return conflict(n.name + "已过期或报损，请更换或停用该瓶")
		}
		use := min(quantity, n.quantity)
		var ledgerID *string
		if use > 0 {
			l := core.ID()
			ledgerID = &l
			_, err = tx.Exec(ctx, `UPDATE batches SET quantity_milli=quantity_milli-$2 WHERE id=$1 AND quantity_milli>=$2`, batch, use)
			if err != nil {
				return err
			}
			_, err = tx.Exec(ctx, `INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason,actor_id,ref_type,ref_id,note,is_estimated) VALUES($1,$2,$3,$4,'consume',$5,'plan_meal',$6,'调味品配方估算',true)`, l, scope.HouseholdID, batch, -use, scope.ActorID, mealID)
			if err != nil {
				return err
			}
		}
		_, err = tx.Exec(ctx, `INSERT INTO pantry_consumptions(id,household_id,pantry_id,plan_meal_id,ledger_id,requested_milli,deducted_milli,unit,basis) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, core.ID(), scope.HouseholdID, id, mealID, ledgerID, n.quantity, use, n.unit, core.JSON(n.basis))
		if err != nil {
			return err
		}
		if err = a.pantryReplenishment(ctx, tx, scope, n.name, unit, capacity, quantity-use, threshold); err != nil {
			return err
		}
	}
	return nil
}
