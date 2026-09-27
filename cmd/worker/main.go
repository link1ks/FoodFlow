package main

import (
	"context"
	"foodflow/internal/app"
	"foodflow/internal/market"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	if err := godotenv.Load(); err != nil && !os.IsNotExist(err) {
		panic("cannot parse .env; check configuration syntax")
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	db, e := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if e != nil {
		panic(e)
	}
	defer db.Close()
	if e = db.Ping(ctx); e != nil {
		panic(e)
	}
	slog.Info("worker started")
	if os.Getenv("MARKET_PRICE_SYNC") == "true" {
		go func() {
			ticker := time.NewTicker(6 * time.Hour)
			defer ticker.Stop()
			for {
				n, err := market.SyncAll(ctx, db)
				if err != nil {
					slog.Error("official prices sync partially failed", "observations", n, "error", err)
				} else {
					slog.Info("official prices synchronized", "observations", n)
				}
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
				}
			}
		}()
	}
	app.New(db).Work(ctx)
}
