package app

import (
	"context"
	"net/http/httptest"
	"testing"

	"foodflow/internal/core"
)

func TestAccountRecoveryReplacementAndEpochFences(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	app := New(db)
	defer app.Close()
	sender := &recordingSMS{}
	app.sms = sender
	server := httptest.NewServer(app.Router())
	defer server.Close()
	h := testAPI{t, server}
	send := func(phone, purpose, token string) (string, string) {
		t.Helper()
		// Deterministic clock setup affects only the isolated fixture's cooldown.
		if _, err := db.Exec(ctx, "UPDATE sms_send_limits SET last_sent=now()-interval '61 seconds'"); err != nil {
			t.Fatal(err)
		}
		path := "/auth/sms/code"
		if token != "" {
			path = "/me/phone/code"
		}
		code, v := h.call("POST", path, token, "", map[string]any{"phone": phone, "purpose": purpose})
		must(t, code, 200, v)
		return get(v, "challenge_id"), sender.code
	}
	challenge, otp := send("13800138101", "register", "")
	code, v := h.call("POST", "/register", "", "", map[string]any{"phone": "13800138101", "password": "Original-pass-2026!", "name": "Recover", "challenge_id": challenge, "code": otp})
	must(t, code, 200, v)
	oldToken, user := get(v, "token"), get(v, "user_id")
	wrongChallenge, wrongOTP := send("13800138101", "login", "")
	reset := map[string]any{"phone": "13800138101", "password": "New-pass-2026!", "confirm": true, "challenge_id": wrongChallenge, "code": wrongOTP}
	code, v = h.call("POST", "/auth/password/reset", "", "", reset)
	must(t, code, 400, v)
	challenge, otp = send("13800138101", "reset", "")
	reset["challenge_id"] = challenge
	reset["code"] = otp
	reset["confirm"] = false
	code, v = h.call("POST", "/auth/password/reset", "", "", reset)
	must(t, code, 400, v)
	reset["confirm"] = true
	code, v = h.call("POST", "/auth/password/reset", "", "", reset)
	must(t, code, 204, v)
	code, v = h.call("POST", "/auth/password/reset", "", "", reset)
	must(t, code, 400, v)
	code, v = h.call("GET", "/me", oldToken, "", nil)
	must(t, code, 401, v)
	code, v = h.call("POST", "/login", "", "", map[string]any{"phone": "13800138101", "password": "Original-pass-2026!"})
	must(t, code, 401, v)
	code, v = h.call("POST", "/login", "", "", map[string]any{"phone": "13800138101", "password": "New-pass-2026!"})
	must(t, code, 200, v)
	current := get(v, "token")
	// Simulate an old authentication finishing after revocation. Its stale epoch
	// still cannot authorize, even if a delayed session insert appears in the DB.
	stale, expires, err := app.signSession(user)
	if err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "INSERT INTO sessions(token_hash,user_id,expires_at,auth_version) VALUES($1,$2,$3,0)", core.Hash(stale), user, expires); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", "/me", stale, "", nil)
	must(t, code, 401, v)
	oldChallenge, oldOTP := send("13800138101", "change_old", current)
	newChallenge, newOTP := send("13800138102", "bind", current)
	bind := map[string]any{"phone": "13800138102", "password": "New-pass-2026!", "challenge_id": newChallenge, "code": newOTP, "old_challenge_id": oldChallenge, "old_code": oldOTP}
	code, v = h.call("POST", "/me/phone", current, "", bind)
	must(t, code, 409, v)
	bind["confirm_replace"] = true
	bind["code"] = "xxxxxx"
	code, v = h.call("POST", "/me/phone", current, "", bind)
	must(t, code, 400, v)
	var used bool
	if err = db.QueryRow(ctx, "SELECT used_at IS NOT NULL FROM sms_challenges WHERE id=$1", oldChallenge).Scan(&used); err != nil || used {
		t.Fatal("failed dual-proof consumed valid old proof", used, err)
	}
	bind["code"] = newOTP
	code, v = h.call("POST", "/me/phone", current, "", bind)
	must(t, code, 204, v)
	code, v = h.call("GET", "/me", current, "", nil)
	must(t, code, 401, v)
	code, v = h.call("POST", "/login", "", "", map[string]any{"phone": "13800138101", "password": "New-pass-2026!"})
	must(t, code, 401, v)
	code, v = h.call("POST", "/login", "", "", map[string]any{"phone": "13800138102", "password": "New-pass-2026!"})
	must(t, code, 200, v)
	if get(v, "user_id") != user {
		t.Fatal("phone replacement changed identity")
	}
}

