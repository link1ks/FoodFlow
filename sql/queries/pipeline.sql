-- name: GetRecipeSteps :many
SELECT rs.*,r.title FROM recipe_steps rs JOIN recipes r ON r.id=rs.recipe_id
WHERE rs.recipe_id=ANY($1::uuid[]) ORDER BY rs.recipe_id,rs.ordinal;

-- name: GetCookingSession :one
SELECT * FROM cooking_sessions WHERE id=$1 AND household_id=$2;
