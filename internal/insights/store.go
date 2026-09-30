// Package insights owns its projection database, independent of kitchen tables.
package insights

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"foodflow/internal/events"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Migrate uses a transaction-scoped lock so multiple service instances can start.
func Migrate(ctx context.Context, db *pgxpool.Pool) error {
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	_, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(482910);
 CREATE TABLE IF NOT EXISTS stock_facts (
 event_id uuid PRIMARY KEY, household_id uuid NOT NULL, ingredient text NOT NULL,
 category text NOT NULL, unit text NOT NULL, delta_milli bigint NOT NULL,
 reason text NOT NULL, occurred_at timestamptz NOT NULL, received_at timestamptz NOT NULL DEFAULT now(),digest text NOT NULL);
 CREATE INDEX IF NOT EXISTS stock_facts_household_time ON stock_facts(household_id,occurred_at);
 CREATE TABLE IF NOT EXISTS rejected_events (
 topic text NOT NULL, partition integer NOT NULL, record_offset bigint NOT NULL,
 digest text NOT NULL, error text NOT NULL, created_at timestamptz NOT NULL DEFAULT now(),
 PRIMARY KEY(topic,partition,record_offset));`)
	if err != nil {
		return err
	}
	return tx.Commit(ctx)
}

var ErrConflictingEvent = errors.New("event ID has different content")

func Apply(ctx context.Context, db *pgxpool.Pool, raw []byte) error {
	e, err := events.DecodeStock(raw)
	if err != nil {
		return err
	}
	canonical, err := json.Marshal(e)
	if err != nil {
		return err
	}
	digest := sha256.Sum256(canonical)
	hash := hex.EncodeToString(digest[:])
	tx, err := db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(context.Background())
	tag, err := tx.Exec(ctx, `INSERT INTO stock_facts(event_id,household_id,ingredient,category,unit,delta_milli,reason,occurred_at,digest) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT(event_id) DO NOTHING`, e.ID, e.HouseholdID, e.Ingredient, e.Category, e.Unit, e.Delta, e.Reason, e.OccurredAt, hash)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		var stored string
		if err = tx.QueryRow(ctx, "SELECT digest FROM stock_facts WHERE event_id=$1", e.ID).Scan(&stored); err != nil {
			return err
		}
		if stored != hash {
			return ErrConflictingEvent
		}
	}
	return tx.Commit(ctx)
}
func Reject(ctx context.Context, db *pgxpool.Pool, topic string, partition int32, offset int64, raw []byte) error {
	hash := sha256.Sum256(raw)
	_, err := db.Exec(ctx, `INSERT INTO rejected_events(topic,partition,record_offset,digest,error) VALUES($1,$2,$3,$4,'invalid or conflicting stock event') ON CONFLICT DO NOTHING`, topic, partition, offset, hex.EncodeToString(hash[:]))
	return err
}
