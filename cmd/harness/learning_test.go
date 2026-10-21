package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func failureFixture(t *testing.T, root string) Failure {
	t.Helper()
	if err := saveJSON(filepath.Join(root, ".cache/harness/proof.json"), map[string]any{"passed": true}); err != nil {
		t.Fatal(err)
	}
	return Failure{ID: "stock-replay", Revision: 1, Task: "fixture", Category: "code", Business: []string{"STOCK"},
		Tests: []string{"stock#replay"}, CodeVersion: "old", ModelVersion: "not-applicable", Environment: "fixture",
		Conditions: "concurrent duplicate inventory request", Symptom: "stock added twice", Reproduction: "replay same request key",
		RootCause: "transaction lacked idempotency", Verified: true, Fix: "Check transaction idempotency key.", Scope: "stock writes",
		Status: "resolved", Severity: 4, Owner: "kitchen", Evidence: []string{".cache/harness/proof.json"},
		Regression: []string{".cache/harness/proof.json"}, PageEvidence: []string{".cache/harness/proof.json"}, RedactionReviewed: true}
}
func TestExperienceLifecycleAndSemanticRetrieval(t *testing.T) {
	root := t.TempDir()
	f := failureFixture(t, root)
	f.ErrorCode = "STOCK_REPLAY"
	f.ApplicableVersions = []string{"affected-version"}
	if e := ingestFailure(root, f); e != nil {
		t.Fatal(e)
	}
	q, e := queryExperience(root, QueryRecord{Query: "重复提交库存", Business: "STOCK", CodeVersion: "new"})
	if e != nil || len(q.Results) != 1 || !q.Results[0].VersionMismatch || len(q.Results[0].Citations) == 0 {
		t.Fatalf("missing semantic match/provenance: %+v %v", q, e)
	}
	q, e = queryExperience(root, QueryRecord{Query: "duplicate", ErrorCode: "OTHER", ApplicableVersion: "affected-version"})
	if e != nil || len(q.Results) != 0 {
		t.Fatal("error-code filter ignored")
	}
	q, e = queryExperience(root, QueryRecord{Query: "duplicate", ErrorCode: "STOCK_REPLAY", ApplicableVersion: "new-version"})
	if e != nil || len(q.Results) != 0 {
		t.Fatal("unknown applicability accepted")
	}
	q, e = queryExperience(root, QueryRecord{Query: "duplicate", ErrorCode: "STOCK_REPLAY", ApplicableVersion: "affected-version"})
	if e != nil || len(q.Results) != 1 {
		t.Fatal("declared applicability not retrieved")
	}
	q, e = queryExperience(root, QueryRecord{Query: "galaxy telescope"})
	if e != nil || len(q.Results) != 0 {
		t.Fatal("unrelated evidence returned")
	}
	if e = ingestFailure(root, f); e == nil {
		t.Fatal("history overwritten")
	}
	f.Revision = 2
	f.Status = "deleted"
	if e = ingestFailure(root, f); e != nil {
		t.Fatal(e)
	}
	q, e = queryExperience(root, QueryRecord{Query: "duplicate"})
	if e != nil || len(q.Results) != 0 {
		t.Fatal("deleted knowledge retrieved")
	}
	// Immutable old revision remains, disposable index no longer contains it.
	if _, e = os.Stat(filepath.Join(root, ".cache/harness/experience/records/stock-replay-000001.json")); e != nil {
		t.Fatal(e)
	}
}
func TestExperienceRejectsStaleIndexSecretsAndUnverifiedRepairs(t *testing.T) {
	root := t.TempDir()
	f := failureFixture(t, root)
	f.Symptom = "Authorization: Bearer private"
	if ingestFailure(root, f) == nil {
		t.Fatal("secret admitted")
	}
	f = failureFixture(t, root)
	f.Verified = false
	if ingestFailure(root, f) == nil {
		t.Fatal("unverified repair admitted")
	}
	f.Status = "open"
	if e := ingestFailure(root, f); e != nil {
		t.Fatal(e)
	}
	f.Verified = true
	f.Revision++
	if e := ingestFailure(root, f); e != nil {
		t.Fatal(e)
	}
	q, e := queryExperience(root, QueryRecord{Query: "duplicate", VerifiedOnly: true})
	if e != nil || len(q.Results) != 0 {
		t.Fatal("unverified experience treated as verified")
	}
	if _, e = queryExperience(root, QueryRecord{Query: "Authorization: Bearer fixture-secret"}); e == nil {
		t.Fatal("sensitive query admitted")
	}
	var index ExperienceIndex
	_ = readLocalJSON(root, ".cache/harness/experience/index.json", &index)
	index.Digest = "stale"
	_ = saveJSON(filepath.Join(root, ".cache/harness/experience/index.json"), index)
	if _, e = queryExperience(root, QueryRecord{Query: "duplicate"}); e == nil {
		t.Fatal("stale index accepted")
	}
	if e = rebuildExperience(root); e != nil {
		t.Fatal(e)
	}
	f.Evidence = []string{"../../private"}
	if validateFailure(root, f) == nil {
		t.Fatal("traversal accepted")
	}
}
func TestFeedbackGateRejectsPendingDevicesAndMissingMetrics(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	c := TaskContract{BoundAt: now.Add(-time.Minute), Feedback: &FeedbackScope{DeviceRequired: true, RAGExemption: "fixture", EvaluationExemption: "fixture"}}
	f := FeedbackReport{Version: 1, Task: "fixture", SourceDigest: "source", StartedAt: now, RecordedAt: now, Device: DeviceResult{Status: "pending"}}
	_ = saveJSON(filepath.Join(root, ".cache/harness/feedback.json"), f)
	if validateFeedback(root, "fixture", c, ".cache/harness/feedback.json", "source") == nil {
		t.Fatal("pending device accepted")
	}
	c.Feedback.DeviceRequired = false
	c.Feedback.DeviceExemption = "no UI change"
	if validateFeedback(root, "fixture", c, ".cache/harness/feedback.json", "source") == nil {
		t.Fatal("missing metrics accepted")
	}
}
func TestLearningMetricsRetainsFailuresAndUnknownHistory(t *testing.T) {
	root := t.TempDir()
	now := time.Now()
	for i, r := range []Evidence{
		{Task: "repair", Mode: "full", StartedAt: now, SourceDigest: "same", Checks: []Check{{Passed: false}}, Tests: []TestOutcome{{ID: "test", Status: "fail"}}},
		{Task: "repair", Mode: "full", StartedAt: now.Add(time.Second), SourceDigest: "same", Checks: []Check{{Passed: true}}, Tests: []TestOutcome{{ID: "test", Status: "pass"}}},
		{Task: "repair", Mode: "acceptance", StartedAt: now.Add(2 * time.Second), Checks: []Check{{Passed: true}}},
		{Mode: "full", Checks: []Check{{Passed: true}}},
	} {
		_ = saveJSON(filepath.Join(root, ".cache/harness/runs", string(rune('a'+i))+".json"), r)
	}
	_ = saveJSON(filepath.Join(root, ".cache/harness/experience/queries/q.json"), map[string]any{"duration_ms": 2, "results": []map[string]bool{{"version_mismatch": true}}})
	_ = saveJSON(filepath.Join(root, ".cache/harness/experience/answers/a.json"), map[string]any{"state": "returned", "latency_ms": 5, "cost_status": "unknown"})
	_ = saveJSON(filepath.Join(root, ".cache/harness/experience/evaluations/e.json"), map[string]any{"passed": true, "cases": 8})
	value, e := learningMetrics(root)
	if e != nil {
		t.Fatal(e)
	}
	m := value.(map[string]any)
	if m["unlinked_runs"] != 1 || m["additional_full_acceptance_runs"] != 1 || m["inconsistent_test_groups"] != 1 {
		t.Fatal(m)
	}
	f := m["first_pass"].(map[string]int)
	if f["numerator"] != 0 || f["denominator"] != 1 {
		t.Fatal("failed first attempt disappeared")
	}
	rag := m["rag"].(map[string]any)
	if rag["queries"] != 1 || rag["version_mismatch_results"] != 1 || rag["generation_unknown_cost"] != 1 || rag["grounded_answers"] != 1 {
		t.Fatal("RAG history lost", rag)
	}
}

