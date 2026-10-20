package main

import (
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func businessFixture(t *testing.T) (string, BusinessMap) {
	t.Helper()
	root := knowledgeFixture(t)
	writeFixture(t, root, "internal/app/invariant_test.go", "package app\nfunc TestStock(t *testing.T) {}\n")
	writeFixture(t, root, "web/e2e/stock.spec.ts", `test("stock", async () => {});`)
	writeFixture(t, root, "docs/guide.md", "Stock never becomes negative.")
	b := BusinessMap{Version: 1, Requirements: []BusinessRequirement{{
		ID: "STOCK", Context: "Exact stock", Guide: "docs/guide.md", Sources: []string{"internal", "cmd", "web/src", "sql"},
		Browser: []string{"web/e2e/stock.spec.ts#stock"}, Data: "zero, negative, maximum", Limits: "bounded fixture",
	}}, Tests: []TestBinding{{ID: "internal/app/invariant_test.go#TestStock", Requirements: []string{"STOCK"}}, {ID: "web/e2e/stock.spec.ts#stock", Requirements: []string{"STOCK"}}}}
	if err := saveJSON(filepath.Join(root, businessPath), b); err != nil {
		t.Fatal(err)
	}
	if err := business(root, true); err != nil {
		t.Fatal(err)
	}
	return root, b
}

func TestBusinessRejectsContextDriftAndUnboundTests(t *testing.T) {
	root, _ := businessFixture(t)
	if err := business(root, false); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "docs/guide.md", "Changed business assertion")
	if err := business(root, false); err == nil || !strings.Contains(err.Error(), "context changed") {
		t.Fatalf("guide drift accepted: %v", err)
	}
	if err := business(root, true); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, "internal/app/new_test.go", "package app\nfunc TestNew(t *testing.T) {}\n")
	if err := business(root, true); err == nil || !strings.Contains(err.Error(), "binding") {
		t.Fatalf("refresh silently accepted unbound test: %v", err)
	}
}

