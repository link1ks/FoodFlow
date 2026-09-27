-- +goose Up
ALTER TABLE ingredients ADD COLUMN image_key text;
ALTER TABLE ingredients ADD COLUMN image_mime text;
ALTER TABLE ingredients ADD COLUMN image_version bigint NOT NULL DEFAULT 0;
ALTER TABLE ingredients ADD CONSTRAINT ingredients_image_pair CHECK ((image_key IS NULL) = (image_mime IS NULL));

-- +goose Down
ALTER TABLE ingredients DROP CONSTRAINT ingredients_image_pair;
ALTER TABLE ingredients DROP COLUMN image_version;
ALTER TABLE ingredients DROP COLUMN image_mime;
ALTER TABLE ingredients DROP COLUMN image_key;
