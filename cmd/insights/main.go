package main

import (
	"context"
	"foodflow/internal/events"
	"foodflow/internal/insights"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/twmb/franz-go/pkg/kgo"
	"log/slog"
	"net/http"
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
	secret := os.Getenv("INSIGHTS_SERVICE_TOKEN")
	if len(secret) < 32 || os.Getenv("KAFKA_BROKERS") == "" {
		slog.Error("service token and brokers required")
		os.Exit(1)
	}
	db, err := pgxpool.New(ctx, os.Getenv("INSIGHTS_DATABASE_URL"))
	if err != nil {
		panic(err)
	}
	defer db.Close()
	if err = insights.Migrate(ctx, db); err != nil {
		panic(err)
	}
	client, err := kgo.NewClient(kgo.SeedBrokers(strings.Split(os.Getenv("KAFKA_BROKERS"), ",")...), kgo.ConsumerGroup("foodflow-insights-v1"), kgo.ConsumeTopics(events.StockTopic), kgo.DisableAutoCommit(), kgo.ConsumeResetOffset(kgo.NewOffset().AtStart()))
	if err != nil {
		panic(err)
	}
	defer client.Close()
	go insights.Consume(ctx, db, client)
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8090"
	}
	server := &http.Server{Addr: addr, Handler: insights.Router(db, secret), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second, WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second}
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			slog.Error("insights HTTP failed")
			stop()
		}
	}()
	<-ctx.Done()
	deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	server.Shutdown(deadline)
}
