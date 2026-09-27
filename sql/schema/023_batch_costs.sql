-- +goose Up
CREATE TABLE batch_purchase_costs (
 batch_id uuid PRIMARY KEY REFERENCES batches(id),
 household_id uuid NOT NULL REFERENCES households(id),
 purchase_quantity_milli bigint NOT NULL CHECK(purchase_quantity_milli>0),
 total_cost numeric(12,2) NOT NULL CHECK(total_cost>=0),
 currency text NOT NULL DEFAULT 'CNY' CHECK(currency='CNY'),
 actor_id uuid NOT NULL REFERENCES users(id),
 recorded_at timestamptz NOT NULL DEFAULT now()
);
CREATE TRIGGER batch_cost_immutable BEFORE UPDATE OR DELETE ON batch_purchase_costs FOR EACH ROW EXECUTE FUNCTION prevent_stock_ledger_mutation();
CREATE TABLE stock_cost_snapshots (
 ledger_id uuid PRIMARY KEY REFERENCES stock_ledger(id),
 household_id uuid NOT NULL REFERENCES households(id),
 allocated_cost numeric(20,8) NOT NULL CHECK(allocated_cost>=0),
 currency text NOT NULL CHECK(currency='CNY')
);
CREATE TRIGGER stock_cost_immutable BEFORE UPDATE OR DELETE ON stock_cost_snapshots FOR EACH ROW EXECUTE FUNCTION prevent_stock_ledger_mutation();
-- +goose StatementBegin
CREATE FUNCTION capture_stock_cost() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO stock_cost_snapshots(ledger_id,household_id,allocated_cost,currency)
 SELECT NEW.id,NEW.household_id,abs(NEW.delta_milli::numeric)*c.total_cost/c.purchase_quantity_milli,c.currency
 FROM batch_purchase_costs c WHERE c.batch_id=NEW.batch_id AND c.household_id=NEW.household_id;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER capture_stock_cost AFTER INSERT ON stock_ledger FOR EACH ROW EXECUTE FUNCTION capture_stock_cost();
-- +goose Down
DROP TRIGGER capture_stock_cost ON stock_ledger;
DROP FUNCTION capture_stock_cost;
DROP TABLE stock_cost_snapshots,batch_purchase_costs;
