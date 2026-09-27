package app

import (
	"context"
	"errors"
	"net/http/httptest"
	"testing"
)

type recordingSMS struct {
	code string
	fail bool
}

func (s *recordingSMS) Send(_ context.Context, phone, code string) error {
	s.code = code
	if s.fail {
		return errors.New("provider failure")
	}
	return nil
}
func TestSMSBindingAndLogin(t *testing.T) {
	db := testDB(t)
	a := New(db)
	sender := &recordingSMS{}
	a.sms = sender
	s := httptest.NewServer(a.Router())
	defer s.Close()
	h := testAPI{t, s}
	code, v := h.call("POST", "/register", "", "", map[string]any{"account": "unified@example.com", "password": "password123", "name": "test"})
	must(t, code, 200, v)
	session := get(v, "token")
	user := get(v, "user_id")
	code, v = h.call("POST", "/me/phone/code", session, "", map[string]any{"phone": "13800138001"})
	must(t, code, 200, v)
	challenge := get(v, "challenge_id")
	otp := sender.code
	body := map[string]any{"phone": "13800138001", "password": "password123", "code": otp, "challenge_id": challenge}
	code, v = h.call("POST", "/me/phone", session, "", body)
	must(t, code, 204, v)
	code, v = h.call("POST", "/me/phone", session, "", body)
	must(t, code, 400, v)
	for _, account := range []string{"13800138001", "unified@example.com"} {
		code, v = h.call("POST", "/login", "", "", map[string]any{"account": account, "password": "password123"})
		must(t, code, 200, v)
		if get(v, "user_id") != user {
			t.Fatal("identifiers refer to different users")
		}
	}
	code, v = h.call("POST", "/auth/sms/code", "", "", map[string]any{"phone": "13800138001", "purpose": "login"})
	must(t, code, 429, v)
	if _, err := db.Exec(context.Background(), "UPDATE sms_send_limits SET last_sent=now()-interval '61 seconds'"); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", "/auth/sms/code", "", "", map[string]any{"phone": "13800138001", "purpose": "login"})
	must(t, code, 200, v)
	login := map[string]any{"phone": "13800138001", "code": sender.code, "challenge_id": get(v, "challenge_id")}
	code, v = h.call("POST", "/auth/sms/login", "", "", login)
	must(t, code, 200, v)
	if get(v, "user_id") != user {
		t.Fatal("SMS login changed user")
	}
	code, v = h.call("POST", "/auth/sms/login", "", "", login)
	must(t, code, 401, v)
	code, v = h.call("POST", "/auth/sms/code", "", "", map[string]any{"phone": "13800138002", "purpose": "register"})
	must(t, code, 200, v)
	challenge = get(v, "challenge_id")
	otp = sender.code
	reg := map[string]any{"account": "13800138002", "password": "password123", "name": "test", "challenge_id": challenge, "code": "xxxxxx"}
	for range 5 {
		code, v = h.call("POST", "/register", "", "", reg)
		must(t, code, 400, v)
	}
	reg["code"] = otp
	code, v = h.call("POST", "/register", "", "", reg)
	must(t, code, 400, v)
	code, v = h.call("POST", "/auth/sms/code", "", "", map[string]any{"phone": "13800138005", "purpose": "register"})
	must(t, code, 200, v)
	proof := map[string]any{"phone": "13800138005", "code": sender.code, "challenge_id": get(v, "challenge_id")}
	code, v = h.call("POST", "/auth/sms/login", "", "", proof)
	must(t, code, 401, v)
	if _, e := db.Exec(context.Background(), "UPDATE sms_challenges SET expires_at=now()-interval '1 second' WHERE id::text=$1", proof["challenge_id"]); e != nil {
		t.Fatal(e)
	}
	proof["name"] = "expired"
	proof["password"] = "password123"
	code, v = h.call("POST", "/register", "", "", proof)
	must(t, code, 400, v)
	sender.fail = true
	code, v = h.call("POST", "/auth/sms/code", "", "", map[string]any{"phone": "13800138003", "purpose": "register"})
	must(t, code, 502, v)
	var state string
	if e := db.QueryRow(context.Background(), "SELECT state FROM sms_challenges WHERE phone='+8613800138003'").Scan(&state); e != nil || state != "failed" {
		t.Fatal(state, e)
	}
	a.sms = nil
	code, v = h.call("POST", "/auth/sms/code", "", "", map[string]any{"phone": "13800138004", "purpose": "login"})
	must(t, code, 503, v)
}
