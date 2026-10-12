package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"errors"
	"foodflow/internal/agent"
	"foodflow/internal/core"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type communityReviewerFixture func(context.Context, agent.CommunitySubmission, agent.CallGuard) (agent.CommunityReview, error)

func (f communityReviewerFixture) Review(c context.Context, s agent.CommunitySubmission, g agent.CallGuard) (agent.CommunityReview, error) {
	return f(c, s, g)
}
func approvedCommunityFixture() communityReviewerFixture {
	return func(ctx context.Context, s agent.CommunitySubmission, g agent.CallGuard) (agent.CommunityReview, error) {
		finish, err := g(ctx, 1000)
		if err != nil {
			return agent.CommunityReview{}, err
		}
		defer finish(true)
		return agent.CommunityReview{Decision: "approved", ImageChecked: len(s.Cover) > 0, Issues: []agent.ReviewIssue{}}, nil
	}
}

func TestCommunityInitialReviewPrivacyCoverRetryAndRewards(t *testing.T) {
	t.Setenv("AI_ALLOWANCE_ENABLED", "true")
	db := testDB(t)
	ctx := context.Background()
	a := New(db)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	register := func() (string, string) {
		code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Review-Fixture-2026!", "name": "虚构初审作者"})
		must(t, code, 200, v)
		return get(v, "token"), get(v, "user_id")
	}
	owner, user := register()
	stranger, _ := register()
	var catalog string
	if err := db.QueryRow(ctx, "SELECT id::text FROM ingredient_catalog WHERE name='番茄'").Scan(&catalog); err != nil {
		t.Fatal(err)
	}
	var imageBytes bytes.Buffer
	jpeg.Encode(&imageBytes, image.NewRGBA(image.Rect(0, 0, 8, 8)), nil)
	draft := map[string]any{"title": "待审核的番茄", "story": "虚构验收数据", "servings": 1, "minutes": 10, "tags": []string{"家常"}, "ingredients": []map[string]any{{"catalog_id": catalog, "quantity": "100.125", "unit": "g"}}, "steps": []string{"洗净切块后煮熟。"}, "confirm_public": true, "cover_base64": base64.StdEncoding.EncodeToString(imageBytes.Bytes()), "review_status": "approved"}
	code, v := h.call("POST", "/community/posts", owner, core.ID(), draft)
	must(t, code, 400, v)
	draft["confirm_review"] = true
	key := core.ID()
	code, v = h.call("POST", "/community/posts", owner, key, draft)
	must(t, code, 200, v)
	id := get(v, "id")
	if v["review_status"] != "pending" {
		t.Fatal("client forged approval")
	}
	code, v = h.call("POST", "/community/posts", owner, key, draft)
	must(t, code, 200, v)
	if get(v, "id") != id {
		t.Fatal("submission replay duplicated")
	}
	private := func() {
		for _, path := range []string{"/community/posts/" + id, "/community/posts/" + id + "/cover"} {
			code, v := h.call("GET", path, stranger, "", nil)
			must(t, code, 404, v)
		}
		code, v := h.call("GET", "/community/posts", stranger, "", nil)
		must(t, code, 200, v)
		if len(v["posts"].([]any)) != 0 {
			t.Fatal("unreviewed post public")
		}
		code, v = h.call("POST", "/community/posts/"+id+"/reactions/like", stranger, core.ID(), map[string]any{"active": true})
		must(t, code, 404, v)
		code, v = h.call("GET", "/community/account", owner, "", nil)
		must(t, code, 200, v)
		if v["balance"] != float64(0) {
			t.Fatal("points before approval")
		}
	}
	private()
	code, v = h.call("GET", "/community/posts?filter=mine", owner, "", nil)
	must(t, code, 200, v)
	if len(v["posts"].([]any)) != 1 {
		t.Fatal("author lost submission")
	}
	calls := 0
	a.communityReviewer = communityReviewerFixture(func(ctx context.Context, s agent.CommunitySubmission, g agent.CallGuard) (agent.CommunityReview, error) {
		calls++
		finish, err := g(ctx, 1000)
		if err != nil {
			return agent.CommunityReview{}, err
		}
		defer finish(true)
		if len(s.Cover) == 0 {
			t.Fatal("cover omitted")
		}
		return agent.CommunityReview{Decision: "rejected", ImageChecked: true, Issues: []agent.ReviewIssue{{Field: "cover", Reason: "请替换含广告的封面。"}}}, nil
	})
	if worked, err := a.ProcessCommunityReview(ctx); !worked || err != nil {
		t.Fatal("review did not complete", err)
	}
	private()
	code, v = h.call("GET", "/community/posts/"+id, owner, "", nil)
	must(t, code, 200, v)
	if v["review_status"] != "rejected" || !strings.Contains(get(v, "review_note"), "封面") {
		t.Fatal("no image feedback")
	}
	code, v = h.call("PATCH", "/community/posts/"+id, stranger, core.ID(), draft)
	must(t, code, 403, v)
	draft["title"] = "修改后重新提交"
	delete(draft, "cover_base64")
	editKey := core.ID()
	code, v = h.call("PATCH", "/community/posts/"+id, owner, editKey, draft)
	must(t, code, 200, v)
	if v["review_version"] != float64(2) {
		t.Fatal("revision absent")
	}
	code, v = h.call("PATCH", "/community/posts/"+id, owner, editKey, draft)
	must(t, code, 200, v)
	if v["review_version"] != float64(2) {
		t.Fatal("edit replay changed revision")
	}
	a.communityReviewer = approvedCommunityFixture()
	if _, err := a.ProcessCommunityReview(ctx); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", "/community/posts/"+id, stranger, "", nil)
	must(t, code, 200, v)
	if v["review_status"] != "approved" || v["cover_reviewed"] != true || v["has_cover"] != true {
		t.Fatal("retained cover not reviewed")
	}
	req, _ := http.NewRequest("GET", server.URL+"/api/community/posts/"+id+"/cover", nil)
	req.Header.Set("Authorization", "Bearer "+stranger)
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	res.Body.Close()
	if res.StatusCode != 200 {
		t.Fatal("approved cover unavailable")
	}
	code, v = h.call("PATCH", "/community/posts/"+id, owner, core.ID(), draft)
	must(t, code, 409, v)
	if worked, err := a.ProcessCommunityReview(ctx); worked || err != nil {
		t.Fatal("repeat worker dispatched")
	}
	var count int
	var total int64
	if err := db.QueryRow(ctx, "SELECT count(*),sum(ceiling_milli) FROM community_review_calls WHERE post_id=$1", id).Scan(&count, &total); err != nil || count != 2 || total != 1000 {
		t.Fatal("review dispatch not unique per revision", count, total, err)
	}
	if err := db.QueryRow(ctx, "SELECT sum(amount) FROM community_points WHERE user_id=$1", user).Scan(&total); err != nil || total != 5 {
		t.Fatal("reward not once", total, err)
	}
	if calls != 1 {
		t.Fatal("rejection dispatched twice")
	}
}

