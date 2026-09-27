-- +goose Up
ALTER TABLE stock_ledger ADD COLUMN is_estimated boolean NOT NULL DEFAULT false;
CREATE TABLE seasoning_conversions (
 name text NOT NULL,
 dose_unit text NOT NULL CHECK(dose_unit IN ('tbsp','tsp','pinch','g','ml')),
 unit text NOT NULL CHECK(unit IN ('g','ml')),
 per_dose_milli bigint NOT NULL CHECK(per_dose_milli BETWEEN 1 AND 100000),
 source text NOT NULL,
 PRIMARY KEY(name,dose_unit)
);
INSERT INTO seasoning_conversions VALUES
 ('食用油','tbsp','ml',15000,'项目示例估算：1汤匙约15ml'),
 ('食用油','tsp','ml',5000,'项目示例估算：1茶匙约5ml'),
 ('酱油','tbsp','ml',15000,'项目示例估算：1汤匙约15ml'),
 ('酱油','tsp','ml',5000,'项目示例估算：1茶匙约5ml'),
 ('食盐','pinch','g',1000,'项目示例估算：食盐少许约1g，实际用量因人而异');
CREATE TABLE recipe_seasonings (
 recipe_id uuid NOT NULL REFERENCES recipes(id),
 name text NOT NULL,
 dose_unit text NOT NULL,
 dose_milli bigint NOT NULL CHECK(dose_milli BETWEEN 1 AND 100000),
 is_seasoning boolean NOT NULL DEFAULT true CHECK(is_seasoning),
 PRIMARY KEY(recipe_id,name),
 FOREIGN KEY(name,dose_unit) REFERENCES seasoning_conversions(name,dose_unit)
);
INSERT INTO recipe_seasonings(recipe_id,name,dose_unit,dose_milli)
 SELECT id,'食用油','tbsp',1000 FROM recipes WHERE id IN ('10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000004');
INSERT INTO recipe_seasonings(recipe_id,name,dose_unit,dose_milli)
 SELECT id,'食盐','pinch',1000 FROM recipes WHERE id IN ('10000000-0000-4000-8000-000000000001','10000000-0000-4000-8000-000000000002','10000000-0000-4000-8000-000000000003','10000000-0000-4000-8000-000000000004');
INSERT INTO recipe_seasonings(recipe_id,name,dose_unit,dose_milli) VALUES('10000000-0000-4000-8000-000000000003','酱油','tsp',1000);
CREATE TABLE virtual_pantry (
 id uuid PRIMARY KEY,
 household_id uuid NOT NULL REFERENCES households(id),
 batch_id uuid NOT NULL UNIQUE REFERENCES batches(id),
 name text NOT NULL,
 capacity_milli bigint NOT NULL CHECK(capacity_milli BETWEEN 1 AND 1000000000),
 alert_threshold_pct int NOT NULL DEFAULT 15 CHECK(alert_threshold_pct BETWEEN 1 AND 50),
 active boolean NOT NULL DEFAULT true,
 accepted_by uuid NOT NULL REFERENCES users(id),
 accepted_at timestamptz NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX virtual_pantry_active_name ON virtual_pantry(household_id,name) WHERE active;
CREATE TABLE pantry_consumptions (
 id uuid PRIMARY KEY,
 household_id uuid NOT NULL REFERENCES households(id),
 pantry_id uuid NOT NULL REFERENCES virtual_pantry(id),
 plan_meal_id uuid NOT NULL REFERENCES plan_meals(id),
 ledger_id uuid REFERENCES stock_ledger(id),
 requested_milli bigint NOT NULL CHECK(requested_milli>0),
 deducted_milli bigint NOT NULL CHECK(deducted_milli>=0 AND deducted_milli<=requested_milli),
 unit text NOT NULL,
 basis jsonb NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(pantry_id,plan_meal_id)
);
CREATE TRIGGER pantry_consumption_immutable BEFORE UPDATE OR DELETE ON pantry_consumptions FOR EACH ROW EXECUTE FUNCTION prevent_stock_ledger_mutation();
CREATE TRIGGER event_pantry AFTER INSERT OR UPDATE ON virtual_pantry FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();
-- +goose StatementBegin
CREATE FUNCTION enforce_pantry_capacity() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 IF EXISTS(SELECT 1 FROM virtual_pantry WHERE batch_id=NEW.id AND active AND capacity_milli<NEW.quantity_milli) THEN
  RAISE EXCEPTION 'quantity exceeds active pantry capacity';
 END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER pantry_capacity BEFORE UPDATE OF quantity_milli ON batches FOR EACH ROW EXECUTE FUNCTION enforce_pantry_capacity();
-- +goose Down
DROP TRIGGER pantry_capacity ON batches;
DROP FUNCTION enforce_pantry_capacity();
DROP TABLE pantry_consumptions,virtual_pantry,recipe_seasonings,seasoning_conversions;
ALTER TABLE stock_ledger DROP COLUMN is_estimated;
