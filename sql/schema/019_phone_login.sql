-- +goose Up
ALTER TABLE users ALTER COLUMN email DROP NOT NULL;
ALTER TABLE users ADD COLUMN phone text UNIQUE;
ALTER TABLE users ADD CONSTRAINT users_login_identity CHECK (email IS NOT NULL OR phone IS NOT NULL);
ALTER TABLE users ADD CONSTRAINT users_phone_format CHECK (phone IS NULL OR phone ~ '^\+861[3-9][0-9]{9}$');
CREATE TABLE auth_rate_limits (ip_hash text PRIMARY KEY, window_start timestamptz NOT NULL, attempts integer NOT NULL);

-- +goose Down
-- Rollback requires migrating phone-only users to email accounts first.
ALTER TABLE users ALTER COLUMN email SET NOT NULL;
ALTER TABLE users DROP CONSTRAINT users_login_identity;
ALTER TABLE users DROP CONSTRAINT users_phone_format;
ALTER TABLE users DROP COLUMN phone;
DROP TABLE auth_rate_limits;