func TestTaskContractRejectsUnrelatedCoverageAndOverwrite(t *testing.T) {
	root, _ := businessFixture(t)
	c := TaskContract{Version: 1, Task: "repair", Criteria: []TaskCriterion{{ID: "user-1", Description: "stock invariant", Business: []string{"STOCK"}, Tests: []string{"internal/app/invariant_test.go#TestStock"}}}}
	if err := validateTaskContract(root, c); err != nil {
		t.Fatal(err)
	}
	c.Criteria[0].Tests = []string{"does-not-exist"}
	if err := validateTaskContract(root, c); err == nil {
		t.Fatal("unrelated test accepted")
	}
	c.Criteria[0].Tests = []string{"internal/app/invariant_test.go#TestStock"}
	if err := saveJSON(filepath.Join(root, "contract.json"), c); err != nil {
		t.Fatal(err)
	}
	if err := recordTask(root, "repair", "start", "maintenance", "unknown", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := bindTask(root, "repair", "contract.json"); err != nil {
		t.Fatal(err)
	}
	if err := bindTask(root, "repair", "contract.json"); err == nil {
		t.Fatal("bound contract overwritten")
	}
}

func TestCompletionRejectsMissingStaleFailedAndForeignGates(t *testing.T) {
	start := time.Now().Add(-time.Minute)
	valid := Evidence{Version: 1, RunID: "run", Mode: "main-site", SourceDigest: "source", StartedAt: start.Add(time.Second), RecordedAt: time.Now(), Checks: []Check{{Name: "main-site-browser", Passed: true}}}
	if err := requiredReport(valid, "main-site", "source", start, []string{"main-site-browser"}); err != nil {
		t.Fatal(err)
	}
	for _, change := range []string{"missing", "failed", "stale", "foreign", "duplicate", "future"} {
		t.Run(change, func(t *testing.T) {
			r := valid
			r.Checks = append([]Check(nil), valid.Checks...)
			switch change {
			case "missing":
				r.Checks = nil
			case "failed":
				r.Checks[0].Passed = false
			case "stale":
				r.StartedAt = start.Add(-time.Second)
			case "foreign":
				r.SourceDigest = "different"
			case "duplicate":
				r.Checks = append(r.Checks, r.Checks[0])
			case "future":
				r.RecordedAt = time.Now().Add(time.Hour)
			}
			if err := requiredReport(r, "main-site", "source", start, []string{"main-site-browser"}); err == nil {
				t.Fatal("invalid completion gate accepted")
			}
		})
	}
}

func TestNamedResultsRejectMissingSkippedAndOneViewport(t *testing.T) {
	root, _ := businessFixture(t)
	tests := []TestOutcome{{ID: "web/e2e/stock.spec.ts#stock", Project: "desktop", Status: "pass"}}
	if err := verifyNamedResults(root, "acceptance", tests); err == nil {
		t.Fatal("missing mobile accepted")
	}
	tests = append(tests, TestOutcome{ID: "web/e2e/stock.spec.ts#stock", Project: "mobile", Status: "pass"})
	if err := verifyNamedResults(root, "acceptance", tests); err != nil {
		t.Fatal(err)
	}
	tests[1].Status = "skip"
	if err := verifyNamedResults(root, "acceptance", tests); err == nil {
		t.Fatal("skipped browser test accepted")
	}
	if err := verifyNamedResults(root, "full", nil); err == nil {
		t.Fatal("missing Go results accepted")
	}
}

func TestWorkloadRejectsUndersizedOrUnmeasuredData(t *testing.T) {
	root := t.TempDir()
	start := time.Now().Add(-time.Second)
	data := map[string]any{"households": 10, "concurrent_clients": 20, "requested_duration_seconds": 30, "completed_cycles": 100, "request_count": 700, "errors": 0, "invariant_failures": 0, "p95_ms": 100}
	path := filepath.Join(root, ".cache/harness/family-load.json")
	if err := saveJSON(path, data); err != nil {
		t.Fatal(err)
	}
	if _, err := validateDataScale(root, start); err != nil {
		t.Fatal(err)
	}
	data["completed_cycles"] = 1
	_ = saveJSON(path, data)
	if _, err := validateDataScale(root, start); err == nil {
		t.Fatal("tiny workload accepted")
	}
	delete(data, "errors")
	_ = saveJSON(path, data)
	if _, err := validateDataScale(root, start); err == nil {
		t.Fatal("unmeasured errors accepted")
	}
}

func TestGoResultsCollectSplitEventsWithoutRawLogs(t *testing.T) {
	g := &goResults{}
	_, _ = g.Write([]byte("{\"Action\":\"pass\",\"Package\":\"foodflow/internal/app\","))
	_, _ = g.Write([]byte("\"Test\":\"TestStock\"}\n{\"Action\":\"output\",\"Output\":\"private payload\"}\n"))
	if g.results["internal/app#TestStock"] != "pass" || len(g.results) != 1 || len(g.pending) != 0 {
		t.Fatal("event collection lost result or retained raw logs")
	}
}

func TestVerificationLockRejectsOverlappingSuites(t *testing.T) {
	root := t.TempDir()
	release, err := verificationLock(root)
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err := verificationLock(root); err == nil {
		t.Fatal("overlapping memory-heavy verification accepted")
	}
	release()
	next, err := verificationLock(root)
	if err != nil {
		t.Fatal(err)
	}
	next()
}

func TestBusinessRejectsUnreviewedImplementation(t *testing.T) {
	root, b := businessFixture(t)
	b.Requirements[0].Sources = []string{"internal/app/invariant_test.go"}
	if err := sourceCoverage(root, b); err == nil {
		t.Fatal("unreviewed implementation accepted")
	}
}

func TestUnifiedCompletionAcceptsOnlyAllGatesAfterBinding(t *testing.T) {
	root, _ := businessFixture(t)
	c := TaskContract{Version: 1, Task: "complete", Criteria: []TaskCriterion{{ID: "stock", Description: "precise stock", Business: []string{"STOCK"}, Tests: []string{"internal/app/invariant_test.go#TestStock"}}}}
	if err := saveJSON(filepath.Join(root, "contract.json"), c); err != nil {
		t.Fatal(err)
	}
	writeFixture(t, root, ".gitignore", ".cache/\n")
	for _, args := range [][]string{
		{"-C", root, "init", "-q"},
		{"-C", root, "add", "."},
		{"-C", root, "-c", "user.name=Harness Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "-qm", "fixture"},
	} {
		if output, err := command("git", args...); err != nil {
			t.Fatalf("fixture git: %v %s", err, output)
		}
	}
	revision, _ := command("git", "-C", root, "rev-parse", "HEAD")
	if err := recordTask(root, "complete", "start", "maintenance", "unknown", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := bindTask(root, "complete", "contract.json"); err != nil {
		t.Fatal(err)
	}
	digest, err := sourceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	input := CompletionInput{Version: 2, Full: ".cache/harness/full.json", Acceptance: ".cache/harness/acceptance.json", Diagnose: ".cache/harness/diagnose.json", MainSite: ".cache/harness/main-site.json", Cloud: ".cache/harness/cloud.json"}
	checks := map[string][]string{
		"full":       {"business-map", "named-tests", "data-scale", "update-regression", "release-regression", "quality-map", "dbgen-check", "dbgen-regression", "format", "vet", "go-tests", "web-api-check", "web-test", "web-build"},
		"acceptance": {"business-map", "acceptance-images", "platform", "kitchen-flow", "gateway-recovery", "browser-fixture", "browser", "named-tests", "runtime-diagnostics"},
		"diagnose":   {"runtime-diagnostics"},
		"main-site":  {"main-site-browser"},
		"cloud":      {"commit-ci"},
	}
	for mode, names := range checks {
		r := Evidence{Version: 1, RunID: mode, Mode: mode, Revision: strings.TrimSpace(revision), IntegrationRequired: true, SourceDigest: digest, StartedAt: time.Now(), RecordedAt: time.Now()}
		for _, name := range append(names, "architecture", "repository-hygiene", "knowledge", "source-stable") {
			r.Checks = append(r.Checks, Check{Name: name, Passed: true})
		}
		if mode == "full" {
			r.Tests = []TestOutcome{{ID: "internal/app/invariant_test.go#TestStock", Status: "pass"}}
		}
		if mode == "acceptance" {
			r.Tests = []TestOutcome{{ID: "web/e2e/stock.spec.ts#stock", Project: "desktop", Status: "pass"}, {ID: "web/e2e/stock.spec.ts#stock", Project: "mobile", Status: "pass"}}
		}
		if err := saveJSON(filepath.Join(root, ".cache/harness/"+mode+".json"), r); err != nil {
			t.Fatal(err)
		}
	}
	path := ".cache/harness/completion.json"
	_ = saveJSON(filepath.Join(root, path), input)
	var task TaskRecord
	_ = readLocalJSON(root, ".cache/harness/tasks/complete.start.json", &task)
	if _, err := validateCompletion(root, task, path, digest); err != nil {
		t.Fatal(err)
	}
	input.MainSite = ""
	_ = saveJSON(filepath.Join(root, path), input)
	if _, err := validateCompletion(root, task, path, digest); err == nil {
		t.Fatal("missing main-site accepted")
	}
	input.MainSite = ".cache/harness/main-site.json"
	_ = saveJSON(filepath.Join(root, path), input)
	var report Evidence
	_ = readLocalJSON(root, input.MainSite, &report)
	report.StartedAt = task.StartedAt.Add(-time.Second)
	_ = saveJSON(filepath.Join(root, input.MainSite), report)
	if _, err := validateCompletion(root, task, path, digest); err == nil {
		t.Fatal("pre-contract verification accepted")
	}
}
