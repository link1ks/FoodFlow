-- +goose Up
-- A city-only observation lets members use prices when browser location is unavailable.
ALTER TABLE local_price_quotes ADD COLUMN city text NOT NULL DEFAULT '' CHECK(length(city)<=80);
ALTER TABLE local_price_quotes ALTER COLUMN latitude DROP NOT NULL;
ALTER TABLE local_price_quotes ALTER COLUMN longitude DROP NOT NULL;
ALTER TABLE local_price_quotes ADD CONSTRAINT local_price_location CHECK (
  (latitude IS NULL AND longitude IS NULL AND city<>'') OR
  (latitude IS NOT NULL AND longitude IS NOT NULL)
);
CREATE INDEX local_price_quotes_city ON local_price_quotes(household_id,city,observed_at DESC);

-- +goose Down
DROP INDEX local_price_quotes_city;
ALTER TABLE local_price_quotes DROP CONSTRAINT local_price_location;
ALTER TABLE local_price_quotes ALTER COLUMN latitude SET NOT NULL;
ALTER TABLE local_price_quotes ALTER COLUMN longitude SET NOT NULL;
ALTER TABLE local_price_quotes DROP COLUMN city;
