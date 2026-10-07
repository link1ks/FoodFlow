-- +goose Up
CREATE TABLE recovery_codes (
 user_id uuid NOT NULL REFERENCES users(id),
 code_hash text PRIMARY KEY,
 auth_version bigint NOT NULL,
 expires_at timestamptz NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX recovery_codes_user ON recovery_codes(user_id);
-- One service-wide row serializes allowance and concurrency decisions, including month rollover.
CREATE TABLE ai_allowance_lock (id boolean PRIMARY KEY CHECK(id));
INSERT INTO ai_allowance_lock(id) VALUES(true);
CREATE TABLE ai_call_allowances (
 job_id uuid PRIMARY KEY REFERENCES jobs(id),
 household_id uuid NOT NULL REFERENCES households(id),
 kind text NOT NULL CHECK(kind IN ('text','image')),
 month date NOT NULL,
 ceiling_milli bigint NOT NULL CHECK(ceiling_milli>0),
 state text NOT NULL CHECK(state IN ('pending','completed','uncertain')),
 created_at timestamptz NOT NULL DEFAULT now(),
 finished_at timestamptz
);
CREATE INDEX ai_call_allowances_period ON ai_call_allowances(month,household_id);
