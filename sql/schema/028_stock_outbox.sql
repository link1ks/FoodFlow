-- +goose Up
CREATE TABLE stock_outbox (
 event_id uuid PRIMARY KEY REFERENCES stock_ledger(id), household_id uuid NOT NULL REFERENCES households(id),
 payload jsonb NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), published_at timestamptz,
 attempts integer NOT NULL DEFAULT 0, retry_at timestamptz NOT NULL DEFAULT now(), last_error text
);
CREATE INDEX stock_outbox_pending ON stock_outbox(retry_at,created_at) WHERE published_at IS NULL;
-- +goose StatementBegin
CREATE FUNCTION enqueue_stock_event() RETURNS trigger LANGUAGE plpgsql AS $$
BEGIN
 INSERT INTO stock_outbox(event_id,household_id,payload)
 SELECT NEW.id,NEW.household_id,jsonb_build_object(
 'version',1,'id',NEW.id,'household_id',NEW.household_id,
 'ingredient',s.ingredient_name,'category',s.category,'unit',s.unit,
 'delta_milli',NEW.delta_milli::text,'reason',NEW.reason,'occurred_at',NEW.created_at)
 FROM stock_ledger_snapshots s WHERE s.ledger_id=NEW.id;
 IF NOT FOUND THEN RAISE EXCEPTION 'ledger snapshot missing for event'; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
-- Same-kind triggers run in name order, after capture_stock_ledger.
CREATE TRIGGER zz_enqueue_stock_event AFTER INSERT ON stock_ledger FOR EACH ROW EXECUTE FUNCTION enqueue_stock_event();
INSERT INTO stock_outbox(event_id,household_id,payload)
SELECT l.id,l.household_id,jsonb_build_object('version',1,'id',l.id,'household_id',l.household_id,
 'ingredient',s.ingredient_name,'category',s.category,'unit',s.unit,'delta_milli',l.delta_milli::text,
 'reason',l.reason,'occurred_at',l.created_at)
FROM stock_ledger l JOIN stock_ledger_snapshots s ON s.ledger_id=l.id;
-- +goose Down
DROP TRIGGER zz_enqueue_stock_event ON stock_ledger;
DROP FUNCTION enqueue_stock_event;
DROP TABLE stock_outbox;
