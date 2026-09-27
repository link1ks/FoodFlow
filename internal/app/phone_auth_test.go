package app

import (
	"foodflow/internal/core"
	"github.com/golang-jwt/jwt/v5"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestPhoneJWTFlow(t *testing.T) {
	db := testDB(t)
	a := New(db)
	sender := &recordingSMS{}
	a.sms = sender
	s := httptest.NewServer(a.Router())
	defer s.Close()
	h := testAPI{t, s}
	body := map[string]any{"phone": "13800138000", "password": "password123", "name": "手机用户"}
	code, v := h.call("POST", "/auth/sms/code", "", "", map[string]any{"phone": "13800138000", "purpose": "register"})
	must(t, code, 200, v)
	body["challenge_id"] = get(v, "challenge_id")
	body["code"] = sender.code
	code, v = h.call("POST", "/register", "", "", body)
	must(t, code, 200, v)
	first := get(v, "token")
	if len(strings.Split(first, ".")) != 3 {
		t.Fatal("not a JWT")
	}
	if _, err := a.verifySession(first); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", "/me", first, "", nil)
	must(t, code, 200, v)
	body["phone"] = "+8613800138000"
	code, v = h.call("POST", "/register", "", "", body)
	must(t, code, 400, v)
	code, v = h.call("POST", "/login", "", "", body)
	must(t, code, 200, v)
	session := get(v, "token")
	body["password"] = "wrong-password"
	code, v = h.call("POST", "/login", "", "", body)
	must(t, code, 401, v)
	code, v = h.call("GET", "/me", session+"x", "", nil)
	must(t, code, 401, v)
	code, v = h.call("POST", "/logout", session, "", nil)
	must(t, code, 204, v)
	code, v = h.call("GET", "/me", session, "", nil)
	must(t, code, 401, v)
	body["phone"] = "123"
	body["password"] = "password123"
	code, v = h.call("POST", "/register", "", "", body)
	must(t, code, 400, v)
	code, v = h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.com", "password": "password123", "name": "邮箱用户"})
	must(t, code, 200, v)
	if _, err := a.verifySession(get(v, "token")); err != nil {
		t.Fatal(err)
	}
}

func TestJWTValidation(t *testing.T) {
	a := &App{jwtKey: []byte("test-secret-with-at-least-32-bytes")}
	for _, tc := range []struct {
		name   string
		method jwt.SigningMethod
		expiry time.Time
		issuer string
	}{
		{"expired", jwt.SigningMethodHS256, time.Now().Add(-time.Hour), "foodflow"},
		{"wrong-algorithm", jwt.SigningMethodHS384, time.Now().Add(time.Hour), "foodflow"},
		{"wrong-issuer", jwt.SigningMethodHS256, time.Now().Add(time.Hour), "other"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			raw, e := jwt.NewWithClaims(tc.method, jwt.RegisteredClaims{Subject: core.ID(), ID: core.ID(), Issuer: tc.issuer, Audience: jwt.ClaimStrings{"foodflow-web"}, ExpiresAt: jwt.NewNumericDate(tc.expiry)}).SignedString(a.jwtKey)
			if e != nil {
				t.Fatal(e)
			}
			if _, e = a.verifySession(raw); e == nil {
				t.Fatal("invalid token accepted")
			}
		})
	}
	for _, s := range []string{"", "+1 1234567890", "1380013800x", "12800138000"} {
		if _, ok := normalizePhone(s); ok {
			t.Fatal("invalid phone accepted")
		}
	}
}
