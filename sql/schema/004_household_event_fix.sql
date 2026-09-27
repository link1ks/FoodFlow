-- +goose Up
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION publish_foodflow_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE h uuid;
BEGIN
  IF TG_TABLE_NAME='shopping_items' THEN SELECT household_id INTO h FROM shopping_lists WHERE id=NEW.list_id;
  ELSIF TG_TABLE_NAME='households' THEN h:=NEW.id;
  ELSE h:=NEW.household_id; END IF;
  INSERT INTO business_events(household_id,kind) VALUES(h,TG_TABLE_NAME);
  RETURN NEW;
END $$;
-- +goose StatementEnd
-- +goose Down
SELECT 1;
