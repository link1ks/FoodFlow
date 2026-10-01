-- +goose Up
ALTER TABLE users ADD COLUMN merged_into uuid REFERENCES users(id);
ALTER TABLE users ADD CONSTRAINT users_merge_not_self CHECK(merged_into IS NULL OR merged_into<>id);
ALTER TABLE users DROP CONSTRAINT users_login_identity;
ALTER TABLE users ADD CONSTRAINT users_login_identity CHECK(merged_into IS NOT NULL OR email IS NOT NULL OR phone IS NOT NULL);
ALTER TABLE users ADD COLUMN auth_version bigint NOT NULL DEFAULT 0 CHECK(auth_version>=0);
ALTER TABLE sessions ADD COLUMN auth_version bigint NOT NULL DEFAULT 0 CHECK(auth_version>=0);
ALTER TABLE sms_challenges DROP CONSTRAINT sms_challenges_purpose_check;
ALTER TABLE sms_challenges ADD CONSTRAINT sms_challenges_purpose_check CHECK(purpose IN ('register','login','bind','reset','change_old','merge'));
CREATE TABLE account_merges (
 id uuid PRIMARY KEY,
 source_id uuid NOT NULL UNIQUE REFERENCES users(id),
 target_id uuid NOT NULL REFERENCES users(id),
 merged_at timestamptz NOT NULL DEFAULT now(),
 CHECK(source_id<>target_id)
);
CREATE TRIGGER account_merge_immutable BEFORE UPDATE OR DELETE ON account_merges FOR EACH ROW EXECUTE FUNCTION prevent_stock_ledger_mutation();
-- +goose Down
-- Account identity/membership transfers require an explicit recovery procedure.
-- Do not silently remove the marker or audit evidence during schema rollback.
-- +goose StatementBegin
DO $$ BEGIN RAISE EXCEPTION 'Account lifecycle migration requires explicit reviewed recovery; automatic downgrade is refused'; END $$;
-- +goose StatementEnd