func TestCommunityReviewFailureWithdrawalAndAmbiguousRestart(t *testing.T) {
	t.Setenv("AI_ALLOWANCE_ENABLED", "true")
	db := testDB(t)
	ctx := context.Background()
	a := New(db)
	defer a.Close()
	server := httptest.NewServer(a.Router())
	defer server.Close()
	h := testAPI{t, server}
	code, v := h.call("POST", "/register", "", "", map[string]any{"email": core.ID() + "@example.test", "password": "Review-Fixture-2026!", "name": "失败初审"})
	must(t, code, 200, v)
	token, user := get(v, "token"), get(v, "user_id")
	var catalog string
	db.QueryRow(ctx, "SELECT id::text FROM ingredient_catalog WHERE name='番茄'").Scan(&catalog)
	draft := func(title string) map[string]any {
		return map[string]any{"title": title, "servings": 1, "minutes": 10, "tags": []string{"家常"}, "ingredients": []map[string]any{{"catalog_id": catalog, "quantity": "100", "unit": "g"}}, "steps": []string{title + "煮熟"}, "confirm_public": true, "confirm_review": true}
	}
	submit := func(title string) string {
		code, v := h.call("POST", "/community/posts", token, core.ID(), draft(title))
		must(t, code, 200, v)
		return get(v, "id")
	}
	id := submit("失败服务")
	a.communityReviewer = communityReviewerFixture(func(ctx context.Context, s agent.CommunitySubmission, g agent.CallGuard) (agent.CommunityReview, error) {
		finish, err := g(ctx, 1000)
		if err != nil {
			return agent.CommunityReview{}, err
		}
		finish(false)
		return agent.CommunityReview{}, errors.New("ambiguous local fixture")
	})
	if _, err := a.ProcessCommunityReview(ctx); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", "/community/posts/"+id, token, "", nil)
	must(t, code, 200, v)
	if v["review_status"] != "unavailable" {
		t.Fatal("failure approved")
	}
	var state string
	db.QueryRow(ctx, "SELECT state FROM community_review_calls WHERE post_id=$1", id).Scan(&state)
	if state != "uncertain" {
		t.Fatal("unknown dispatch released")
	}
	// Simulate a crash after dispatch, then recreate the worker. No paid resend.
	if _, err := db.Exec(ctx, "UPDATE community_posts SET review_status='reviewing',review_token=$2,review_until=now()-interval '1 minute' WHERE id=$1", id, core.ID()); err != nil {
		t.Fatal(err)
	}
	restarted := New(db)
	defer restarted.Close()
	restarted.communityReviewer = communityReviewerFixture(func(context.Context, agent.CommunitySubmission, agent.CallGuard) (agent.CommunityReview, error) {
		t.Fatal("ambiguous revision resent")
		return agent.CommunityReview{}, nil
	})
	if worked, err := restarted.ProcessCommunityReview(ctx); worked || err != nil {
		t.Fatal("restart unexpectedly processed", err)
	}
	// An unresolved community reservation also blocks an ordinary kitchen job.
	code, v = h.call("POST", "/households", token, "", map[string]any{"name": "共享额度隔离验收", "servings": 1})
	must(t, code, 201, v)
	job := claimed{ID: core.ID(), Household: get(v, "id"), Creator: user, Token: core.ID()}
	if _, err := db.Exec(ctx, `INSERT INTO jobs(id,household_id,created_by,kind,status,payload,lease_token,lease_until) VALUES($1,$2,$3,'plan','running','{}',$4,now()+interval '5 minutes')`, job.ID, job.Household, user, job.Token); err != nil {
		t.Fatal(err)
	}
	if _, err := a.allowanceGuard(job, "text")(ctx, 1000); err == nil {
		t.Fatal("community call not counted in shared slot")
	}
	// Operator acknowledgement on fictional DB only, to test withdrawal fencing.
	db.Exec(ctx, "UPDATE community_review_calls SET state='completed' WHERE post_id=$1", id)
	deleted := submit("审核期间撤下")
	a.communityReviewer = communityReviewerFixture(func(ctx context.Context, s agent.CommunitySubmission, g agent.CallGuard) (agent.CommunityReview, error) {
		finish, err := g(ctx, 1000)
		if err != nil {
			return agent.CommunityReview{}, err
		}
		defer finish(true)
		code, v := h.call("DELETE", "/community/posts/"+deleted, token, core.ID(), nil)
		must(t, code, 200, v)
		return agent.CommunityReview{Decision: "approved", Issues: []agent.ReviewIssue{}}, nil
	})
	if _, err := a.ProcessCommunityReview(ctx); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", "/community/posts/"+deleted, token, "", nil)
	must(t, code, 404, v)
	code, v = h.call("GET", "/community/account", token, "", nil)
	must(t, code, 200, v)
	if v["balance"] != float64(0) {
		t.Fatal("withdrawal awarded points")
	}
	// Disabled allowance never reaches a model, even on an explicit new revision.
	t.Setenv("AI_ALLOWANCE_ENABLED", "false")
	code, v = h.call("PATCH", "/community/posts/"+id, token, core.ID(), draft("重新提交失败服务"))
	must(t, code, 200, v)
	a.communityReviewer = approvedCommunityFixture()
	if _, err := a.ProcessCommunityReview(ctx); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", "/community/posts/"+id, token, "", nil)
	must(t, code, 200, v)
	if v["review_status"] != "unavailable" {
		t.Fatal("disabled allowance approved")
	}
	// A fabricated/malformed model verdict cannot be made public by the worker.
	t.Setenv("AI_ALLOWANCE_ENABLED", "true")
	code, v = h.call("PATCH", "/community/posts/"+id, token, core.ID(), draft("无效审核输出"))
	must(t, code, 200, v)
	a.communityReviewer = communityReviewerFixture(func(ctx context.Context, s agent.CommunitySubmission, g agent.CallGuard) (agent.CommunityReview, error) {
		finish, err := g(ctx, 1000)
		if err != nil {
			return agent.CommunityReview{}, err
		}
		defer finish(true)
		return agent.CommunityReview{Decision: "approved", ImageChecked: true, Issues: []agent.ReviewIssue{}}, nil
	})
	if _, err := a.ProcessCommunityReview(ctx); err != nil {
		t.Fatal(err)
	}
	code, v = h.call("GET", "/community/posts/"+id, token, "", nil)
	must(t, code, 200, v)
	if v["review_status"] != "unavailable" {
		t.Fatal("malformed model output approved")
	}
	// A crash before dispatch can be retried, unlike the ambiguous dispatched revision.
	code, v = h.call("PATCH", "/community/posts/"+id, token, core.ID(), draft("派发前恢复"))
	must(t, code, 200, v)
	if _, err := db.Exec(ctx, "UPDATE community_posts SET review_status='reviewing',review_token=$2,review_until=now()-interval '1 minute' WHERE id=$1", id, core.ID()); err != nil {
		t.Fatal(err)
	}
	a.communityReviewer = approvedCommunityFixture()
	if worked, err := a.ProcessCommunityReview(ctx); !worked || err != nil {
		t.Fatal("undispatched revision not recovered", err)
	}
	code, v = h.call("GET", "/community/posts/"+id, token, "", nil)
	must(t, code, 200, v)
	if v["review_status"] != "approved" {
		t.Fatal("recovered revision not approved")
	}
}
