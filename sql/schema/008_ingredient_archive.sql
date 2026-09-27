-- +goose Up
ALTER TABLE ingredients ADD COLUMN archived_at timestamptz;
CREATE INDEX ingredients_active_household ON ingredients(household_id,name) WHERE archived_at IS NULL;

-- +goose Down
DROP INDEX ingredients_active_household;
ALTER TABLE ingredients DROP COLUMN archived_at;
