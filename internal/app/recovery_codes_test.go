package app

import (
	"context"
	"foodflow/internal/core"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
)

func TestRecoveryCodesRotateExpireConsumeAndRevokeSessions(t *testing.T) {
	db := testDB(t)
	a := New(db)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	ctx := context.Background()
	email := core.ID() + "@example.test"
	password := "Recovery-Origin-2026!"
	status, v := h.call("POST", "/register", "", "", map[string]any{"email": email, "password": password, "name": "Recovery"})
	must(t, status, 200, v)
	session, user := get(v, "token"), get(v, "user_id")
	issue := func(pw string, confirm bool) (int, map[string]any) {
		return h.call("POST", "/me/recovery-codes", session, "", map[string]any{"password": pw, "confirm": confirm})
	}
	status, v = issue(password, false)
	must(t, status, 400, v)
	status, v = issue("Wrong-password-2026!", true)
	must(t, status, 400, v)
	status, v = issue(password, true)
	must(t, status, 200, v)
	codes := v["codes"].([]any)
	if len(codes) != 5 {
		t.Fatal("code count differs")
	}
	first := codes[0].(string)
	var stored string
	if err := db.QueryRow(ctx, "SELECT code_hash FROM recovery_codes WHERE user_id=$1 LIMIT 1", user).Scan(&stored); err != nil || len(stored) != 64 || strings.Contains(stored, first) {
		t.Fatal("code not stored as digest")
	}
	status, v = issue(password, true)
	must(t, status, 200, v)
	codes = v["codes"].([]any)
	current := codes[0].(string)
	reset := func(account, code string, confirm bool) (int, map[string]any) {
		return h.call("POST", "/auth/recovery/reset", "", "", map[string]any{"account": account, "code": code, "password": "Recovery-New-2026!", "confirm": confirm})
	}
	// SMS recovery, phone replacement and merge advance the same credential epoch.
	if _, err := db.Exec(ctx, "UPDATE users SET auth_version=auth_version+1 WHERE id=$1", user); err != nil {
		t.Fatal(err)
	}
	status, v = reset(email, current, true)
	must(t, status, 400, v)
	status, v = h.call("POST", "/login", "", "", map[string]any{"email": email, "password": password})
	must(t, status, 200, v)
	session = get(v, "token")
	status, v = h.call("GET", "/me/recovery-codes", session, "", nil)
	must(t, status, 200, v)
	if v["remaining"] != float64(0) {
		t.Fatal("old epoch codes still counted")
	}
	status, v = issue(password, true)
	must(t, status, 200, v)
	codes = v["codes"].([]any)
	current = codes[0].(string)
	status, v = reset(email, first, true)
	must(t, status, 400, v)
	status, v = reset(email, current, false)
	must(t, status, 400, v)
	// Wrong account and wrong/expired code share the same response.
	status, v = reset("missing@example.test", current, true)
	must(t, status, 400, v)
	missing := get(v, "error")
	status, v = reset(email, first, true)
	must(t, status, 400, v)
	if get(v, "error") != missing {
		t.Fatal("account existence disclosed")
	}
	normalized, _ := normalizeRecoveryCode(current)
	if _, err := db.Exec(ctx, "UPDATE recovery_codes SET expires_at=now()-interval '1 second' WHERE code_hash=$1", core.Hash(normalized)); err != nil {
		t.Fatal(err)
	}
	status, v = reset(email, current, true)
	must(t, status, 400, v)
	current = codes[1].(string)
	// Simultaneous reset must consume the proof once under the user lock.
	var wg sync.WaitGroup
	var mu sync.Mutex
	success := 0
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			code, _ := reset(email, current, true)
			mu.Lock()
			defer mu.Unlock()
			if code == 204 {
				success++
			} else if code != 400 {
				t.Error("unexpected reset status", code)
			}
		}()
	}
	wg.Wait()
	if success != 1 {
		t.Fatal("recovery proof accepted more than once", success)
	}
	status, v = h.call("GET", "/me", session, "", nil)
	must(t, status, 401, v)
	status, v = h.call("POST", "/login", "", "", map[string]any{"email": email, "password": password})
	must(t, status, 401, v)
	status, v = h.call("POST", "/login", "", "", map[string]any{"email": email, "password": "Recovery-New-2026!"})
	must(t, status, 200, v)
	status, v = reset(email, codes[2].(string), true)
	must(t, status, 400, v)
	var remaining int
	if err := db.QueryRow(ctx, "SELECT count(*) FROM recovery_codes WHERE user_id=$1", user).Scan(&remaining); err != nil || remaining != 0 {
		t.Fatal("unused proofs survived password reset")
	}
	if _, err := db.Exec(ctx, "UPDATE auth_rate_limits SET attempts=60"); err != nil {
		t.Fatal(err)
	}
	status, v = reset(email, current, true)
	must(t, status, 429, v)
}
