package insights

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"foodflow/internal/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"net/http"
	"time"
)

type SummaryRow struct {
	Ingredient string `json:"ingredient"`
	Category   string `json:"category"`
	Unit       string `json:"unit"`
	Inbound    string `json:"inbound_milli"`
	Consumed   string `json:"consumed_milli"`
	Wasted     string `json:"wasted_milli"`
	Adjusted   string `json:"adjusted_milli"`
}

func Router(db *pgxpool.Pool, secret string) http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /health/ready", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), time.Second)
		defer cancel()
		if db.Ping(ctx) != nil {
			http.Error(w, "database unavailable", 503)
			return
		}
		w.Write([]byte("ok"))
	})
	mux.HandleFunc("GET /v1/summary", func(w http.ResponseWriter, r *http.Request) {
		if len(secret) < 32 || subtle.ConstantTimeCompare([]byte(r.Header.Get("Authorization")), []byte("Bearer "+secret)) != 1 {
			http.Error(w, "unauthorized", 401)
			return
		}
		home := r.URL.Query().Get("household_id")
		month := r.URL.Query().Get("month")
		day, err := time.Parse("2006-01", month)
		if !events.UUID(home) || err != nil {
			http.Error(w, "invalid household/month", 400)
			return
		}
		// The gateway supplies household timezone; a bounded IANA name is validated.
		zone := r.URL.Query().Get("timezone")
		if zone == "" {
			zone = "Asia/Shanghai"
		}
		loc, err := time.LoadLocation(zone)
		if err != nil {
			http.Error(w, "invalid timezone", 400)
			return
		}
		start := time.Date(day.Year(), day.Month(), 1, 0, 0, 0, 0, loc)
		end := start.AddDate(0, 1, 0)
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		rows, err := db.Query(ctx, `SELECT ingredient,category,unit,
  coalesce(sum(delta_milli) FILTER(WHERE reason IN ('purchase','manual')),0)::text,
  coalesce(-sum(delta_milli) FILTER(WHERE reason='consume'),0)::text,
  coalesce(-sum(delta_milli) FILTER(WHERE reason='waste'),0)::text,
  coalesce(sum(delta_milli) FILTER(WHERE reason='correction'),0)::text
  FROM stock_facts WHERE household_id=$1 AND occurred_at>=$2 AND occurred_at<$3
  GROUP BY ingredient,category,unit ORDER BY ingredient,unit`, home, start, end)
		if err != nil {
			http.Error(w, "projection unavailable", 503)
			return
		}
		defer rows.Close()
		result := []SummaryRow{}
		for rows.Next() {
			var item SummaryRow
			if rows.Scan(&item.Ingredient, &item.Category, &item.Unit, &item.Inbound, &item.Consumed, &item.Wasted, &item.Adjusted) != nil {
				http.Error(w, "projection unavailable", 503)
				return
			}
			result = append(result, item)
		}
		if rows.Err() != nil {
			http.Error(w, "projection unavailable", 503)
			return
		}
		var last *time.Time
		if err = db.QueryRow(ctx, "SELECT max(received_at) FROM stock_facts WHERE household_id=$1", home).Scan(&last); err != nil {
			http.Error(w, "projection unavailable", 503)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"month": month, "timezone": zone, "items": result, "last_received_at": last, "consistency": "eventual", "coverage": "ledger snapshots since migration 021"})
	})
	return mux
}
