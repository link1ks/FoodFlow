-- +goose Up
CREATE TABLE market_benchmark_prices (
 id bigserial PRIMARY KEY,
 city_code varchar(32) NOT NULL,
 city_name varchar(32) NOT NULL,
 ingredient_catalog_id uuid NOT NULL REFERENCES ingredient_catalog(id),
 ingredient_name varchar(64) NOT NULL,
 category varchar(32) NOT NULL,
 price numeric(8,2) NOT NULL CHECK(price>0),
 unit varchar(16) NOT NULL CHECK(unit IN ('500g','1kg','个','1l')),
 source_agency varchar(64) NOT NULL,
 source_url text NOT NULL CHECK(source_url LIKE 'https://%'),
 source_item_name text NOT NULL,
 specification text NOT NULL,
 price_type text NOT NULL CHECK(price_type IN ('retail_monitor','wholesale_monitor','government_guidance')),
 recorded_date date NOT NULL,
 period_start date NOT NULL,
 collected_at timestamptz NOT NULL DEFAULT now(),
 CHECK(period_start<=recorded_date),
 UNIQUE(city_code,ingredient_catalog_id,source_agency,source_item_name,specification,price_type,unit,recorded_date)
);
CREATE INDEX market_benchmark_city_item_date ON market_benchmark_prices(city_name,ingredient_name,recorded_date DESC);

CREATE TABLE community_price_records (
 id bigserial PRIMARY KEY,
 family_id uuid NOT NULL REFERENCES households(id),
 user_id uuid NOT NULL REFERENCES users(id),
 city_name varchar(32) NOT NULL CHECK(length(city_name)>0),
 ingredient_catalog_id uuid NOT NULL REFERENCES ingredient_catalog(id),
 ingredient_name varchar(64) NOT NULL,
 store_name varchar(128) NOT NULL CHECK(length(store_name)>0),
 price numeric(8,2) NOT NULL CHECK(price>0),
 package_quantity numeric(12,3) NOT NULL CHECK(package_quantity>0),
 unit varchar(16) NOT NULL CHECK(unit IN ('g','kg','ml','l','个','只')),
 source_type varchar(32) NOT NULL DEFAULT 'manual' CHECK(source_type IN ('manual','receipt')),
 purchase_date date NOT NULL,
 visibility text NOT NULL DEFAULT 'household' CHECK(visibility IN ('household','community')),
 idempotency_key varchar(120) NOT NULL,
 request_hash text NOT NULL,
 created_at timestamptz NOT NULL DEFAULT now(),
 normalized_unit text GENERATED ALWAYS AS (CASE WHEN unit IN ('g','kg') THEN '500g' WHEN unit IN ('ml','l') THEN '1l' ELSE unit END) STORED,
 normalized_price numeric GENERATED ALWAYS AS (CASE unit WHEN 'g' THEN price*500/package_quantity WHEN 'kg' THEN price/(package_quantity*2) WHEN 'ml' THEN price*1000/package_quantity ELSE price/package_quantity END) STORED,
 UNIQUE(family_id,idempotency_key)
);
CREATE INDEX community_price_family ON community_price_records(family_id,city_name,ingredient_catalog_id,purchase_date DESC);
CREATE INDEX community_price_public ON community_price_records(city_name,ingredient_catalog_id,purchase_date DESC) WHERE visibility='community';

-- +goose Down
DROP TABLE community_price_records;
DROP TABLE market_benchmark_prices;
