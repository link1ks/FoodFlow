package app

import (
	"context"
	"encoding/json"
	"errors"
	"foodflow/internal/core"
	"foodflow/internal/events"
	"foodflow/internal/insights"
	"foodflow/internal/outbox"
	"net/http/httptest"
	"testing"
)

type failedProducer struct{}

func (failedProducer) Publish(context.Context, string, []byte) error {
	return errors.New("broker offline")
}

type capturedProducer struct{ values [][]byte }

func (p *capturedProducer) Publish(_ context.Context, _ string, raw []byte) error {
	p.values = append(p.values, append([]byte(nil), raw...))
	return nil
}
func TestOutboxAtomicRollbackRetryAndProjectionInbox(t *testing.T) {
	pool := testDB(t)
	ctx := context.Background()
	server := httptest.NewServer(New(pool).Router())
	defer server.Close()
	api := testAPI{t, server}
	code, u := api.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "password123", "name": "Outbox"})
	must(t, code, 200, u)
	token := get(u, "token")
	code, h := api.call("POST", "/households", token, "", map[string]any{"name": "Outbox", "servings": 2})
	must(t, code, 201, h)
	home := get(h, "id")
	root := "/households/" + home
	var catalog string
	if err := pool.QueryRow(ctx, "SELECT id FROM ingredient_catalog WHERE name='番茄'").Scan(&catalog); err != nil {
		t.Fatal(err)
	}
	code, stock := api.call("POST", root+"/catalog-stock", token, core.ID(), map[string]any{"catalog_id": catalog, "quantity": "100"})
	must(t, code, 200, stock)
	tx, err := pool.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	_, err = tx.Exec(ctx, "INSERT INTO stock_ledger(id,household_id,batch_id,delta_milli,reason) VALUES($1,$2,$3,-1000,'consume')", core.ID(), home, get(stock, "batch_id"))
	if err != nil {
		t.Fatal(err)
	}
	if err = tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM stock_outbox WHERE household_id=$1", home).Scan(&count); err != nil || count != 1 {
		t.Fatalf("rollback leaked outbox: %d %v", count, err)
	}
	if worked, err := outbox.Dispatch(ctx, pool, failedProducer{}); !worked || err == nil {
		t.Fatal("failed publish not observable")
	}
	var attempts int
	var published bool
	if err = pool.QueryRow(ctx, "SELECT attempts,published_at IS NOT NULL FROM stock_outbox WHERE household_id=$1", home).Scan(&attempts, &published); err != nil {
		t.Fatal(err)
	}
	if attempts != 1 || published {
		t.Fatal("failed event was marked published")
	}
	if _, err = pool.Exec(ctx, "UPDATE stock_outbox SET retry_at=now() WHERE household_id=$1", home); err != nil {
		t.Fatal(err)
	}
	producer := &capturedProducer{}
	if _, err = outbox.Dispatch(ctx, pool, producer); err != nil {
		t.Fatal(err)
	}
	if len(producer.values) != 1 {
		t.Fatal("event not dispatched")
	}
	if _, err = events.DecodeStock(producer.values[0]); err != nil {
		t.Fatal(err)
	}
	// Integration fixture uses one test database to exercise both schemas only;
	// deployed insights has a distinct database and no kitchen-table queries.
	if err = insights.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		if err = insights.Apply(ctx, pool, producer.values[0]); err != nil {
			t.Fatal(err)
		}
	}
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM stock_facts WHERE household_id=$1", home).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatal("duplicate event counted twice")
	}
	event, err := events.DecodeStock(producer.values[0])
	if err != nil {
		t.Fatal(err)
	}
	canonical, _ := json.Marshal(event)
	if err = insights.Apply(ctx, pool, canonical); err != nil {
		t.Fatalf("equivalent JSON rejected: %v", err)
	}
	event.Delta++
	conflict, _ := json.Marshal(event)
	if err = insights.Apply(ctx, pool, conflict); !errors.Is(err, insights.ErrConflictingEvent) {
		t.Fatalf("event ID content conflict not rejected: %v", err)
	}
}
