package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestQualityRejectsLostScenarioAndEscapingPath(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "test.go"), []byte("func TestInvariant() {}"), 0600); err != nil {
		t.Fatal(err)
	}
	domains := []QualityDomain{{ID: "inventory", Owner: "test.go", Reviewed: "2026-10-01", Acceptance: []string{"stock remains nonnegative"}, Checks: []QualityCheck{{Path: "test.go", Anchor: "func TestInvariant("}}}}
	if err := validateQuality(root, domains); err != nil {
		t.Fatal(err)
	}
	domains[0].Checks[0].Anchor = "func TestMissing("
	if err := validateQuality(root, domains); err == nil {
		t.Fatal("removed scenario accepted")
	}
	domains[0].Checks[0] = QualityCheck{Path: "../outside.go", Anchor: "func TestInvariant("}
	if err := validateQuality(root, domains); err == nil {
		t.Fatal("escaping scenario accepted")
	}
	domains[0].Checks[0] = QualityCheck{Path: "test.go", Anchor: "func TestInvariant("}
	domains[0].Debts = []QualityDebt{{ID: "INV-TEST", Priority: "P1", Status: "closed", Description: "load risk", Completion: "measured acceptance"}}
	if err := validateQuality(root, domains); err == nil {
		t.Fatal("closed debt without evidence accepted")
	}
	domains[0].Debts[0].Evidence = "test.go"
	if err := validateQuality(root, domains); err != nil {
		t.Fatal(err)
	}
}

func TestTaskFailureClassification(t *testing.T) {
	root := t.TempDir()
	if err := recordTask(root, "fixture-repair", "start", "regression", "reproduced", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := recordTask(root, "fixture-repair", "finish", "", "", "failed", "", "invalid"); err == nil {
		t.Fatal("invalid classification accepted")
	}
	if err := recordTask(root, "fixture-repair", "finish", "", "", "failed", "", "fixture"); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, ".cache/harness/tasks/fixture-repair.finish.json"))
	if err != nil {
		t.Fatal(err)
	}
	var record TaskRecord
	if err = json.Unmarshal(raw, &record); err != nil || record.FailureCategory != "fixture" {
		t.Fatal("failure category lost")
	}
	summary, err := historySummary(root)
	if err != nil {
		t.Fatal(err)
	}
	if summary.(map[string]any)["failed_task_categories"].(map[string]int)["fixture"] != 1 {
		t.Fatal("failure category not summarized")
	}
}
