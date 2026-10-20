package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type TaskCriterion struct {
	ID          string   `json:"id"`
	Description string   `json:"description"`
	Business    []string `json:"business"`
	Tests       []string `json:"tests"`
}

type TaskContract struct {
	Feedback *FeedbackScope  `json:"feedback,omitempty"`
	Version  int             `json:"version"`
	Task     string          `json:"task"`
	BoundAt  time.Time       `json:"bound_at,omitempty"`
	Criteria []TaskCriterion `json:"criteria"`
}

type CompletionInput struct {
	Feedback   string `json:"feedback,omitempty"`
	Version    int    `json:"version"`
	Full       string `json:"full"`
	Acceptance string `json:"acceptance"`
	Diagnose   string `json:"diagnose"`
	MainSite   string `json:"main_site"`
	Cloud      string `json:"cloud"`
}

func validateTaskContract(root string, c TaskContract) error {
	if _, err := os.Stat(filepath.Join(root, "docs/quality/feedback-policy.json")); err == nil {
		if err := validateFeedbackScope(c.Feedback); err != nil {
			return err
		}
	}
	b, err := readBusiness(root)
	if err != nil {
		return err
	}
	if c.Version != 1 || !taskName.MatchString(c.Task) || len(c.Criteria) == 0 {
		return fmt.Errorf("task needs a versioned nonempty acceptance contract")
	}
	requirements := map[string]bool{}
	bindings := map[string]map[string]bool{}
	for _, r := range b.Requirements {
		requirements[r.ID] = true
	}
	for _, t := range b.Tests {
		bindings[t.ID] = map[string]bool{}
		for _, id := range t.Requirements {
			bindings[t.ID][id] = true
		}
	}
	seen := map[string]bool{}
	for _, criterion := range c.Criteria {
		if criterion.ID == "" || seen[criterion.ID] || criterion.Description == "" || len(criterion.Business) == 0 || len(criterion.Tests) == 0 {
			return fmt.Errorf("task criterion needs unique ID, description, business and tests")
		}
		seen[criterion.ID] = true
		for _, id := range criterion.Business {
			if !requirements[id] {
				return fmt.Errorf("unknown task business requirement %s", id)
			}
			covered := false
			for _, test := range criterion.Tests {
				covered = covered || bindings[test][id]
			}
			if !covered {
				return fmt.Errorf("criterion %s lacks a test bound to %s", criterion.ID, id)
			}
		}
		for _, test := range criterion.Tests {
			if bindings[test] == nil {
				return fmt.Errorf("unknown criterion test %s", test)
			}
		}
	}
	return nil
}

// Binding is exclusive. Changes in scope require a new bounded task; previous
// outcomes and contracts remain intact instead of silently weakening criteria.
func bindTask(root, id, path string) error {
	if !taskName.MatchString(id) {
		return fmt.Errorf("invalid task ID")
	}
	full, err := repositoryFile(root, path)
	if err != nil {
		return err
	}
	var c TaskContract
	raw, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	if err = json.Unmarshal(raw, &c); err != nil {
		return err
	}
	if c.Task != id {
		return fmt.Errorf("contract task mismatch")
	}
	if err = validateTaskContract(root, c); err != nil {
		return err
	}
	c.BoundAt = time.Now().UTC()
	dir := filepath.Join(root, ".cache/harness/tasks")
	if _, err = os.Stat(filepath.Join(dir, id+".start.json")); err != nil {
		return err
	}
	if _, err = os.Stat(filepath.Join(dir, id+".finish.json")); !os.IsNotExist(err) {
		return fmt.Errorf("cannot bind a finished task")
	}
	return writeExclusive(filepath.Join(dir, id+".contract.json"), c)
}

