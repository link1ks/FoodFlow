package main

import (
	"database/sql"
	"github.com/jackc/pgx/v5/stdlib"
	"github.com/pressly/goose/v3"
	"log"
	"os"
)

func main() {
	url := os.Getenv("DATABASE_URL")
	if url == "" {
		log.Fatal("DATABASE_URL required")
	}
	db, e := sql.Open("pgx", url)
	if e != nil {
		log.Fatal(e)
	}
	defer db.Close()
	_ = stdlib.GetDefaultDriver()
	if e = goose.SetDialect("postgres"); e != nil {
		log.Fatal(e)
	}
	if e = goose.Up(db, "sql/schema"); e != nil {
		log.Fatal(e)
	}
}
