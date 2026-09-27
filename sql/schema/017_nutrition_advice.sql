-- +goose Up
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK(kind IN ('plan','reminder','image','advice'));
ALTER TABLE jobs ADD COLUMN request_key varchar(120);
ALTER TABLE jobs ADD COLUMN request_hash text;
CREATE UNIQUE INDEX jobs_request_key ON jobs(household_id,created_by,request_key) WHERE request_key IS NOT NULL;

-- +goose Down
-- Preserve advice history: rollback fails if advice rows have not been archived.
DROP INDEX jobs_request_key;
ALTER TABLE jobs DROP COLUMN request_key, DROP COLUMN request_hash;
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK(kind IN ('plan','reminder','image'));