func TestFeedbackGateRequiresRecordedQueryAndResolvedFailures(t *testing.T) {
	root := t.TempDir()
	before := time.Now().Add(-2 * time.Minute)
	bound := time.Now().Add(-time.Minute)
	now := time.Now()
	c := TaskContract{BoundAt: bound, Feedback: &FeedbackScope{DeviceExemption: "no hardware change", RAGExemption: "fixture", EvaluationExemption: "fixture"}}
	f := FeedbackReport{Version: 1, Task: "fixture", SourceDigest: "source", StartedAt: now, RecordedAt: now, Metrics: ".cache/harness/metrics.json"}
	initialMetrics, _ := learningMetrics(root)
	_ = saveJSON(filepath.Join(root, f.Metrics), initialMetrics)
	p := ".cache/harness/feedback.json"
	_ = saveJSON(filepath.Join(root, p), f)
	if validateFeedback(root, "fixture", c, p, "source") == nil {
		t.Fatal("missing historical query accepted")
	}
	_ = saveJSON(filepath.Join(root, ".cache/harness/experience/queries/q.json"), QueryRecord{Task: "fixture", Stage: "start", RecordedAt: before, Disposition: "no related evidence"})
	afterQuery, _ := learningMetrics(root)
	_ = saveJSON(filepath.Join(root, f.Metrics), afterQuery)
	if e := validateFeedback(root, "fixture", c, p, "source"); e != nil {
		t.Fatal(e)
	}
	failure := failureFixture(t, root)
	failure.Status = "open"
	failure.Verified = false
	if e := ingestFailure(root, failure); e != nil {
		t.Fatal(e)
	}
	if validateFeedback(root, "fixture", c, p, "source") == nil {
		t.Fatal("open failure ignored")
	}
	failure.Status = "resolved"
	failure.Verified = true
	failure.Revision++
	for _, mode := range []string{"full", "acceptance"} {
		report := Evidence{Version: 1, RunID: mode, Mode: mode, SourceDigest: "source", StartedAt: now, RecordedAt: now,
			Checks: []Check{{Name: "named-tests", Passed: true}}, Tests: []TestOutcome{{ID: "stock#replay", Status: "pass"}}}
		_ = saveJSON(filepath.Join(root, ".cache/harness/"+mode+".json"), report)
	}
	failure.Regression = []string{".cache/harness/full.json"}
	failure.PageEvidence = []string{".cache/harness/acceptance.json"}
	if e := ingestFailure(root, failure); e != nil {
		t.Fatal(e)
	}
	f.Failures = []string{failure.ID}
	_ = saveJSON(filepath.Join(root, p), f)
	refreshed, _ := learningMetrics(root)
	_ = saveJSON(filepath.Join(root, f.Metrics), refreshed)
	if e := validateFeedback(root, "fixture", c, p, "source"); e != nil {
		t.Fatal(e)
	}
	_ = saveJSON(filepath.Join(root, f.Metrics), map[string]any{"version": 1, "recorded_at": now})
	if validateFeedback(root, "fixture", c, p, "source") == nil {
		t.Fatal("placeholder metrics accepted")
	}
}

