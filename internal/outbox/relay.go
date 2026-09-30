// Package outbox publishes committed kitchen facts. Delivery can repeat.
package outbox

import (
	"context"
	"errors"
	"foodflow/internal/events"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"log/slog"
	"time"
)

type Producer interface {
	Publish(context.Context, string, []byte) error
}
type Kafka struct{ Client *kgo.Client }

func (k Kafka) Publish(ctx context.Context, key string, raw []byte) error {
	return k.Client.ProduceSync(ctx, &kgo.Record{Topic: events.StockTopic, Key: []byte(key), Value: raw}).FirstErr()
}

// The row stays locked until broker acknowledgement. Broker ack followed by a
// DB failure duplicates delivery; consumer inbox deduplication handles this.
func Dispatch(ctx context.Context, db *pgxpool.Pool, p Producer) (bool, error) {
	tx, err := db.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer tx.Rollback(context.Background())
	var id, home string
	var raw []byte
	err = tx.QueryRow(ctx, `SELECT event_id,household_id,payload FROM stock_outbox WHERE published_at IS NULL AND retry_at<=now() ORDER BY created_at,event_id FOR UPDATE SKIP LOCKED LIMIT 1`).Scan(&id, &home, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	deadline, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	publishErr := p.Publish(deadline, home, raw)
	if publishErr != nil {
		_, err = tx.Exec(ctx, `UPDATE stock_outbox SET attempts=attempts+1,last_error='broker publish failed',retry_at=now()+least(60, attempts+1)*interval '1 second' WHERE event_id=$1`, id)
	} else {
		_, err = tx.Exec(ctx, `UPDATE stock_outbox SET published_at=now(),attempts=attempts+1,last_error=NULL WHERE event_id=$1`, id)
	}
	if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, publishErr
}
func Run(ctx context.Context, db *pgxpool.Pool, p Producer) {
	for ctx.Err() == nil {
		worked, err := Dispatch(ctx, db, p)
		if err != nil {
			slog.Warn("outbox dispatch failed")
		}
		if !worked || err != nil {
			timer := time.NewTimer(time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}
}
