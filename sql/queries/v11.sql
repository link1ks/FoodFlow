-- name: LockMealForCooking :one
SELECT p.status, pm.day::text AS day, pm.meal
FROM plan_meals pm JOIN plans p ON p.id=pm.plan_id
WHERE pm.id=$1 AND pm.plan_id=$2 AND p.household_id=$3
FOR UPDATE OF p;
