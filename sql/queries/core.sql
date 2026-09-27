-- name: Membership :one
SELECT role FROM members WHERE household_id=$1 AND user_id=$2;
