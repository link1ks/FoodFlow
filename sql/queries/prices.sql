-- name: GetLatestBenchmarkPrice :many
SELECT DISTINCT ON (source_agency,source_item_name,specification,price_type,unit) b.*,
 COALESCE((SELECT p.price::text FROM market_benchmark_prices p
 WHERE p.city_code=b.city_code AND p.ingredient_catalog_id=b.ingredient_catalog_id
 AND p.source_agency=b.source_agency AND p.source_item_name=b.source_item_name
 AND p.specification=b.specification AND p.price_type=b.price_type AND p.unit=b.unit
 AND p.recorded_date=b.recorded_date-7 AND p.period_start=b.period_start-7 LIMIT 1),'')::text AS previous_price
FROM market_benchmark_prices b WHERE b.city_name=$1 AND b.ingredient_name=$2
ORDER BY source_agency,source_item_name,specification,price_type,unit,recorded_date DESC;

-- name: ListBenchmarkPricesByCity :many
SELECT DISTINCT ON (ingredient_name,source_agency,source_item_name,specification,price_type,unit) *
FROM market_benchmark_prices WHERE city_name=$1 AND ($2::text='' OR category=$2)
ORDER BY ingredient_name,source_agency,source_item_name,specification,price_type,unit,recorded_date DESC;

-- name: FindPriceIngredient :one
SELECT id,name FROM ingredient_catalog WHERE name=$1 OR $1=ANY(aliases) ORDER BY (name=$1) DESC,name LIMIT 1;

-- name: UpsertOfficialPrice :execrows
INSERT INTO market_benchmark_prices(city_code,city_name,ingredient_catalog_id,ingredient_name,category,price,unit,source_agency,source_url,source_item_name,specification,price_type,recorded_date,period_start)
SELECT '110100','北京市',id,name,category,sqlc.arg(price)::numeric,'500g','北京市价格监测中心','https://www.beijingprice.cn/mrjg/',sqlc.arg(source_item)::text,sqlc.arg(specification)::text,'retail_monitor',sqlc.arg(day)::date,sqlc.arg(day)::date
FROM ingredient_catalog WHERE name=sqlc.arg(ingredient)::text
ON CONFLICT(city_code,ingredient_catalog_id,source_agency,source_item_name,specification,price_type,unit,recorded_date)
DO UPDATE SET price=EXCLUDED.price,collected_at=now();

-- name: UpsertRegionalPrice :execrows
INSERT INTO market_benchmark_prices(city_code,city_name,ingredient_catalog_id,ingredient_name,category,price,unit,source_agency,source_url,source_item_name,specification,price_type,recorded_date,period_start)
SELECT sqlc.arg(city_code)::text,sqlc.arg(city)::text,id,name,category,sqlc.arg(price)::numeric,sqlc.arg(unit)::text,sqlc.arg(agency)::text,sqlc.arg(source_url)::text,sqlc.arg(original)::text,sqlc.arg(specification)::text,'retail_monitor',sqlc.arg(day)::date,sqlc.arg(period_start)::date
FROM ingredient_catalog WHERE name=sqlc.arg(ingredient)::text
ON CONFLICT(city_code,ingredient_catalog_id,source_agency,source_item_name,specification,price_type,unit,recorded_date)
DO UPDATE SET price=EXCLUDED.price,source_url=EXCLUDED.source_url,collected_at=now();
