package app

import (
	"context"
	"errors"
	"foodflow/internal/core"
	"foodflow/internal/dbgen"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
	"sort"
)

// Scope is supplied only after household authorization. Services receive no HTTP objects.
type cookingScope struct{ HouseholdID, ActorID string }
type completeMealRequest struct{ Plan, Meal string }

// The caller owns the transaction and idempotency record. Never commit inside this service.
func (a *App) completeMealTx(ctx context.Context, tx pgx.Tx, scope cookingScope, request completeMealRequest) (any, error) {
	var mealID, planID, householdID pgtype.UUID
	if mealID.Scan(request.Meal) != nil || planID.Scan(request.Plan) != nil || householdID.Scan(scope.HouseholdID) != nil {
		return nil, conflict("meal unavailable")
	}
	locked, err := dbgen.New(tx).LockMealForCooking(ctx, dbgen.LockMealForCookingParams{ID: mealID, PlanID: planID, HouseholdID: householdID})
	if err != nil {
		return nil, conflict("meal unavailable")
	}
	if locked.Status != "confirmed" {
		return nil, conflict("confirmed meal required")
	}
	// The natural key protects against another confirmed plan for the same
	// date and meal, including retries with a different Idempotency-Key.
	completionID := core.ID()
	var inserted string
	err = tx.QueryRow(ctx, `INSERT INTO meal_completions(id,household_id,plan_meal_id,day,meal,actor_id)
			VALUES($1,$2,$3,$4::date,$5,$6)
			ON CONFLICT DO NOTHING RETURNING id`, completionID, scope.HouseholdID, request.Meal, locked.Day, locked.Meal, scope.ActorID).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, conflict("meal already completed")
	}
	if err != nil {
		return nil, err
	}
	needs, err := a.mealNeeds(ctx, tx, request.Meal)
	if err != nil {
		return nil, err
	}
	for _, need := range needs {
		if err = a.consumeNeed(ctx, tx, scope, need, request.Meal); err != nil {
			return nil, err
		}
	}
	if err = a.consumeSeasonings(ctx, tx, scope, request.Meal); err != nil {
		return nil, err
	}
	_, err = tx.Exec(ctx, `UPDATE plans SET status='consumed' WHERE id=$1
			AND (SELECT count(*) FROM plan_meals WHERE plan_id=$1)=
			    (SELECT count(*) FROM meal_completions mc JOIN plan_meals pm ON pm.id=mc.plan_meal_id WHERE pm.plan_id=$1)`, request.Plan)
	if err != nil {
		return nil, err
	}
	return map[string]any{"plan_id": request.Plan, "plan_meal_id": request.Meal, "day": locked.Day, "meal": locked.Meal, "status": "consumed"}, nil
}
func (a *App) mealNeeds(ctx context.Context, tx pgx.Tx, mealID string) ([]demand, error) {
	rows, err := tx.Query(ctx, `WITH dishes AS (
		SELECT pm.servings,pm.recipe_id FROM plan_meals pm WHERE pm.id=$1
		UNION ALL SELECT pm.servings,pmd.recipe_id FROM plan_meals pm
		JOIN plan_meal_dishes pmd ON pmd.plan_meal_id=pm.id WHERE pm.id=$1
	) SELECT ri.name,ri.unit,ri.quantity_milli,d.servings,r.servings
		FROM dishes d JOIN recipes r ON r.id=d.recipe_id JOIN recipe_items ri ON ri.recipe_id=r.id
		ORDER BY ri.name,ri.unit`, mealID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	merged := map[string]*demand{}
	for rows.Next() {
		var name, unit string
		var quantity int64
		var servings, base int
		if err = rows.Scan(&name, &unit, &quantity, &servings, &base); err != nil {
			return nil, err
		}
		scaled, err := core.Scale(quantity, servings, base)
		if err != nil {
			return nil, err
		}
		dim, _, err := core.Dimension(unit)
		if err != nil {
			return nil, err
		}
		canonical := unit
		if dim == "mass" {
			canonical = "g"
		} else if dim == "volume" {
			canonical = "ml"
		}
		scaled, err = core.Convert(scaled, unit, canonical)
		if err != nil {
			return nil, err
		}
		key := name + "\x00" + canonical
		if merged[key] == nil {
			merged[key] = &demand{Name: name, Unit: canonical}
		}
		merged[key].Needed += scaled
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	out := make([]demand, 0, len(merged))
	for _, need := range merged {
		out = append(out, *need)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Name != out[j].Name {
			return out[i].Name < out[j].Name
		}
		return out[i].Unit < out[j].Unit
	})
	return out, nil
}

func (a *App) consumeNeed(ctx context.Context, tx pgx.Tx, scope cookingScope, need demand, mealID string) error {
	rows, err := tx.Query(ctx, `SELECT i.id,i.unit,COALESCE(sum(b.quantity_milli) FILTER
		(WHERE b.condition='normal' AND (b.expires_at IS NULL OR b.expires_at>now())),0)
		FROM ingredients i LEFT JOIN batches b ON b.ingredient_id=i.id AND b.household_id=i.household_id
		WHERE i.household_id=$1 AND i.name=$2 AND i.archived_at IS NULL
		GROUP BY i.id ORDER BY i.unit,i.id`, scope.HouseholdID, need.Name)
	if err != nil {
		return err
	}
	type source struct {
		id, unit  string
		available int64
	}
	var sources []source
	for rows.Next() {
		var s source
		if err = rows.Scan(&s.id, &s.unit, &s.available); err != nil {
			rows.Close()
			return err
		}
		sources = append(sources, s)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	remaining := need.Needed
	for _, s := range sources {
		if remaining == 0 {
			break
		}
		available, err := core.Convert(s.available, s.unit, need.Unit)
		if err != nil {
			if s.available > 0 {
				return conflict("unit conversion requires confirmation: " + need.Name)
			}
			continue
		}
		use := min(remaining, available)
		if use <= 0 {
			continue
		}
		original, err := core.Convert(use, need.Unit, s.unit)
		if err != nil {
			return conflict("unit conversion requires confirmation: " + need.Name)
		}
		if err = reserve(ctx, tx, scope.HouseholdID, s.id, original, scope.ActorID, "consume", "plan_meal", mealID); err != nil {
			return err
		}
		if err = a.suggestReplenishment(ctx, tx, scope, s.id); err != nil {
			return err
		}
		remaining -= use
	}
	if remaining > 0 {
		return conflict("insufficient stock: " + need.Name)
	}
	return nil
}

func (a *App) suggestReplenishment(ctx context.Context, tx pgx.Tx, scope cookingScope, ingredientID string) error {
	var name, unit string
	var low, available int64
	err := tx.QueryRow(ctx, `SELECT i.name,i.unit,i.low_milli,COALESCE(sum(b.quantity_milli) FILTER
		(WHERE b.condition='normal' AND (b.expires_at IS NULL OR b.expires_at>now())),0)
		FROM ingredients i LEFT JOIN batches b ON b.ingredient_id=i.id AND b.household_id=i.household_id
		WHERE i.id=$1 AND i.household_id=$2 GROUP BY i.id`, ingredientID, scope.HouseholdID).Scan(&name, &unit, &low, &available)
	if err != nil || low == 0 || available >= low {
		return err
	}
	listID := core.ID()
	err = tx.QueryRow(ctx, `INSERT INTO shopping_lists(id,household_id,source) VALUES($1,$2,'system')
		ON CONFLICT (household_id) WHERE source='system'
		DO UPDATE SET source='system' RETURNING id`, listID, scope.HouseholdID).Scan(&listID)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO shopping_items(id,list_id,name,unit,needed_milli,origin,source_ingredient_id)
		VALUES($1,$2,$3,$4,$5,'system',$6)
		ON CONFLICT (list_id,name,unit) WHERE stocked=false AND deleted_at IS NULL
		DO UPDATE SET needed_milli=EXCLUDED.needed_milli`, core.ID(), listID, name, unit, low-available, ingredientID)
	return err
}
