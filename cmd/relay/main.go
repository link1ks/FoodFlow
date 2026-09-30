package main

import (
	"context"
	"foodflow/internal/outbox"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"log/slog"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if os.Getenv("KAFKA_BROKERS") == "" {
		slog.Error("KAFKA_BROKERS required")
		os.Exit(1)
	}
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	client, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(os.Getenv("KAFKA_BROKERS"), ",")...), kgo.RequiredAcks(kgo.AllISRAcks()), kgo.ProducerBatchMaxBytes(64<<10), kgo.RecordDeliveryTimeout(4*time.Second))
	if err != nil {
		panic(err)
	}
	defer client.Close()
	outbox.Run(ctx, db, outbox.Kafka{Client: client})
}
