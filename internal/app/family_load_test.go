package app

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"foodflow/internal/core"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Bounded local-family workload, always an isolated fixture. No capacity claim.
func TestFamilyMixedLoadConservesStockLedgerAndOutbox(t *testing.T) {
	if os.Getenv("TEST_DATABASE_URL") != "" {
		t.Fatal("family load requires its own isolated database")
	}
	db := testDB(t)
	a := New(db)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	type home struct{ root, token, ingredient, batch, id string }
	homes := make([]home, 10)
	for i := range homes {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Family-load-2026!", "name": "Load fixture"})
		must(t, code, 200, v)
		homes[i].token = get(v, "token")
		code, v = h.call("POST", "/households", homes[i].token, "", map[string]any{"name": "Load fixture", "servings": 2})
		must(t, code, 201, v)
		homes[i].id = get(v, "id")
		homes[i].root = "/households/" + homes[i].id
		code, v = h.call("POST", homes[i].root+"/ingredients", homes[i].token, "", map[string]any{"name": "番茄", "unit": "g"})
		must(t, code, 201, v)
		homes[i].ingredient = get(v, "id")
		code, v = h.call("POST", homes[i].root+"/stock", homes[i].token, core.ID(), map[string]any{"ingredient_id": homes[i].ingredient, "quantity": "1000", "reason": "purchase"})
		must(t, code, 200, v)
		homes[i].batch = get(v, "batch_id")
	}
	client := server.Client()
	client.Timeout = 5 * time.Second
	var mu sync.Mutex
	latencies := []float64{}
	errorKinds := map[string]int{}
	var failures, cycles, lockPeak atomic.Int64
	call := func(method, path, token, key string, body any, want int) bool {
		req, err := http.NewRequest(method, server.URL+"/api"+path, bytes.NewReader(core.JSON(body)))
		if err != nil {
			failures.Add(1)
			return false
		}
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Content-Type", "application/json")
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		started := time.Now()
		res, err := client.Do(req)
		ok := err == nil
		if res != nil {
			_, readErr := io.Copy(io.Discard, res.Body)
			_ = res.Body.Close()
			ok = ok && readErr == nil && res.StatusCode == want
		}
		mu.Lock()
		latencies = append(latencies, float64(time.Since(started).Microseconds())/1000)
		mu.Unlock()
		if !ok {
			failures.Add(1)
			kind := "transport"
			if res != nil {
				kind = fmt.Sprintf("http_%d_%s", res.StatusCode, method)
			}
			mu.Lock()
			errorKinds[kind]++
			mu.Unlock()
		}
		return ok
	}
	const duration = 30 * time.Second
	started := time.Now()
	deadline := started.Add(duration)
	var workers sync.WaitGroup
	stopSampler := make(chan struct{})
	samplerDone := make(chan struct{})
	go func() {
		defer close(samplerDone)
		ticker := time.NewTicker(100 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-stopSampler:
				return
			case <-ticker.C:
				var n int64
				err := db.QueryRow(context.Background(), "SELECT count(*) FROM pg_stat_activity WHERE datname=current_database() AND wait_event_type='Lock'").Scan(&n)
				if err != nil {
					failures.Add(1)
					continue
				}
				for prev := lockPeak.Load(); n > prev; prev = lockPeak.Load() {
					if lockPeak.CompareAndSwap(prev, n) {
						break
					}
				}
			}
		}
	}()
	for worker := 0; worker < 20; worker++ {
		workers.Add(1)
		go func(index int) {
			defer workers.Done()
			home := homes[index%len(homes)]
			other := homes[(index+1)%len(homes)]
			for time.Now().Before(deadline) {
				// Same-key replay is part of every write pair; total inventory stays 1000 g.
				pairOK := true
				for _, reason := range []string{"purchase", "consume"} {
					body := map[string]any{"ingredient_id": home.ingredient, "batch_id": home.batch, "quantity": "0.001", "reason": reason}
					key := core.ID()
					if !call("POST", home.root+"/stock", home.token, key, body, 200) {
						pairOK = false
						break
					}
					if !call("POST", home.root+"/stock", home.token, key, body, 200) {
						pairOK = false
						break
					}
				}
				if !pairOK {
					return
				}
				if !call("GET", home.root+"/inventory", home.token, "", nil, 200) || !call("GET", home.root+"/monthly-report", home.token, "", nil, 200) || !call("GET", other.root+"/inventory", home.token, "", nil, 404) {
					return
				}
				cycles.Add(1)
				time.Sleep(100 * time.Millisecond)
			}
		}(worker)
	}
	workers.Wait()
	close(stopSampler)
	<-samplerDone
	// Check authoritative tables after all requests settle, including exactly one
	// outbox event per ledger write and no replay-created facts.
	var invariantFailures int
	for _, home := range homes {
		var balance, ledger int64
		var events, facts int
		if err := db.QueryRow(context.Background(), `SELECT b.quantity_milli,COALESCE(sum(l.delta_milli),0)::bigint,count(l.id),count(o.event_id) FROM batches b JOIN stock_ledger l ON l.batch_id=b.id LEFT JOIN stock_outbox o ON o.event_id=l.id WHERE b.id=$1 GROUP BY b.id`, home.batch).Scan(&balance, &ledger, &facts, &events); err != nil || balance != 1000000 || ledger != balance || events != facts {
			invariantFailures++
		}
	}
	var totalFacts int
	if err := db.QueryRow(context.Background(), "SELECT count(*) FROM stock_ledger").Scan(&totalFacts); err != nil || int64(totalFacts) != 10+2*cycles.Load() {
		invariantFailures++
	}
	sort.Float64s(latencies)
	percentile := func(p float64) float64 {
		if len(latencies) == 0 {
			return 0
		}
		return latencies[int(float64(len(latencies)-1)*p)]
	}
	report := map[string]any{"households": 10, "concurrent_clients": 20, "requested_duration_seconds": 30, "elapsed_ms": time.Since(started).Milliseconds(), "request_count": len(latencies), "completed_cycles": cycles.Load(), "errors": failures.Load(), "invariant_failures": invariantFailures, "p50_ms": percentile(.5), "p95_ms": percentile(.95), "p99_ms": percentile(.99), "peak_sampled_lock_waiters": lockPeak.Load(), "lock_sampling_ms": 100, "p95_target_ms": 2000, "scope": "isolated in-process API plus real PostgreSQL; no gateway/cache/Kafka or model calls"}
	report["error_kinds"] = errorKinds
	report["max_request_ms"] = percentile(1)
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("..", "..", ".cache", "harness", "family-load.json")
	if err = os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err = os.WriteFile(path, append(raw, '\n'), 0600); err != nil {
		t.Fatal(err)
	}
	t.Log(string(raw))
	if failures.Load() != 0 || invariantFailures != 0 || cycles.Load() < 100 || percentile(.95) > 2000 {
		t.Fatal("family workload failed its bounded acceptance target")
	}
}
