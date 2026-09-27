package main

import (
	"context"
	"foodflow/internal/market"
	"github.com/jackc/pgx/v5/pgxpool"
	"log"
	"os"
)

func main() {
	ctx := context.Background()
	db, err := pgxpool.New(ctx, os.Getenv("DATABASE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	defer db.Close()
	n, err := market.SyncAll(ctx, db)
	log.Printf("synced %d official observations with original monitoring dates", n)
	if err != nil {
		log.Fatal(err)
	}
}
