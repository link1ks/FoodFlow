-- +goose Up
CREATE TABLE business_events (id bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, household_id uuid NOT NULL REFERENCES households(id) ON DELETE CASCADE, kind text NOT NULL, created_at timestamptz NOT NULL DEFAULT now());
CREATE INDEX business_events_household_id ON business_events(household_id,id);
-- +goose StatementBegin
CREATE FUNCTION publish_foodflow_change() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE h uuid;
BEGIN
  IF TG_TABLE_NAME='shopping_items' THEN SELECT household_id INTO h FROM shopping_lists WHERE id=NEW.list_id;
  ELSIF TG_TABLE_NAME='households' THEN h:=NEW.id;
  ELSE h:=NEW.household_id; END IF;
  INSERT INTO business_events(household_id,kind) VALUES(h,TG_TABLE_NAME);
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER event_stock AFTER INSERT ON stock_ledger FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();
CREATE TRIGGER event_ingredient AFTER INSERT OR UPDATE ON ingredients FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();
CREATE TRIGGER event_plan AFTER INSERT OR UPDATE ON plans FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();
CREATE TRIGGER event_shopping AFTER INSERT OR UPDATE ON shopping_items FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();
CREATE TRIGGER event_member AFTER INSERT OR UPDATE ON members FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();
CREATE TRIGGER event_household AFTER INSERT OR UPDATE ON households FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();
-- +goose Down
DROP TRIGGER event_stock ON stock_ledger;
DROP TRIGGER event_ingredient ON ingredients;
DROP TRIGGER event_plan ON plans;
DROP TRIGGER event_shopping ON shopping_items;
DROP TRIGGER event_member ON members;
DROP TRIGGER event_household ON households;
DROP FUNCTION publish_foodflow_change;
DROP TABLE business_events;
