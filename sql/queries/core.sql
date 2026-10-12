-- name: Membership :one
SELECT m.role FROM members m JOIN households h ON h.id=m.household_id
WHERE m.household_id=$1 AND m.user_id=$2 AND (h.kind='family' OR h.owner_id=m.user_id);