func TestFeedbackMetricsCanonicalizationKeepsUnrelatedOpenFailures(t *testing.T) {
	root := t.TempDir()
	now := time.Now().UTC()
	c := TaskContract{BoundAt: now.Add(-time.Minute), Feedback: &FeedbackScope{DeviceExemption: "no hardware", RAGExemption: "metrics only", EvaluationExemption: "metrics only"}}
	f := FeedbackReport{Version: 1, Task: "fixture", SourceDigest: "source", StartedAt: now, RecordedAt: now, Metrics: ".cache/harness/metrics.json"}
	old := failureFixture(t, root)
	old.Task, old.Status, old.Verified = "other-task", "open", false
	if err := ingestFailure(root, old); err != nil {
		t.Fatal(err)
	}
	_ = saveJSON(filepath.Join(root, ".cache/harness/experience/queries/q.json"), QueryRecord{Task: "fixture", Stage: "start", RecordedAt: now.Add(-2 * time.Minute), Disposition: "unrelated open issue retained"})
	metrics, err := learningMetrics(root)
	if err != nil {
		t.Fatal(err)
	}
	_ = saveJSON(filepath.Join(root, f.Metrics), metrics)
	path := ".cache/harness/feedback.json"
	_ = saveJSON(filepath.Join(root, path), f)
	if err := validateFeedback(root, "fixture", c, path, "source"); err != nil {
		t.Fatalf("identical metrics with retained unrelated failure rejected: %v", err)
	}
	// Semantic normalization must still reject a missing backlog or changed count.
	for _, key := range []string{"improvement_backlog", "runs", "rag"} {
		changed := metrics.(map[string]any)
		previous := changed[key]
		delete(changed, key)
		_ = saveJSON(filepath.Join(root, f.Metrics), changed)
		if validateFeedback(root, "fixture", c, path, "source") == nil {
			t.Fatalf("incomplete %s accepted", key)
		}
		changed[key] = previous
	}
}

func TestRAGFrozenAcceptance(t *testing.T) {
	root, err := filepath.Abs("../..")
	if err != nil {
		t.Fatal(err)
	}
	if err = evaluateRAG(root, filepath.Join(t.TempDir(), "rag.json"), false); err != nil {
		t.Fatal(err)
	}
}
