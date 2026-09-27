-- +goose Up
ALTER TABLE users ADD COLUMN phone_verified boolean NOT NULL DEFAULT false;
CREATE TABLE sms_challenges (
 id uuid PRIMARY KEY, phone text NOT NULL, purpose text NOT NULL CHECK(purpose IN ('register','login','bind')),
 actor text NOT NULL DEFAULT '', code_hash text NOT NULL, state text NOT NULL CHECK(state IN ('pending','sent','failed')),
 expires_at timestamptz NOT NULL, attempts integer NOT NULL DEFAULT 0, used_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX sms_phone_recent ON sms_challenges(phone,created_at DESC);
CREATE TABLE sms_send_limits (phone text PRIMARY KEY,last_sent timestamptz NOT NULL,day date NOT NULL,count integer NOT NULL);
-- +goose Down
DROP TABLE sms_send_limits,sms_challenges;
ALTER TABLE users DROP COLUMN phone_verified;
