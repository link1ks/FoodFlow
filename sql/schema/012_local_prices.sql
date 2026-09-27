-- +goose Up
-- Member observations are snapshots, not claims of a live retailer feed.
CREATE TABLE local_price_quotes (
  id uuid PRIMARY KEY,
  household_id uuid NOT NULL REFERENCES households(id) ON DELETE CASCADE,
  ingredient_name text NOT NULL CHECK(length(ingredient_name) BETWEEN 1 AND 80),
  store_name text NOT NULL CHECK(length(store_name) BETWEEN 1 AND 120),
  package_quantity_milli bigint NOT NULL CHECK(package_quantity_milli > 0),
  package_unit text NOT NULL,
  price_minor bigint NOT NULL CHECK(price_minor > 0),
  currency text NOT NULL CHECK(currency IN ('CNY','USD','EUR')),
  latitude double precision NOT NULL CHECK(latitude BETWEEN -90 AND 90),
  longitude double precision NOT NULL CHECK(longitude BETWEEN -180 AND 180),
  source text NOT NULL DEFAULT 'member_observation' CHECK(source='member_observation'),
  observed_by uuid NOT NULL REFERENCES users(id),
  observed_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX local_price_quotes_recent ON local_price_quotes(household_id,observed_at DESC);

-- +goose Down
DROP TABLE local_price_quotes;
