-- name: GetNutritionRadar :one
WITH bounds AS (
 SELECT (now() AT TIME ZONE h.timezone)::date today FROM households h WHERE h.id=sqlc.arg(household_id)
), meals AS (
 SELECT m.id,m.plan_meal_id,m.meal,s.servings,(m.completed_at AT TIME ZONE COALESCE(s.timezone,'Asia/Shanghai'))::date AS day
 FROM meal_completions m LEFT JOIN meal_completion_snapshots s ON s.completion_id=m.id AND s.household_id=m.household_id
 CROSS JOIN bounds b WHERE m.household_id=sqlc.arg(household_id)
 AND (m.completed_at AT TIME ZONE COALESCE(s.timezone,'Asia/Shanghai'))::date BETWEEN b.today-sqlc.arg(period_days)::int+1 AND b.today
), lines AS (
 SELECT m.*,l.id ledger_id,l.is_estimated,s.ingredient_name,s.unit,s.category,s.is_dark_vegetable,
 s.nutrition_profile->'nutrients' n,
 CASE WHEN s.unit='g' THEN -l.delta_milli/1000.0 WHEN s.unit='kg' THEN -l.delta_milli::numeric END grams,
 COALESCE(m.servings>0 AND s.unit IN ('g','kg') AND s.nutrition_profile->>'status'='reference'
 AND (s.nutrition_profile->'nutrients') ?& ARRAY['energy_kcal','protein_g','fat_g','carbs_g','fiber_g'],false) known
 FROM meals m LEFT JOIN stock_ledger l ON l.household_id=sqlc.arg(household_id) AND l.ref_type='plan_meal' AND l.ref_id=m.plan_meal_id AND l.reason='consume' AND l.delta_milli<0
 LEFT JOIN stock_ledger_snapshots s ON s.ledger_id=l.id AND s.household_id=l.household_id
), daily AS (
 SELECT day,count(DISTINCT id) meals,count(DISTINCT meal) slots,count(*) lines,count(*) FILTER(WHERE known) known_lines,
 count(*) FILTER(WHERE is_estimated) estimated_lines,
 sum(grams/100/servings*(n->>'energy_kcal')::numeric) FILTER(WHERE known) energy,
 sum(grams/100/servings*(n->>'protein_g')::numeric) FILTER(WHERE known) protein,
 sum(grams/100/servings*(n->>'fat_g')::numeric) FILTER(WHERE known) fat,
 sum(grams/100/servings*(n->>'carbs_g')::numeric) FILTER(WHERE known) carbs,
 sum(grams/100/servings*(n->>'fiber_g')::numeric) FILTER(WHERE known) fiber,
 sum(grams) FILTER(WHERE category='蔬菜') vegetable_grams,
 sum(grams) FILTER(WHERE category='蔬菜' AND is_dark_vegetable) dark_grams,
 count(*) FILTER(WHERE category='蔬菜' AND (is_dark_vegetable IS NULL OR grams IS NULL)) unknown_vegetables
 FROM lines GROUP BY day
), calendar AS (
 SELECT b.today-g.i AS day FROM bounds b CROSS JOIN generate_series(0,sqlc.arg(period_days)::int-1) g(i)
), series AS (
 SELECT c.day,COALESCE(d.meals,0) meals,COALESCE(d.slots,0) slots,COALESCE(d.lines,0) lines,COALESCE(d.known_lines,0) known_lines,
 COALESCE(d.estimated_lines,0) estimated_lines,d.energy,d.protein,d.fat,d.carbs,d.fiber,
 d.vegetable_grams,d.dark_grams,COALESCE(d.unknown_vegetables,0) unknown_vegetables
 FROM calendar c LEFT JOIN daily d USING(day)
), flagged AS (
 SELECT *,count(*) OVER w=3 AND bool_and(slots=3 AND lines=known_lines AND protein<60) OVER w protein_low_streak
 FROM series WINDOW w AS (ORDER BY day ROWS BETWEEN 2 PRECEDING AND CURRENT ROW)
)
SELECT jsonb_build_object(
 'days',sqlc.arg(period_days)::int,
 'daily',(SELECT jsonb_agg(to_jsonb(f) ORDER BY day) FROM flagged f),
 'excluded',COALESCE((SELECT jsonb_agg(x) FROM (SELECT DISTINCT COALESCE(ingredient_name,'历史记录缺少快照') name,COALESCE(unit,'未知') unit,
 CASE WHEN servings IS NULL OR ledger_id IS NULL OR ingredient_name IS NULL THEN '缺少历史消耗或份数快照' WHEN grams IS NULL THEN '缺少已确认的重量换算' ELSE '缺少营养参考数据' END reason FROM lines WHERE NOT known) x),'[]'::jsonb)
) AS report;
