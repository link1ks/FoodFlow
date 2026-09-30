package insights

import (
	"context"
	"errors"
	"foodflow/internal/events"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"log/slog"
	"time"
)

func Consume(ctx context.Context, db *pgxpool.Pool, client *kgo.Client) {
	for ctx.Err() == nil {
		fetches := client.PollFetches(ctx)
		if ctx.Err() != nil {
			return
		}
		if len(fetches.Errors()) > 0 {
			slog.Warn("Kafka fetch failed")
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Second):
			}
		}
		// Sequential processing preserves offsets within each partition. Stop this
		// poll on database failure; never commit past an unprocessed record.
		failed := false
		fetches.EachRecord(func(r *kgo.Record) {
			if failed {
				return
			}
			for ctx.Err() == nil {
				_, validationErr := events.DecodeStock(r.Value)
				var err error
				if validationErr != nil {
					err = Reject(ctx, db, r.Topic, r.Partition, r.Offset, r.Value)
				} else {
					err = Apply(ctx, db, r.Value)
					if errors.Is(err, ErrConflictingEvent) {
						err = Reject(ctx, db, r.Topic, r.Partition, r.Offset, r.Value)
					}
				}
				if err == nil {
					if err = client.CommitRecords(ctx, r); err == nil {
						break
					}
				}
				slog.Warn("projection apply or offset commit failed")
				select {
				case <-ctx.Done():
					failed = true
					return
				case <-time.After(time.Second):
				}
			}
		})
	}
}
