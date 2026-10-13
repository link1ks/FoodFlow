-- +goose Up
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK(kind IN ('plan','reminder','image','advice','voice'));
ALTER TABLE ai_call_allowances DROP CONSTRAINT ai_call_allowances_kind_check;
ALTER TABLE ai_call_allowances ADD CONSTRAINT ai_call_allowances_kind_check CHECK(kind IN ('text','image','speech'));

-- +goose Down
-- Downgrade deliberately fails if voice jobs or speech allowances exist.
ALTER TABLE ai_call_allowances DROP CONSTRAINT ai_call_allowances_kind_check;
ALTER TABLE ai_call_allowances ADD CONSTRAINT ai_call_allowances_kind_check CHECK(kind IN ('text','image'));
ALTER TABLE jobs DROP CONSTRAINT jobs_kind_check;
ALTER TABLE jobs ADD CONSTRAINT jobs_kind_check CHECK(kind IN ('plan','reminder','image','advice'));
