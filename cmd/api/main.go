package main

import (
	"context"
	"errors"
	"foodflow/internal/app"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	"log/slog"
	"net/http"
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
	a := app.New(db)
	defer a.Close()
	addr := os.Getenv("HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	slog.Info("api started", "addr", addr)
	server := &http.Server{Addr: addr, Handler: a.Router(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 1 << 20}
	// SSE responses are long-lived: a global WriteTimeout would cut streams off.
	done := make(chan error, 1)
	go func() { done <- server.ListenAndServe() }()
	select {
	case e = <-done:
		if !errors.Is(e, http.ErrServerClosed) {
			panic(e)
		}
	case <-ctx.Done():
		deadline, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		if e = server.Shutdown(deadline); e != nil {
			slog.Warn("graceful shutdown deadline reached", "error", e)
			_ = server.Close()
		}
		slog.Info("api stopped")
	}
}
