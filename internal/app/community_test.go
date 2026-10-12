package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"image"
	"image/jpeg"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"foodflow/internal/core"
)

func TestCommunityShanghaiWeekAndDraftSafety(t *testing.T) {
	for _, sample := range []struct{ at, week string }{
		{"2026-10-11T15:59:59Z", "2026-10-05"},
		{"2026-10-11T16:00:00Z", "2026-10-12"},
		{"2027-01-01T00:00:00Z", "2026-12-28"},
	} {
		at, _ := time.Parse(time.RFC3339, sample.at)
		if got := communityWeek(at).Format("2006-01-02"); got != sample.week {
			t.Fatalf("week %s: %s", sample.at, got)
		}
	}
	base := communityDraft{Title: "清炒番茄", Servings: 2, Minutes: 10, Tags: []string{"家常"}, Ingredients: []communityIngredient{{CatalogID: core.ID(), Quantity: "200", Unit: "g"}}, Steps: []string{"炒熟"}, ConfirmPublic: true, ConfirmReview: true}
	for _, edit := range []func(*communityDraft){
		func(d *communityDraft) { d.ConfirmPublic = false },
		func(d *communityDraft) { d.ConfirmReview = false },
		func(d *communityDraft) { d.Ingredients[0].Quantity = "0" },
		func(d *communityDraft) { d.Ingredients[0].Quantity = "0.0001" },
		func(d *communityDraft) { d.Steps = []string{" "} },
		func(d *communityDraft) { d.Tags = []string{"任意标签"} },
		func(d *communityDraft) { d.Tags = []string{"家常|快手"} },
	} {
		d := base
		d.Ingredients = append([]communityIngredient{}, base.Ingredients...)
		edit(&d)
		if validateCommunityDraft(&d) == nil {
			t.Fatal("unsafe draft accepted")
		}
	}
	if _, e := communityCover(base64.StdEncoding.EncodeToString([]byte(`<svg onload="alert(1)"/>`))); e == nil {
		t.Fatal("executable cover accepted")
	}
}

