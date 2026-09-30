package insights

import (
	"net/http/httptest"
	"strings"
	"testing"
)

func TestInternalAuthorizationAndValidation(t *testing.T) {
	secret := strings.Repeat("s", 32)
	handler := Router(nil, secret)
	for _, c := range []struct {
		auth, url string
		status    int
	}{{"", "/v1/summary", 401}, {"Bearer wrong", "/v1/summary", 401}, {"Bearer " + secret, "/v1/summary?household_id=bad&month=2026-09", 400}} {
		req := httptest.NewRequest("GET", c.url, nil)
		req.Header.Set("Authorization", c.auth)
		w := httptest.NewRecorder()
		handler.ServeHTTP(w, req)
		if w.Code != c.status {
			t.Fatalf("got %d expected %d", w.Code, c.status)
		}
	}
}
