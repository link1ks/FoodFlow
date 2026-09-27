package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"runtime"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

func main() {
	base := flag.String("url", "http://127.0.0.1:18080", "local acceptance API")
	total := flag.Int("requests", 500, "measured GET requests (1-100000)")
	concurrency := flag.Int("concurrency", 8, "concurrent requests (1-128)")
	output := flag.String("out", "", "JSON report path")
	flag.Parse()
	u, err := url.Parse(*base)
	if err != nil || u.Scheme != "http" || (u.Hostname() != "127.0.0.1" && u.Hostname() != "localhost") || u.Port() != "18080" {
		panic("only dedicated local acceptance API on port 18080 is allowed")
	}
	if *total < 1 || *total > 100000 || *concurrency < 1 || *concurrency > 128 {
		panic("invalid workload size")
	}
	client := &http.Client{Timeout: 10 * time.Second, Transport: &http.Transport{MaxIdleConns: 256, MaxIdleConnsPerHost: 128}}
	token := ""
	call := func(method, path string, body any) map[string]any {
		raw, _ := json.Marshal(body)
		req, _ := http.NewRequest(method, *base+"/api"+path, bytes.NewReader(raw))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+token)
		req.Header.Set("Idempotency-Key", fmt.Sprintf("loadcheck-%d", time.Now().UnixNano()))
		r, e := client.Do(req)
		if e != nil {
			panic(e)
		}
		defer r.Body.Close()
		if r.StatusCode >= 300 {
			panic(fmt.Sprintf("fixture %s: HTTP %d", path, r.StatusCode))
		}
		var v map[string]any
		if e = json.NewDecoder(r.Body).Decode(&v); e != nil {
			panic(e)
		}
		return v
	}
	suffix := fmt.Sprint(time.Now().UnixNano())
	user := call("POST", "/register", map[string]any{"email": "loadcheck-" + suffix + "@example.test", "password": "loadcheck-local-only-" + suffix, "name": "Loadcheck fixture"})
	token = user["token"].(string)
	house := call("POST", "/households", map[string]any{"name": "Loadcheck " + suffix, "servings": 2})
	root := "/households/" + house["id"].(string)
	for i := 0; i < 20; i++ {
		item := call("POST", root+"/ingredients", map[string]any{"name": fmt.Sprintf("压测食材%02d", i), "unit": "g"})
		call("POST", root+"/stock", map[string]any{"ingredient_id": item["id"], "quantity": "1000", "reason": "purchase"})
	}
	request := func() bool {
		req, _ := http.NewRequest("GET", *base+"/api"+root+"/inventory", nil)
		req.Header.Set("Authorization", "Bearer "+token)
		r, e := client.Do(req)
		if e != nil {
			return false
		}
		defer r.Body.Close()
		_, e = io.Copy(io.Discard, r.Body)
		return r.StatusCode == 200 && e == nil
	}
	for i := 0; i < 10; i++ {
		if !request() {
			panic("warmup failed")
		}
	}
	times := make([]float64, *total)
	var next atomic.Int64
	var failed atomic.Int64
	var wg sync.WaitGroup
	start := time.Now()
	for c := 0; c < *concurrency; c++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				index := int(next.Add(1) - 1)
				if index >= *total {
					return
				}
				s := time.Now()
				ok := request()
				times[index] = float64(time.Since(s).Microseconds()) / 1000
				if !ok {
					failed.Add(1)
				}
			}
		}()
	}
	wg.Wait()
	elapsed := time.Since(start)
	sort.Float64s(times)
	percentile := func(p int) float64 { index := (*total*p+99)/100 - 1; return times[index] }
	report := map[string]any{"recorded_at": time.Now().UTC(), "go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH, "logical_cpus": runtime.NumCPU(), "workload": "authenticated inventory GET, one household, 20 ingredients and 20 batches, 10 warmups", "requests": *total, "concurrency": *concurrency, "failures": failed.Load(), "elapsed_seconds": elapsed.Seconds(), "requests_per_second": float64(*total) / elapsed.Seconds(), "p50_ms": percentile(50), "p95_ms": percentile(95), "p99_ms": percentile(99), "max_ms": times[len(times)-1], "limitations": "closed-loop local read workload; excludes model calls and writes; not a capacity or SLO claim"}
	data, _ := json.MarshalIndent(report, "", "  ")
	fmt.Println(string(data))
	if *output != "" {
		if err = os.WriteFile(*output, append(data, '\n'), 0600); err != nil {
			panic(err)
		}
	}
	if failed.Load() > 0 {
		os.Exit(1)
	}
}
