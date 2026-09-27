-- +goose Up
ALTER TABLE recipe_items ADD COLUMN catalog_id uuid REFERENCES ingredient_catalog(id);
UPDATE recipe_items ri SET catalog_id=c.id FROM ingredient_catalog c WHERE c.name=ri.name;
CREATE INDEX recipe_items_catalog_id ON recipe_items(catalog_id) WHERE catalog_id IS NOT NULL;

-- +goose Down
DROP INDEX recipe_items_catalog_id;
ALTER TABLE recipe_items DROP COLUMN catalog_id;