func TestCommunityContributionReviewAndAtomicRewards(t *testing.T) {
	t.Setenv("AI_ALLOWANCE_ENABLED", "true")
	pool := testDB(t)
	ctx := context.Background()
	application := New(pool)
	application.communityReviewer = approvedCommunityFixture()
	defer application.Close()
	server := httptest.NewServer(application.Router())
	defer server.Close()
	h := testAPI{t, server}
	reg := func(name string) (string, string) {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": name + core.ID() + "@example.test", "password": "Community-Fixture-2026!", "name": name})
		must(t, code, 200, v)
		token := get(v, "token")
		code, v = h.call("GET", "/community/account", token, "", nil)
		must(t, code, 200, v)
		return token, get(v, "user_id")
	}
	alice, aliceID := reg("社区甲")
	bob, bobID := reg("社区乙")
	carol, _ := reg("社区丙")
	dave, _ := reg("社区丁")
	admin, adminID := reg("审核员")
	t.Setenv("COMMUNITY_ADMIN_IDS", adminID)
	code, v := h.call("POST", "/households", alice, "", map[string]any{"name": "家庭权限不等于社区权限", "servings": 2})
	must(t, code, 201, v)
	code, v = h.call("GET", "/community/posts", "", "", nil)
	must(t, code, 401, v)
	var tomato string
	if e := pool.QueryRow(ctx, "SELECT id::text FROM ingredient_catalog WHERE name='番茄'").Scan(&tomato); e != nil {
		t.Fatal(e)
	}
	draft := func(title string) map[string]any {
		return map[string]any{"title": title, "story": "隔离测试配方", "servings": 2, "minutes": 10, "tags": []string{"家常"}, "ingredients": []map[string]any{{"catalog_id": tomato, "quantity": "0.2", "unit": "kg"}}, "steps": []string{"洗净切块", title + "炒熟"}, "confirm_public": true, "confirm_review": true}
	}
	var photo bytes.Buffer
	if e := jpeg.Encode(&photo, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil); e != nil {
		t.Fatal(e)
	}
	firstDraft := draft("甲的一道菜")
	firstDraft["cover_base64"] = base64.StdEncoding.EncodeToString(photo.Bytes())
	key := core.ID()
	code, v = h.call("POST", "/community/posts", alice, key, firstDraft)
	must(t, code, 200, v)
	first := get(v, "id")
	code, v = h.call("POST", "/community/posts", alice, key, firstDraft)
	must(t, code, 200, v)
	if get(v, "id") != first {
		t.Fatal("publish replay changed post")
	}
	code, v = h.call("POST", "/community/posts", alice, key, draft("改写请求"))
	must(t, code, 409, v)
	code, v = h.call("POST", "/community/posts", alice, core.ID(), firstDraft)
	must(t, code, 409, v)
	if worked, err := application.ProcessCommunityReview(ctx); !worked || err != nil {
		t.Fatal("initial review failed", err)
	}
	code, v = h.call("GET", "/community/posts/"+first, alice, "", nil)
	must(t, code, 200, v)
	ingredient := v["ingredients"].([]any)[0].(map[string]any)
	if ingredient["quantity"] != "200.000" || ingredient["unit"] != "g" {
		t.Fatalf("quantity not normalized: %+v", ingredient)
	}
	req, _ := http.NewRequest("GET", server.URL+"/api/community/posts/"+first+"/cover", nil)
	req.Header.Set("Authorization", "Bearer "+alice)
	res, e := http.DefaultClient.Do(req)
	if e != nil {
		t.Fatal(e)
	}
	raw, e := io.ReadAll(res.Body)
	res.Body.Close()
	if e != nil {
		t.Fatal(e)
	}
	validateFixtureResponse(t, req, res, raw)
	if res.StatusCode != 200 || res.Header.Get("Content-Type") != "image/jpeg" {
		t.Fatal("photo missing")
	}
	if _, e = jpeg.Decode(bytes.NewReader(raw)); e != nil {
		t.Fatal(e)
	}
	publish := func(token, title string) string {
		code, v := h.call("POST", "/community/posts", token, core.ID(), draft(title))
		must(t, code, 200, v)
		if worked, err := application.ProcessCommunityReview(ctx); !worked || err != nil {
			t.Fatal("initial review failed", err)
		}
		return get(v, "id")
	}
	second := publish(alice, "甲的另一道菜")
	bp := publish(bob, "乙的一道菜")
	cp := publish(carol, "丙的一道菜")
	dp := publish(dave, "丁的一道菜")
	code, v = h.call("POST", "/community/posts", alice, core.ID(), draft("甲的第三道菜"))
	must(t, code, 429, v)
	react := func(token, post, kind string, active bool, want int) {
		code, v := h.call("POST", "/community/posts/"+post+"/reactions/"+kind, token, core.ID(), map[string]any{"active": active})
		must(t, code, want, v)
	}
	react(alice, first, "like", true, 400)
	for _, post := range []string{first, second, bp, cp, dp} {
		react(admin, post, "like", true, 200)
	}
	react(dave, second, "like", true, 200)
	react(admin, first, "like", false, 200)
	react(admin, first, "like", true, 200)
	react(admin, first, "like", true, 200)
	react(alice, bp, "save", true, 200)
	code, v = h.call("GET", "/community/posts?filter=saved", alice, "", nil)
	must(t, code, 200, v)
	if len(v["posts"].([]any)) != 1 {
		t.Fatal("saved feed not private")
	}
	code, v = h.call("GET", "/community/account", alice, "", nil)
	must(t, code, 200, v)
	if v["balance"] != float64(8) {
		t.Fatalf("repeat like/publication awarded twice: %+v", v)
	}
	// Represent nineteen previous awards in this private database and exercise the daily cap through HTTP.
	if _, e = pool.Exec(ctx, "INSERT INTO community_points(id,user_id,amount,reason,event_key) VALUES($1,$2,19,'like','fixture-like-history')", core.ID(), bobID); e != nil {
		t.Fatal(e)
	}
	react(alice, bp, "like", true, 200)
	code, v = h.call("GET", "/community/account", bob, "", nil)
	must(t, code, 200, v)
	if v["balance"] != float64(25) {
		t.Fatal("daily first-like award cap exceeded")
	}
	if _, e = pool.Exec(ctx, "INSERT INTO community_points(id,user_id,amount,reason,event_key) VALUES($1,$2,92,'fixture','fixture-credit')", core.ID(), aliceID); e != nil {
		t.Fatal(e)
	}
	// Different simultaneous purchases total 110, but only 100 points are available.
	var wg sync.WaitGroup
	codes := make([]int, 2)
	for i, reward := range []string{"seedling", "sunshine"} {
		wg.Add(1)
		go func(i int, reward string) {
			defer wg.Done()
			codes[i], _ = h.call("POST", "/community/rewards/"+reward+"/redeem", alice, core.ID(), nil)
		}(i, reward)
	}
	wg.Wait()
	if !((codes[0] == 200 && codes[1] == 409) || (codes[1] == 200 && codes[0] == 409)) {
		t.Fatalf("concurrent spend: %v", codes)
	}
	owned := "seedling"
	cost := 30
	if codes[1] == 200 {
		owned = "sunshine"
		cost = 80
	}
	code, v = h.call("POST", "/community/rewards/"+owned+"/redeem", alice, core.ID(), nil)
	must(t, code, 200, v)
	code, v = h.call("GET", "/community/account", alice, "", nil)
	must(t, code, 200, v)
	if v["balance"] != float64(100-cost) {
		t.Fatal("repeat redemption spent twice")
	}
	code, v = h.call("POST", "/community/rewards/amber/equip", alice, core.ID(), nil)
	must(t, code, 403, v)
	if _, e = pool.Exec(ctx, "UPDATE community_points SET amount=999 WHERE user_id=$1", aliceID); e == nil {
		t.Fatal("points ledger mutable")
	}
	// Move this isolated fixture into the previous week; do not modify any development data.
	week := communityWeek(time.Now()).AddDate(0, 0, -7)
	for i, post := range []string{first, second, bp, cp, dp} {
		if _, e = pool.Exec(ctx, "UPDATE community_posts SET created_at=$2 WHERE id=$1", post, week.Add(time.Duration(i)*time.Minute)); e != nil {
			t.Fatal(e)
		}
		if _, e = pool.Exec(ctx, "UPDATE community_reactions SET created_at=$2 WHERE post_id=$1", post, week.Add(24*time.Hour)); e != nil {
			t.Fatal(e)
		}
	}
	finalizePath := "/community/weeks/" + week.Format("2006-01-02") + "/finalize"
	code, v = h.call("POST", finalizePath, alice, core.ID(), nil)
	must(t, code, 403, v)
	code, v = h.call("POST", "/community/weeks/"+communityWeek(time.Now()).Format("2006-01-02")+"/finalize", admin, core.ID(), nil)
	must(t, code, 400, v)
	code, v = h.call("POST", finalizePath, admin, core.ID(), nil)
	must(t, code, 200, v)
	code, v = h.call("GET", "/community/weekly", admin, "", nil)
	must(t, code, 200, v)
	candidates := v["candidates"].([]any)
	if len(candidates) != 3 {
		t.Fatalf("shortlist: %+v", v)
	}
	winner := candidates[0].(map[string]any)
	if winner["post_id"] != second || winner["votes"] != float64(2) {
		t.Fatal("wrong winner")
	}
	for _, c := range candidates {
		if c.(map[string]any)["post_id"] == first {
			t.Fatal("author has multiple seats")
		}
	}
	react(admin, second, "like", false, 200)
	code, v = h.call("POST", finalizePath, admin, core.ID(), nil)
	must(t, code, 200, v)
	code, v = h.call("GET", "/community/weekly", admin, "", nil)
	must(t, code, 200, v)
	if v["candidates"].([]any)[0].(map[string]any)["votes"] != float64(2) {
		t.Fatal("closed votes not frozen")
	}
	react(admin, second, "like", true, 200)
	code, v = h.call("GET", "/community/weekly", admin, "", nil)
	must(t, code, 200, v)
	if len(v["current"].([]any)) != 0 {
		t.Fatal("old first like re-entered the new week")
	}
	reviewPath := "/community/candidates/" + get(winner, "id") + "/review"
	review := map[string]any{"accept": true, "confirm": true, "note": "已核对", "workflow": []map[string]any{{"equipment": "board", "duration_seconds": 60}, {"equipment": "stove", "duration_seconds": 180}}}
	code, v = h.call("POST", reviewPath, alice, core.ID(), review)
	must(t, code, 403, v)
	code, v = h.call("POST", reviewPath, admin, core.ID(), map[string]any{"accept": true, "confirm": true})
	must(t, code, 400, v)
	reviewKey := core.ID()
	code, v = h.call("POST", reviewPath, admin, reviewKey, review)
	must(t, code, 200, v)
	recipe := get(v, "recipe_id")
	code, v = h.call("POST", reviewPath, admin, reviewKey, review)
	must(t, code, 200, v)
	if recipe != get(v, "recipe_id") {
		t.Fatal("review replay changed recipe")
	}
	code, v = h.call("POST", reviewPath, admin, core.ID(), review)
	must(t, code, 409, v)
	var quantity int64
	var unit, source string
	var steps int
	if e = pool.QueryRow(ctx, "SELECT quantity_milli,unit FROM recipe_items WHERE recipe_id=$1", recipe).Scan(&quantity, &unit); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, "SELECT source FROM recipes WHERE id=$1", recipe).Scan(&source); e != nil {
		t.Fatal(e)
	}
	if e = pool.QueryRow(ctx, "SELECT count(*) FROM recipe_steps WHERE recipe_id=$1", recipe).Scan(&steps); e != nil {
		t.Fatal(e)
	}
	if quantity != 200000 || unit != "g" || steps != 2 || !strings.Contains(source, "社区甲") {
		t.Fatal("accepted recipe lost exact BOM, workflow or attribution")
	}
	code, v = h.call("GET", "/community/account", alice, "", nil)
	must(t, code, 200, v)
	if v["balance"] != float64(150-cost) {
		t.Fatal("review award not once")
	}
	code, v = h.call("DELETE", "/community/posts/"+second, bob, core.ID(), nil)
	must(t, code, 403, v)
	code, v = h.call("DELETE", "/community/posts/"+second, alice, core.ID(), nil)
	must(t, code, 200, v)
	code, v = h.call("GET", "/community/posts/"+second, alice, "", nil)
	must(t, code, 404, v)
	if e = pool.QueryRow(ctx, "SELECT source FROM recipes WHERE id=$1", recipe).Scan(&source); e != nil {
		t.Fatal("withdrawal removed accepted recipe")
	}
}
