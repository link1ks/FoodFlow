-- +goose Up
CREATE TABLE multi_meal_proposals (
 id uuid PRIMARY KEY,
 household_id uuid NOT NULL REFERENCES households(id),
 created_by uuid NOT NULL REFERENCES users(id),
 request jsonb NOT NULL,
 result jsonb NOT NULL,
 snapshot_hash text NOT NULL,
 status text NOT NULL DEFAULT 'pending' CHECK(status IN ('pending','adopted','cancelled')),
 plan_id uuid REFERENCES plans(id),
 created_at timestamptz NOT NULL DEFAULT now(),
 expires_at timestamptz NOT NULL DEFAULT now()+interval '30 minutes'
);
CREATE INDEX multi_meal_household ON multi_meal_proposals(household_id,created_at DESC);
CREATE TRIGGER event_multi_meal AFTER INSERT OR UPDATE ON multi_meal_proposals FOR EACH ROW EXECUTE FUNCTION publish_foodflow_change();
-- +goose Down
DROP TABLE multi_meal_proposals;
