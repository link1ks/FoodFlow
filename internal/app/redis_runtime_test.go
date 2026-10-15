package app

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNewAPIRequiresReachableRedis(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-secret-with-at-least-32-bytes")
	fixture := newRedisProbeFixture(t)
	fixture.available.Store(false)
	for _, tc := range []struct {
		name, url, want string
	}{
		{"missing", "", "REDIS_URL is required"},
		{"blank", "  ", "REDIS_URL is required"},
		{"wrong scheme", "https://user:private-test-password@example.invalid", "invalid REDIS_URL"},
		{"malformed", "redis://user:private-test-password@%zz", "invalid REDIS_URL"},
		{"unavailable", fixture.url(), "redis unavailable"},
		{"unresponsive", fixture.url(), "redis unavailable"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			fixture.stall.Store(tc.name == "unresponsive")
			t.Setenv("REDIS_URL", tc.url)
			start := time.Now()
			a, err := NewAPI(context.Background(), nil)
			if a != nil || err == nil || err.Error() != tc.want {
				t.Fatalf("startup = (%v, %v), want redacted error %q", a, err, tc.want)
			}
			if elapsed := time.Since(start); elapsed > time.Second {
				t.Fatalf("startup dependency failure blocked for %v", elapsed)
			}
		})
	}
}

func TestNewAPIPingsRedisAndClosesClient(t *testing.T) {
	t.Setenv("JWT_SECRET", "test-only-secret-with-at-least-32-bytes")
	fixture := newRedisProbeFixture(t)
	t.Setenv("REDIS_URL", fixture.url())
	a, err := NewAPI(context.Background(), nil)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	if a.catalogCache == nil || fixture.pings.Load() == 0 {
		t.Fatal("API accepted Redis without checking the connection")
	}
	a.Close()
	if err = a.catalogCache.Ping(context.Background()); err == nil {
		t.Fatal("API close left the Redis client usable")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if a, err = NewAPI(ctx, nil); a != nil || err == nil || err.Error() != "redis unavailable" {
		t.Fatalf("canceled startup = (%v, %v)", a, err)
	}
}

func TestRedisReadinessOutageAndRecovery(t *testing.T) {
	db := testDB(t)
	fixture := newRedisProbeFixture(t)
	t.Setenv("REDIS_URL", fixture.url())
	a, err := NewAPI(context.Background(), db)
	if err != nil {
		t.Fatal(err)
	}
	defer a.Close()
	router := a.Router()
	probe := func(path string, want int) string {
		t.Helper()
		response := httptest.NewRecorder()
		router.ServeHTTP(response, httptest.NewRequest(http.MethodGet, path, nil))
		if response.Code != want {
			t.Fatalf("%s returned %d: %s", path, response.Code, response.Body.String())
		}
		return response.Body.String()
	}
	probe("/health/ready", http.StatusOK)
	fixture.available.Store(false)
	if body := probe("/health/ready", http.StatusServiceUnavailable); body != `{"error":"redis unavailable"}` {
		t.Fatalf("dependency error was not redacted: %s", body)
	}
	probe("/health/live", http.StatusOK)
	fixture.available.Store(true)
	probe("/health/ready", http.StatusOK)
}

// This deterministic RESP fixture checks the API lifecycle without a Redis
// installation or any business data. Platform acceptance uses real Redis.
type redisProbeFixture struct {
	listener  net.Listener
	available atomic.Bool
	stall     atomic.Bool
	pings     atomic.Uint64
	clients   sync.Map
	workers   sync.WaitGroup
}

func newRedisProbeFixture(t *testing.T) *redisProbeFixture {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	f := &redisProbeFixture{listener: listener}
	f.available.Store(true)
	acceptDone := make(chan struct{})
	f.workers.Add(1)
	go func() {
		defer f.workers.Done()
		defer close(acceptDone)
		for {
			client, err := listener.Accept()
			if err != nil {
				return
			}
			f.clients.Store(client, struct{}{})
			f.workers.Add(1)
			go func() {
				defer f.workers.Done()
				defer f.clients.Delete(client)
				defer client.Close()
				reader := bufio.NewReader(client)
				for {
					command, err := readProbeCommand(reader)
					if err != nil {
						return
					}
					response := "+OK\r\n"
					switch strings.ToLower(command[0]) {
					case "hello":
						response = "-ERR unknown command 'hello'\r\n"
					case "ping":
						f.pings.Add(1)
						if f.stall.Load() {
							continue
						}
						response = "+PONG\r\n"
						if !f.available.Load() {
							response = "-ERR fixture unavailable\r\n"
						}
					}
					if _, err := io.WriteString(client, response); err != nil {
						return
					}
				}
			}()
		}
	}()
	t.Cleanup(func() {
		_ = listener.Close()
		<-acceptDone
		f.clients.Range(func(client, _ any) bool {
			_ = client.(net.Conn).Close()
			return true
		})
		f.workers.Wait()
	})
	return f
}

func (f *redisProbeFixture) url() string { return "redis://" + f.listener.Addr().String() + "/0" }

func readProbeCommand(reader *bufio.Reader) ([]string, error) {
	header, err := reader.ReadString('\n')
	if err != nil || !strings.HasPrefix(header, "*") {
		return nil, fmt.Errorf("invalid RESP array: %w", err)
	}
	count, err := strconv.Atoi(strings.TrimSpace(header[1:]))
	if err != nil || count < 1 || count > 10 {
		return nil, fmt.Errorf("invalid RESP array length")
	}
	command := make([]string, count)
	for i := range command {
		header, err = reader.ReadString('\n')
		if err != nil || !strings.HasPrefix(header, "$") {
			return nil, fmt.Errorf("invalid RESP bulk string")
		}
		length, err := strconv.Atoi(strings.TrimSpace(header[1:]))
		if err != nil || length < 0 || length > 1024 {
			return nil, fmt.Errorf("invalid RESP bulk length")
		}
		data := make([]byte, length+2)
		if _, err = io.ReadFull(reader, data); err != nil {
			return nil, err
		}
		command[i] = string(data[:length])
	}
	return command, nil
}