func readLocalJSON(root, path string, target any) error {
	if !strings.HasPrefix(path, ".cache/harness/") {
		return fmt.Errorf("evidence must be under .cache/harness")
	}
	full, err := repositoryFile(root, path)
	if err != nil {
		return err
	}
	raw, err := os.ReadFile(full)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func requiredReport(r Evidence, mode, digest string, started time.Time, checks []string) error {
	if r.Version != 1 || r.RunID == "" || r.Mode != mode || r.SourceDigest != digest || r.StartedAt.Before(started) || r.RecordedAt.Before(r.StartedAt) || r.RecordedAt.After(time.Now().Add(time.Minute)) {
		return fmt.Errorf("%s evidence is stale, foreign or incomplete", mode)
	}
	found := map[string]bool{}
	for _, c := range r.Checks {
		if !c.Passed || found[c.Name] {
			return fmt.Errorf("%s contains failed or duplicate checks", mode)
		}
		found[c.Name] = true
	}
	for _, name := range checks {
		if !found[name] {
			return fmt.Errorf("%s is missing %s", mode, name)
		}
	}
	return nil
}

func validateCompletion(root string, task TaskRecord, path, digest string) (string, error) {
	var input CompletionInput
	if err := readLocalJSON(root, path, &input); err != nil {
		return "", err
	}
	if input.Version != 2 {
		return "", fmt.Errorf("completion needs version 2 bundle: full, acceptance, diagnose, main_site, cloud")
	}
	var contract TaskContract
	if err := readLocalJSON(root, ".cache/harness/tasks/"+task.ID+".contract.json", &contract); err != nil {
		return "", fmt.Errorf("bind acceptance contract before verification: %w", err)
	}
	if err := validateTaskContract(root, contract); err != nil {
		return "", err
	}
	// Legacy bound records must be rebound as a new task; never backdate review.
	if contract.BoundAt.IsZero() || contract.BoundAt.Before(task.StartedAt) {
		return "", fmt.Errorf("task contract lacks a valid binding timestamp")
	}
	if contract.Feedback != nil {
		if err := validateFeedback(root, task.ID, contract, input.Feedback, digest); err != nil {
			return "", err
		}
	}
	var allTests []TestOutcome
	var fullID string
	for _, gate := range []struct {
		mode, path string
		checks     []string
	}{
		{"full", input.Full, []string{"business-map", "named-tests", "data-scale", "update-regression", "release-regression"}},
		{"acceptance", input.Acceptance, []string{"business-map", "acceptance-images", "platform", "kitchen-flow", "gateway-recovery", "browser-fixture", "browser", "named-tests", "runtime-diagnostics"}},
		{"diagnose", input.Diagnose, []string{"runtime-diagnostics"}},
		{"main-site", input.MainSite, []string{"main-site-browser"}},
		{"cloud", input.Cloud, []string{"commit-ci"}},
	} {
		var r Evidence
		if err := readLocalJSON(root, gate.path, &r); err != nil {
			return "", fmt.Errorf("%s: %w", gate.mode, err)
		}
		gate.checks = append(gate.checks, "architecture", "repository-hygiene", "knowledge", "source-stable")
		if err := requiredReport(r, gate.mode, digest, contract.BoundAt, gate.checks); err != nil {
			return "", err
		}
		if gate.mode == "full" {
			if !fullPass(r) {
				return "", fmt.Errorf("incomplete full evidence")
			}
			fullID = r.RunID
		}
		if gate.mode == "full" || gate.mode == "acceptance" {
			if err := verifyNamedResults(root, gate.mode, r.Tests); err != nil {
				return "", err
			}
			allTests = append(allTests, r.Tests...)
		}
		if gate.mode == "cloud" {
			revision, err := command("git", "-C", root, "rev-parse", "HEAD")
			if err != nil || r.Revision != strings.TrimSpace(revision) {
				return "", fmt.Errorf("cloud evidence must match current HEAD")
			}
			status, err := command("git", "-C", root, "status", "--porcelain")
			if err != nil || strings.TrimSpace(status) != "" {
				return "", fmt.Errorf("finish requires published, clean source")
			}
		}
	}
	passed := map[string]bool{}
	for _, result := range allTests {
		passed[result.ID] = result.Status == "pass"
	}
	for _, criterion := range contract.Criteria {
		for _, id := range criterion.Tests {
			if !passed[id] {
				return "", fmt.Errorf("criterion %s lacks executed passing test %s", criterion.ID, id)
			}
		}
	}
	return fullID, nil
}

func validateDataScale(root string, started time.Time) (string, error) {
	path := filepath.Join(root, ".cache/harness/family-load.json")
	info, err := os.Stat(path)
	if err != nil || info.ModTime().Before(started) {
		return "", fmt.Errorf("missing fresh representative workload evidence")
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	var m map[string]any
	if err = json.Unmarshal(raw, &m); err != nil {
		return "", err
	}
	number := func(key string) float64 { n, _ := m[key].(float64); return n }
	for _, key := range []string{"households", "concurrent_clients", "requested_duration_seconds", "completed_cycles", "request_count", "errors", "invariant_failures", "p95_ms"} {
		if _, ok := m[key].(float64); !ok {
			return "", fmt.Errorf("workload metric missing: %s", key)
		}
	}
	if number("households") < 10 || number("concurrent_clients") < 20 || number("requested_duration_seconds") < 30 || number("completed_cycles") < 100 || number("request_count") < 700 || number("errors") != 0 || number("invariant_failures") != 0 || number("p95_ms") > 2000 {
		return "", fmt.Errorf("representative workload failed size, correctness or latency threshold")
	}
	return string(raw), nil
}
