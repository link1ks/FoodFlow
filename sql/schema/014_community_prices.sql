-- +goose Up
-- Observations remain private to a household unless the reporter explicitly shares one.
ALTER TABLE local_price_quotes ADD COLUMN visibility text NOT NULL DEFAULT 'household'
  CHECK(visibility IN ('household','community'));
CREATE INDEX local_price_quotes_community_recent ON local_price_quotes(observed_at DESC)
  WHERE visibility='community';

-- +goose Down
DROP INDEX local_price_quotes_community_recent;
ALTER TABLE local_price_quotes DROP COLUMN visibility;
