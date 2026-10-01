-- +goose Up
-- User-weighed edible mass is batch-specific; no historical backfill.
CREATE TABLE batch_nutrition_confirmations (
 id uuid PRIMARY KEY,
 household_id uuid NOT NULL REFERENCES households(id),
 batch_id uuid NOT NULL REFERENCES batches(id),
 revision integer NOT NULL CHECK(revision>0),
 quantity_milli bigint NOT NULL CHECK(quantity_milli>0),
 edible_grams_milli bigint NOT NULL CHECK(edible_grams_milli>0),
 classification text NOT NULL CHECK(classification IN ('catalog','dark','other','unknown')),
 actor_id uuid NOT NULL REFERENCES users(id),
 confirmed_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(batch_id,revision)
);
CREATE TRIGGER batch_nutrition_immutable BEFORE UPDATE OR DELETE ON batch_nutrition_confirmations FOR EACH ROW EXECUTE FUNCTION prevent_stock_ledger_mutation();
ALTER TABLE stock_ledger_snapshots ADD COLUMN nutrition_confirmation_id uuid REFERENCES batch_nutrition_confirmations(id);
ALTER TABLE stock_ledger_snapshots ADD COLUMN confirmed_quantity_milli bigint;
ALTER TABLE stock_ledger_snapshots ADD COLUMN confirmed_edible_grams_milli bigint;
ALTER TABLE stock_ledger_snapshots ADD COLUMN classification_source text;
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION capture_stock_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE batch_household uuid;
BEGIN
 SELECT b.household_id INTO batch_household FROM batches b JOIN ingredients i ON i.id=b.ingredient_id AND i.household_id=b.household_id WHERE b.id=NEW.batch_id;
 IF batch_household IS DISTINCT FROM NEW.household_id THEN RAISE EXCEPTION 'batch household mismatch'; END IF;
 INSERT INTO stock_ledger_snapshots(ledger_id,household_id,ingredient_id,ingredient_name,category,unit,dimension,expires_at,expiry_kind,is_dark_vegetable,nutrition_profile,nutrition_version,nutrition_confirmation_id,confirmed_quantity_milli,confirmed_edible_grams_milli,classification_source)
 SELECT NEW.id,NEW.household_id,i.id,i.name,COALESCE(NULLIF(i.category,''),ic.category,''),i.unit,i.dimension,b.expires_at,b.expiry_kind,
 CASE nc.classification WHEN 'dark' THEN true WHEN 'other' THEN false WHEN 'unknown' THEN NULL ELSE ic.is_dark_vegetable END,
 np.profile,np.version,nc.id,nc.quantity_milli,nc.edible_grams_milli,
 CASE WHEN nc.classification IN ('dark','other','unknown') THEN 'household-confirmed' ELSE 'catalog-v1' END
 FROM batches b JOIN ingredients i ON i.id=b.ingredient_id
 LEFT JOIN ingredient_catalog ic ON ic.id=i.catalog_id OR (i.catalog_id IS NULL AND ic.name=i.name)
 LEFT JOIN ingredient_nutrition_profiles np ON np.catalog_id=ic.id
 LEFT JOIN LATERAL (SELECT * FROM batch_nutrition_confirmations n WHERE n.batch_id=b.id AND n.household_id=b.household_id ORDER BY revision DESC LIMIT 1) nc ON true
 WHERE b.id=NEW.batch_id;
 RETURN NEW;
END $$;
-- +goose StatementEnd
-- +goose Down
-- Removing confirmed historical bases would change prior reports.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Confirmed nutrition history requires reviewed recovery; automatic downgrade is refused'; END $$;
-- +goose StatementEnd
