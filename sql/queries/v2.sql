-- name: GetMealCompletionSnapshot :one
SELECT s.completion_id,s.servings,s.timezone,s.dishes,s.captured_at
FROM meal_completion_snapshots s
JOIN meal_completions mc ON mc.id=s.completion_id
WHERE s.household_id=$1 AND mc.plan_meal_id=$2;

-- name: LockBatchForCost :one
SELECT id FROM batches WHERE id=$1 AND household_id=$2 FOR UPDATE;

-- name: GetBatchCost :one
SELECT total_cost::text AS total_cost,currency,purchase_quantity_milli,recorded_at
FROM batch_purchase_costs WHERE batch_id=$1 AND household_id=$2;
