-- +goose Up
ALTER TABLE households ADD COLUMN excluded_ingredients text[] NOT NULL DEFAULT '{}';
ALTER TABLE plans ADD COLUMN excluded_ingredients text[] NOT NULL DEFAULT '{}';
-- +goose Down
ALTER TABLE plans DROP COLUMN excluded_ingredients;
ALTER TABLE households DROP COLUMN excluded_ingredients;
