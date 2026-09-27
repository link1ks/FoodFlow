-- +goose Up
ALTER TABLE households ADD COLUMN timezone text NOT NULL DEFAULT 'Asia/Shanghai';
UPDATE households h SET timezone=u.timezone FROM users u WHERE u.id=h.owner_id;

ALTER TABLE batches ADD COLUMN expires_at timestamptz;
ALTER TABLE batches ADD COLUMN condition text NOT NULL DEFAULT 'normal' CHECK(condition IN ('normal','spoiled'));
ALTER TABLE stock_ledger ADD COLUMN note text NOT NULL DEFAULT '';
UPDATE batches b SET expires_at=(b.expires_on + 1)::timestamp AT TIME ZONE h.timezone
FROM households h WHERE h.id=b.household_id AND b.expires_on IS NOT NULL;

-- Existing date-only callers continue to mean the end of the recorded local day.
-- A future client may supply an exact expires_at timestamp independently.
-- +goose StatementBegin
CREATE FUNCTION foodflow_batch_expiry() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE tz text;
BEGIN
  IF NEW.expires_at IS NULL AND NEW.expires_on IS NOT NULL THEN
    SELECT timezone INTO tz FROM households WHERE id=NEW.household_id;
    NEW.expires_at := (NEW.expires_on + 1)::timestamp AT TIME ZONE tz;
  END IF;
  RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER batch_expiry_before_insert BEFORE INSERT ON batches
FOR EACH ROW EXECUTE FUNCTION foodflow_batch_expiry();
CREATE INDEX batches_fefo_active ON batches(household_id,ingredient_id,expires_at,created_at)
WHERE quantity_milli>0 AND condition='normal';

CREATE TABLE meal_completions (
  id uuid PRIMARY KEY,
  household_id uuid NOT NULL REFERENCES households(id),
  plan_meal_id uuid NOT NULL UNIQUE REFERENCES plan_meals(id),
  day date NOT NULL,
  meal text NOT NULL CHECK(meal IN ('breakfast','lunch','dinner')),
  actor_id uuid NOT NULL REFERENCES users(id),
  completed_at timestamptz NOT NULL DEFAULT now(),
  UNIQUE(household_id,day,meal)
);

ALTER TABLE shopping_lists ALTER COLUMN plan_id DROP NOT NULL;
ALTER TABLE shopping_lists ADD COLUMN source text NOT NULL DEFAULT 'plan' CHECK(source IN ('plan','system'));
CREATE UNIQUE INDEX shopping_system_list_once ON shopping_lists(household_id) WHERE source='system';
ALTER TABLE shopping_items DROP CONSTRAINT shopping_items_list_id_name_unit_key;
ALTER TABLE shopping_items ADD COLUMN origin text NOT NULL DEFAULT 'plan' CHECK(origin IN ('plan','system'));
ALTER TABLE shopping_items ADD COLUMN source_ingredient_id uuid REFERENCES ingredients(id);
CREATE UNIQUE INDEX shopping_open_item_once ON shopping_items(list_id,name,unit) WHERE stocked=false;

-- +goose Down
DROP INDEX shopping_open_item_once;
ALTER TABLE shopping_items DROP COLUMN source_ingredient_id;
ALTER TABLE shopping_items DROP COLUMN origin;
ALTER TABLE shopping_items ADD CONSTRAINT shopping_items_list_id_name_unit_key UNIQUE(list_id,name,unit);
DROP INDEX shopping_system_list_once;
ALTER TABLE shopping_lists DROP COLUMN source;
ALTER TABLE shopping_lists ALTER COLUMN plan_id SET NOT NULL;
DROP TABLE meal_completions;
DROP INDEX batches_fefo_active;
DROP TRIGGER batch_expiry_before_insert ON batches;
DROP FUNCTION foodflow_batch_expiry;
ALTER TABLE batches DROP COLUMN condition;
ALTER TABLE batches DROP COLUMN expires_at;
ALTER TABLE stock_ledger DROP COLUMN note;
ALTER TABLE households DROP COLUMN timezone;
