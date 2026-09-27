-- +goose Up
ALTER TABLE shopping_items ADD COLUMN deleted_at timestamptz;
DROP INDEX shopping_open_item_once;
CREATE UNIQUE INDEX shopping_open_item_once ON shopping_items(list_id,name,unit) WHERE stocked=false AND deleted_at IS NULL;

-- +goose Down
-- Preserve archived records; rollback fails safely if active-key duplicates exist.
DROP INDEX shopping_open_item_once;
CREATE UNIQUE INDEX shopping_open_item_once ON shopping_items(list_id,name,unit) WHERE stocked=false;
ALTER TABLE shopping_items DROP COLUMN deleted_at;
