package app

import (
	"context"
	"foodflow/internal/core"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
)

func TestProxyIdentityRejectsSpoofingAndSeparatesClients(t *testing.T) {
	for _, tc := range []struct {
		name, trusted, peer, real, forwarded, want string
	}{
		{"default ignores headers", "", "192.0.2.10:1234", "198.51.100.1", "198.51.100.2", "192.0.2.10"},
		{"untrusted peer", "192.0.2.20", "192.0.2.10:1234", "198.51.100.1", "198.51.100.2", "192.0.2.10"},
		{"trusted first client", "192.0.2.20", "192.0.2.20:1234", "198.51.100.1", "198.51.100.2", "198.51.100.1"},
		{"trusted second client", "192.0.2.20", "192.0.2.20:1234", "198.51.100.3", "198.51.100.2", "198.51.100.3"},
		{"no XFF fallback", "192.0.2.20", "192.0.2.20:1234", "", "198.51.100.2", "192.0.2.20"},
		{"invalid real IP", "192.0.2.20", "192.0.2.20:1234", "not-an-ip", "198.51.100.2", "192.0.2.20"},
		{"IPv6 gateway", "2001:db8::20", "[2001:db8::20]:1234", "2001:db8::1", "", "2001:db8::1"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := gin.New()
			configureTrustedProxies(r, tc.trusted)
			r.GET("/", func(c *gin.Context) { c.String(200, c.ClientIP()) })
			req := httptest.NewRequest(http.MethodGet, "/", nil)
			req.RemoteAddr = tc.peer
			req.Header.Set("X-Real-IP", tc.real)
			req.Header.Set("X-Forwarded-For", tc.forwarded)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Body.String() != tc.want {
				t.Fatalf("client identity = %q, want %q", w.Body.String(), tc.want)
			}
		})
	}
}

func TestProxyAuthenticationBudgetsRemainSeparatedAndSpoofResistant(t *testing.T) {
	pool := testDB(t)
	t.Setenv("TRUSTED_PROXIES", "192.0.2.20")
	a := New(pool)
	defer a.Close()
	router := a.Router()
	login := func(peer, real, forwarded string) int {
		req := httptest.NewRequest(http.MethodPost, "/api/login", strings.NewReader(`{"account":"missing@example.test","password":"invalid-fixture-password"}`))
		req.RemoteAddr = peer + ":1234"
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Real-IP", real)
		req.Header.Set("X-Forwarded-For", forwarded)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
		return w.Code
	}
	for i := 0; i < 60; i++ {
		if code := login("192.0.2.20", "198.51.100.1", "203.0.113.1"); code != 401 {
			t.Fatalf("valid budget request %d returned %d", i, code)
		}
	}
	if code := login("192.0.2.20", "198.51.100.1", "203.0.113.2"); code != 429 {
		t.Fatalf("forged XFF bypassed exhausted budget: %d", code)
	}
	if code := login("192.0.2.20", "198.51.100.2", "203.0.113.2"); code != 401 {
		t.Fatalf("second client's independent budget rejected: %d", code)
	}
	// Untrusted clients cannot choose a different budget with either header.
	for i := 0; i < 2; i++ {
		if code := login("192.0.2.10", "198.51.100.2", "203.0.113.2"); code != 401 {
			t.Fatalf("untrusted transport request rejected: %d", code)
		}
	}
	var attempts int
	if err := pool.QueryRow(context.Background(), "SELECT attempts FROM auth_rate_limits WHERE ip_hash=$1", core.Hash("192.0.2.10")).Scan(&attempts); err != nil || attempts != 2 {
		t.Fatalf("untrusted peer identity was not used for persisted budget: count=%d, err=%v", attempts, err)
	}
}

func TestProxyConfigurationRejectsBroadOrInvalidTrust(t *testing.T) {
	for _, value := range []string{"0.0.0.0", "::", "0.0.0.0/0", "192.0.2.0/24", "gateway", "192.0.2.20,", "224.0.0.1"} {
		t.Run(value, func(t *testing.T) {
			defer func() {
				if recover() == nil {
					t.Fatal("unsafe or invalid trust configuration accepted")
				}
			}()
			configureTrustedProxies(gin.New(), value)
		})
	}
}
