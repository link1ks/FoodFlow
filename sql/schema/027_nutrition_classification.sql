-- +goose Up
-- Project classification v1: explicit varieties only; unknown cultivars remain NULL.
-- Reference: https://dghb.dg.gov.cn/zsjg/dzsjbyfkzzx/jkzt/jkxj/qmyyz/content/post_3810699.html
UPDATE ingredient_catalog SET is_dark_vegetable=true WHERE name IN ('菠菜','西兰花','胡萝卜','韭菜','空心菜','紫甘蓝');
UPDATE ingredient_catalog SET is_dark_vegetable=false WHERE name IN ('白萝卜','菜花','大白菜');
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION capture_stock_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
DECLARE batch_household uuid;
BEGIN
 SELECT b.household_id INTO batch_household FROM batches b JOIN ingredients i ON i.id=b.ingredient_id AND i.household_id=b.household_id WHERE b.id=NEW.batch_id;
 IF batch_household IS DISTINCT FROM NEW.household_id THEN RAISE EXCEPTION 'batch household mismatch'; END IF;
 INSERT INTO stock_ledger_snapshots(ledger_id,household_id,ingredient_id,ingredient_name,category,unit,dimension,expires_at,expiry_kind,is_dark_vegetable,nutrition_profile,nutrition_version)
 SELECT NEW.id,NEW.household_id,i.id,i.name,COALESCE(NULLIF(i.category,''),ic.category,''),i.unit,i.dimension,b.expires_at,b.expiry_kind,ic.is_dark_vegetable,np.profile,np.version
 FROM batches b JOIN ingredients i ON i.id=b.ingredient_id
 LEFT JOIN ingredient_catalog ic ON ic.id=i.catalog_id OR (i.catalog_id IS NULL AND ic.name=i.name)
 LEFT JOIN ingredient_nutrition_profiles np ON np.catalog_id=ic.id WHERE b.id=NEW.batch_id;
 RETURN NEW;
END $$;
-- +goose StatementEnd
-- +goose Down
UPDATE ingredient_catalog SET is_dark_vegetable=NULL WHERE name IN ('菠菜','西兰花','胡萝卜','韭菜','空心菜','紫甘蓝','白萝卜','菜花','大白菜');
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION capture_stock_ledger() RETURNS trigger LANGUAGE plpgsql AS $$
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
