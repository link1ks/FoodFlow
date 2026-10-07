package app

import (
	"context"
	"foodflow/internal/core"
	"net/http/httptest"
	"sync"
	"testing"
)

func TestAIAllowanceSerializesDebitsAndRetainsAmbiguousCalls(t *testing.T) {
	t.Setenv("AI_ALLOWANCE_ENABLED", "true")
	t.Setenv("AI_MONTHLY_ALLOWANCE_CNY", "0.20")
	t.Setenv("AI_TEXT_CALL_ALLOWANCE_CNY", "0.10")
	t.Setenv("AI_CONCURRENT_CALL_LIMIT", "1")
	db := testDB(t)
	a := New(db)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	ctx := context.Background()
	status, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Allowance-Fixture-2026!", "name": "AI"})
	must(t, status, 200, v)
	session, user := get(v, "token"), get(v, "user_id")
	status, v = h.call("POST", "/households", session, "", map[string]any{"name": "AI", "servings": 2})
	must(t, status, 201, v)
	house := get(v, "id")
	job := func() claimed {
		j := claimed{ID: core.ID(), Household: house, Creator: user, Token: core.ID()}
		if _, err := db.Exec(ctx, `INSERT INTO jobs(id,household_id,created_by,kind,status,payload,lease_token,lease_until) VALUES($1,$2,$3,'plan','running','{}',$4,now()+interval '5 minutes')`, j.ID, house, user, j.Token); err != nil {
			t.Fatal(err)
		}
		return j
	}
	one, two := job(), job()
	var wg sync.WaitGroup
	var mu sync.Mutex
	var winner claimed
	var release func(bool)
	wins := 0
	for _, j := range []claimed{one, two} {
		wg.Add(1)
		go func(j claimed) {
			defer wg.Done()
			finish, err := a.allowanceGuard(j, "text")(ctx, 100)
			if err == nil {
				mu.Lock()
				wins++
				winner = j
				release = finish
				mu.Unlock()
			}
		}(j)
	}
	wg.Wait()
	if wins != 1 {
		t.Fatal("concurrency guard allowed overlapping requests", wins)
	}
	// Recreating the App cannot clear the database allowance or slot.
	restarted := New(db)
	defer restarted.Close()
	third := job()
	if _, err := restarted.allowanceGuard(third, "text")(ctx, 100); err == nil {
		t.Fatal("restart bypassed active slot")
	}
	release(true)
	if _, err := a.allowanceGuard(winner, "text")(ctx, 100); err == nil {
		t.Fatal("same paid job was dispatched twice")
	}
	finish, err := restarted.allowanceGuard(third, "text")(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	finish(false)
	var used int64
	if err := db.QueryRow(ctx, "SELECT sum(ceiling_milli) FROM ai_call_allowances").Scan(&used); err != nil || used != 200 {
		t.Fatal("exact allowance was not fully retained", used, err)
	}
	fourth := job()
	if _, err = a.allowanceGuard(fourth, "text")(ctx, 100); err == nil {
		t.Fatal("monthly ceiling exceeded")
	}
	t.Setenv("AI_MONTHLY_ALLOWANCE_CNY", "5")
	// Even a new month cannot automatically release an uncertain old request.
	if _, err = db.Exec(ctx, "UPDATE ai_call_allowances SET month=(current_date-interval '2 months')::date,created_at=now()-interval '2 months'"); err != nil {
		t.Fatal(err)
	}
	if _, err = a.allowanceGuard(fourth, "text")(ctx, 100); err == nil {
		t.Fatal("old ambiguous slot was refunded/released")
	}
	// Explicit operator acknowledgement in this isolated fixture only.
	if _, err = db.Exec(ctx, "UPDATE ai_call_allowances SET state='completed' WHERE state='uncertain'"); err != nil {
		t.Fatal(err)
	}
	if _, err = db.Exec(ctx, "UPDATE jobs SET lease_until=now()-interval '1 second' WHERE id=$1", fourth.ID); err != nil {
		t.Fatal(err)
	}
	if _, err = a.allowanceGuard(fourth, "text")(ctx, 100); err == nil {
		t.Fatal("expired job lease dispatched")
	}
	fifth := job()
	if _, err = a.allowanceGuard(fifth, "text")(ctx, 33<<10); err == nil {
		t.Fatal("oversized text dispatched")
	}
	t.Setenv("AI_DAILY_CALL_LIMIT", "1")
	finish, err = a.allowanceGuard(fifth, "text")(ctx, 100)
	if err != nil {
		t.Fatal(err)
	}
	finish(true)
	if _, err = a.allowanceGuard(job(), "text")(ctx, 100); err == nil {
		t.Fatal("daily cap bypassed")
	}
	t.Setenv("AI_ALLOWANCE_ENABLED", "false")
	if _, err = a.allowanceGuard(job(), "text")(ctx, 100); err == nil {
		t.Fatal("disabled paid mode dispatched")
	}
	t.Setenv("AI_ALLOWANCE_ENABLED", "true")
	if _, err = db.Exec(ctx, "UPDATE members SET role='viewer' WHERE household_id=$1 AND user_id=$2", house, user); err != nil {
		t.Fatal(err)
	}
	if _, err = a.allowanceGuard(job(), "text")(ctx, 100); err == nil {
		t.Fatal("downgraded actor dispatched a paid request")
	}
	status, v = h.call("GET", "/households/"+house+"/ai-allowance", "", "", nil)
	must(t, status, 401, v)
}

func TestAIAllowanceRejectsMalformedMoney(t *testing.T) {
	for _, raw := range []string{"5.-1", "+5", "0", "-1", "1e3", "5.0000", "10000000000"} {
		t.Setenv("AI_MONTHLY_ALLOWANCE_CNY", raw)
		if _, err := loadAllowanceConfig(); err == nil {
			t.Fatal("malformed amount accepted", raw)
		}
	}
}
