package app

import (
	"time"

	"github.com/gin-gonic/gin"
)

// Costs stay with their kitchen owner. The independent quantity projection is
// never queried for financial facts, nor given access to kitchen tables.
func (a *App) monthlyReport(c *gin.Context) {
	var zone string
	if err := a.DB.QueryRow(c, "SELECT timezone FROM households WHERE id=$1", hid(c)).Scan(&zone); err != nil {
		fail(c, 500, "家庭信息读取失败")
		return
	}
	loc, err := time.LoadLocation(zone)
	if err != nil {
		fail(c, 500, "家庭时区无效")
		return
	}
	month := c.Query("month")
	if month == "" {
		month = time.Now().In(loc).Format("2006-01")
	}
	start, err := time.ParseInLocation("2006-01", month, loc)
	if err != nil || start.Year() < 1970 || start.Year() > 9998 {
		fail(c, 400, "月份须为 1970–9998 年的 YYYY-MM")
		return
	}
	var raw []byte
	if err = a.DB.QueryRow(c, monthlyReportSQL, hid(c), start, start.AddDate(0, 1, 0), month, zone).Scan(&raw); err != nil {
		fail(c, 500, "月报读取失败")
		return
	}
	c.Data(200, "application/json; charset=utf-8", raw)
}

// One statement supplies a consistent snapshot. Numeric sums remain decimal
// strings. Quantities group by historical ingredient AND unit; missing old
// snapshots are kept as an explicitly unknown unit, never guessed from today.
const monthlyReportSQL = `
WITH facts AS (
 SELECT l.delta_milli,l.reason,l.is_estimated,s.ingredient_name,s.unit,c.allocated_cost
 FROM stock_ledger l
 LEFT JOIN stock_ledger_snapshots s ON s.ledger_id=l.id AND s.household_id=l.household_id
 LEFT JOIN stock_cost_snapshots c ON c.ledger_id=l.id AND c.household_id=l.household_id
 WHERE l.household_id=$1 AND l.created_at >= $2 AND l.created_at < $3
), summary AS (
 SELECT
 count(*) FILTER(WHERE reason IN ('consume','waste')) AS outbound_events,
 count(*) FILTER(WHERE reason IN ('consume','waste') AND allocated_cost IS NOT NULL) AS priced_outbound_events,
 count(*) FILTER(WHERE reason IN ('consume','waste') AND allocated_cost IS NULL) AS unknown_outbound_events,
 count(*) FILTER(WHERE reason IN ('consume','waste') AND is_estimated) AS estimated_outbound_events,
 round(COALESCE(sum(allocated_cost) FILTER(WHERE reason='consume'),0),2)::text AS known_consumed_cost,
 round(COALESCE(sum(allocated_cost) FILTER(WHERE reason='waste'),0),2)::text AS known_wasted_cost
 FROM facts
), purchases AS (
 SELECT count(*) AS purchase_records,round(COALESCE(sum(total_cost),0),2)::text AS recorded_purchase_cost
 FROM batch_purchase_costs WHERE household_id=$1 AND recorded_at >= $2 AND recorded_at < $3
), grouped AS (
 SELECT COALESCE(ingredient_name,'历史食材（缺少快照）') AS ingredient,unit,
 COALESCE(sum(delta_milli::numeric) FILTER(WHERE reason IN ('purchase','manual')),0)::text AS inbound_milli,
 COALESCE(sum(-delta_milli::numeric) FILTER(WHERE reason='consume'),0)::text AS consumed_milli,
 COALESCE(sum(-delta_milli::numeric) FILTER(WHERE reason='waste'),0)::text AS wasted_milli,
 COALESCE(sum(delta_milli::numeric) FILTER(WHERE reason='correction'),0)::text AS adjusted_milli,
 round(COALESCE(sum(allocated_cost) FILTER(WHERE reason='consume'),0),2)::text AS known_consumed_cost,
 round(COALESCE(sum(allocated_cost) FILTER(WHERE reason='waste'),0),2)::text AS known_wasted_cost,
 count(*) FILTER(WHERE reason IN ('consume','waste') AND allocated_cost IS NULL) AS unknown_outbound_events
 FROM facts GROUP BY ingredient_name,unit
)
SELECT jsonb_build_object(
 'month',$4::text,'timezone',$5::text,'currency','CNY','as_of',now(),
 'summary',(SELECT to_jsonb(summary) || to_jsonb(purchases) FROM summary,purchases),
 'item_count',(SELECT count(*) FROM grouped),
 'items',COALESCE((SELECT jsonb_agg(to_jsonb(g) ORDER BY ingredient,unit) FROM
   (SELECT * FROM grouped ORDER BY ingredient,unit LIMIT 200) g),'[]'::jsonb),
 'items_limit',200,
 'cost_basis','immutable_purchase_allocations',
 'purchase_period_basis','cost_recorded_at',
 'historical_outbound_repriced',false
)`
