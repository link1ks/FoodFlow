-- name: LockMultiMealProposal :one
SELECT * FROM multi_meal_proposals WHERE id=$1 AND household_id=$2 FOR UPDATE;
