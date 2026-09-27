-- +goose Up
-- Historical rows are deliberately not backfilled with today's recipe/profile data.
CREATE TABLE ingredient_nutrition_profiles (
 catalog_id uuid PRIMARY KEY REFERENCES ingredient_catalog(id),
 version text NOT NULL,
 profile jsonb NOT NULL CHECK(jsonb_typeof(profile)='object'),
 updated_at timestamptz NOT NULL DEFAULT now()
);
ALTER TABLE ingredient_catalog ADD COLUMN is_dark_vegetable boolean;

CREATE TABLE meal_completion_snapshots (
 completion_id uuid PRIMARY KEY REFERENCES meal_completions(id),
 household_id uuid NOT NULL REFERENCES households(id),
 servings int NOT NULL CHECK(servings>0),
 timezone text NOT NULL,
 dishes jsonb NOT NULL CHECK(jsonb_typeof(dishes)='array'),
 captured_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX meal_snapshot_household ON meal_completion_snapshots(household_id,captured_at);
CREATE TRIGGER meal_snapshot_immutable BEFORE UPDATE OR DELETE ON meal_completion_snapshots
 FOR EACH ROW EXECUTE FUNCTION prevent_stock_ledger_mutation();

CREATE TABLE stock_ledger_snapshots (
 ledger_id uuid PRIMARY KEY REFERENCES stock_ledger(id),
 household_id uuid NOT NULL REFERENCES households(id),
 ingredient_id uuid NOT NULL REFERENCES ingredients(id),
 ingredient_name text NOT NULL,
 category text NOT NULL,
 unit text NOT NULL,
 dimension text NOT NULL,
 expires_at timestamptz,
 expiry_kind text NOT NULL,
 is_dark_vegetable boolean,
 nutrition_profile jsonb,
 nutrition_version text,
 captured_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX ledger_snapshot_household ON stock_ledger_snapshots(household_id,captured_at);
CREATE INDEX stock_ledger_household_time ON stock_ledger(household_id,created_at,id);
CREATE INDEX meal_completion_household_day ON meal_completions(household_id,day);
CREATE TRIGGER ledger_snapshot_immutable BEFORE UPDATE OR DELETE ON stock_ledger_snapshots
 FOR EACH ROW EXECUTE FUNCTION prevent_stock_ledger_mutation();

-- +goose StatementBegin
CREATE FUNCTION capture_meal_completion() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE meal_household uuid;
BEGIN
 SELECT p.household_id INTO meal_household FROM plan_meals pm JOIN plans p ON p.id=pm.plan_id WHERE pm.id=NEW.plan_meal_id;
 IF meal_household IS DISTINCT FROM NEW.household_id THEN RAISE EXCEPTION 'meal household mismatch'; END IF;
 INSERT INTO meal_completion_snapshots(completion_id,household_id,servings,timezone,dishes)
 SELECT NEW.id,NEW.household_id,pm.servings,h.timezone,
   (SELECT jsonb_agg(jsonb_build_object('recipe_id',r.id,'title',r.title,'base_servings',r.servings,
       'minutes',r.minutes,'steps',r.steps,'items',(SELECT jsonb_agg(to_jsonb(ri) ORDER BY ri.name,ri.unit) FROM recipe_items ri WHERE ri.recipe_id=r.id)) ORDER BY r.id)
    FROM recipes r WHERE r.id=pm.recipe_id OR r.id IN (SELECT recipe_id FROM plan_meal_dishes WHERE plan_meal_id=pm.id))
 FROM plan_meals pm JOIN households h ON h.id=NEW.household_id WHERE pm.id=NEW.plan_meal_id;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER capture_meal_completion AFTER INSERT ON meal_completions FOR EACH ROW EXECUTE FUNCTION capture_meal_completion();

-- +goose StatementBegin
CREATE FUNCTION capture_stock_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE batch_household uuid;
BEGIN
 SELECT b.household_id INTO batch_household FROM batches b JOIN ingredients i ON i.id=b.ingredient_id AND i.household_id=b.household_id WHERE b.id=NEW.batch_id;
 IF batch_household IS DISTINCT FROM NEW.household_id THEN RAISE EXCEPTION 'batch household mismatch'; END IF;
 INSERT INTO stock_ledger_snapshots(ledger_id,household_id,ingredient_id,ingredient_name,category,unit,dimension,expires_at,expiry_kind,is_dark_vegetable,nutrition_profile,nutrition_version)
 SELECT NEW.id,NEW.household_id,i.id,i.name,i.category,i.unit,i.dimension,b.expires_at,b.expiry_kind,ic.is_dark_vegetable,np.profile,np.version
 FROM batches b JOIN ingredients i ON i.id=b.ingredient_id
 LEFT JOIN ingredient_catalog ic ON ic.id=i.catalog_id OR (i.catalog_id IS NULL AND ic.name=i.name)
 LEFT JOIN ingredient_nutrition_profiles np ON np.catalog_id=ic.id WHERE b.id=NEW.batch_id;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER capture_stock_ledger AFTER INSERT ON stock_ledger FOR EACH ROW EXECUTE FUNCTION capture_stock_ledger();

-- +goose Down
DROP TRIGGER capture_stock_ledger ON stock_ledger;
DROP FUNCTION capture_stock_ledger;
DROP TRIGGER capture_meal_completion ON meal_completions;
DROP FUNCTION capture_meal_completion;
DROP TABLE stock_ledger_snapshots,meal_completion_snapshots;
DROP INDEX stock_ledger_household_time;
DROP INDEX meal_completion_household_day;
ALTER TABLE ingredient_catalog DROP COLUMN is_dark_vegetable;
DROP TABLE ingredient_nutrition_profiles;
