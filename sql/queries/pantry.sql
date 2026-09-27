-- name: LockPantryBatch :one
SELECT p.id,p.batch_id,p.name,p.capacity_milli,p.alert_threshold_pct,p.active,
 b.quantity_milli,b.condition,b.expires_at,b.ingredient_id,i.unit
FROM virtual_pantry p JOIN batches b ON b.id=p.batch_id AND b.household_id=p.household_id
JOIN ingredients i ON i.id=b.ingredient_id AND i.household_id=b.household_id
WHERE p.id=$1 AND p.household_id=$2 FOR UPDATE OF b,p;