func TestComplementaryAccountMergePreservesHistoryAndHouseholds(t *testing.T) {
	db := testDB(t)
	ctx := context.Background()
	app := New(db)
	defer app.Close()
	sender := &recordingSMS{}
	app.sms = sender
	server := httptest.NewServer(app.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Primary-pass-2026!", "name": "Primary"})
	must(t, code, 200, v)
	primary, target := get(v, "token"), get(v, "user_id")
	code, v = h.call("POST", "/auth/sms/code", "", "", map[string]any{"phone": "13800138103", "purpose": "register"})
	must(t, code, 200, v)
	code, v = h.call("POST", "/register", "", "", map[string]any{"phone": "13800138103", "password": "Source-pass-2026!", "name": "Source", "challenge_id": get(v, "challenge_id"), "code": sender.code})
	must(t, code, 200, v)
	sourceToken, source := get(v, "token"), get(v, "user_id")
	code, v = h.call("POST", "/households", sourceToken, "", map[string]any{"name": "Source household", "servings": 2})
	must(t, code, 201, v)
	house := get(v, "id")
	root := "/households/" + house
	if _, err := db.Exec(ctx, "INSERT INTO members(household_id,user_id,role) VALUES($1,$2,'viewer')", house, target); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/ingredients", sourceToken, "", map[string]any{"name": "番茄", "unit": "g"})
	must(t, code, 201, v)
	code, v = h.call("POST", root+"/stock", sourceToken, core.ID(), map[string]any{"ingredient_id": get(v, "id"), "quantity": "100", "reason": "purchase"})
	must(t, code, 200, v)
	code, v = h.call("POST", root+"/jobs/plan", sourceToken, "", map[string]any{"day": "2026-10-01", "meal": "dinner", "servings": 2, "max_minutes": 15})
	must(t, code, 202, v)
	job := get(v, "id")
	if _, err := db.Exec(ctx, "UPDATE sms_send_limits SET last_sent=now()-interval '61 seconds'"); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", "/me/phone/code", primary, "", map[string]any{"phone": "13800138103", "purpose": "merge"})
	must(t, code, 200, v)
	body := map[string]any{"source_account": "13800138103", "password": "Primary-pass-2026!", "source_password": "Source-pass-2026!", "challenge_id": get(v, "challenge_id"), "code": sender.code}
	code, v = h.call("POST", "/me/merge", primary, "", body)
	must(t, code, 400, v)
	body["confirm"] = true
	body["source_password"] = "wrong-password"
	code, v = h.call("POST", "/me/merge", primary, "", body)
	must(t, code, 401, v)
	body["source_password"] = "Source-pass-2026!"
	code, v = h.call("POST", "/me/merge", primary, "", body)
	must(t, code, 204, v)
	for _, token := range []string{primary, sourceToken} {
		code, v = h.call("GET", "/me", token, "", nil)
		must(t, code, 401, v)
	}
	code, v = h.call("POST", "/login", "", "", map[string]any{"phone": "13800138103", "password": "Source-pass-2026!"})
	must(t, code, 401, v)
	code, v = h.call("POST", "/login", "", "", map[string]any{"phone": "13800138103", "password": "Primary-pass-2026!"})
	must(t, code, 200, v)
	if get(v, "user_id") != target {
		t.Fatal("login did not retain current account")
	}
	canonical := get(v, "token")
	code, v = h.call("GET", root, canonical, "", nil)
	must(t, code, 200, v)
	if v["role"] != "owner" {
		t.Fatal("existing source ownership was lost")
	}
	var actor, owner, merged, jobStatus string
	var quantity int64
	if err := db.QueryRow(ctx, "SELECT l.actor_id,b.quantity_milli FROM stock_ledger l JOIN batches b ON b.id=l.batch_id WHERE l.household_id=$1", house).Scan(&actor, &quantity); err != nil || actor != source || quantity != 100000 {
		t.Fatal("merge rewrote stock/history", actor, quantity, err)
	}
	if err := db.QueryRow(ctx, "SELECT owner_id FROM households WHERE id=$1", house).Scan(&owner); err != nil || owner != target {
		t.Fatal("owner reference not transferred", owner, err)
	}
	if err := db.QueryRow(ctx, "SELECT merged_into FROM users WHERE id=$1", source).Scan(&merged); err != nil || merged != target {
		t.Fatal("source tombstone lost", merged, err)
	}
	if err := db.QueryRow(ctx, "SELECT status FROM jobs WHERE id=$1", job).Scan(&jobStatus); err != nil || jobStatus != "cancelled" {
		t.Fatal("source generation was not fenced", jobStatus, err)
	}
	if _, err := db.Exec(ctx, "DELETE FROM account_merges WHERE source_id=$1", source); err == nil {
		t.Fatal("merge audit was mutable")
	}
	code, v = h.call("POST", "/me/merge", canonical, "", body)
	must(t, code, 400, v)
	// Explicit retry runs under its new requester, not a deactivated source ID.
	if _, err := db.Exec(ctx, "UPDATE jobs SET status='failed' WHERE id=$1", job); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("POST", root+"/jobs/"+job+"/retry", canonical, "", map[string]any{})
	must(t, code, 204, v)
	var creator string
	if err := db.QueryRow(ctx, "SELECT created_by FROM jobs WHERE id=$1", job).Scan(&creator); err != nil || creator != target {
		t.Fatal("retry retained disabled creator", creator, err)
	}
}
