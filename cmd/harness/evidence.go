package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
)

type Evidence struct {
	Version             int               `json:"version"`
	RunID               string            `json:"run_id"`
	RecordedAt          time.Time         `json:"recorded_at"`
	Revision            string            `json:"revision"`
	WorktreeDirty       bool              `json:"worktree_dirty"`
	SourceDigest        string            `json:"source_digest"`
	Toolchain           map[string]string `json:"toolchain"`
	Mode                string            `json:"mode"`
	IntegrationRequired bool              `json:"integration_required"`
	Checks              []Check           `json:"checks"`
}

func runID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		panic(err)
	}
	return hex.EncodeToString(id[:])
}

// Digest versioned inputs and new source files, without reading .env or logs.
func sourceDigest(root string) (string, error) {
	files := []string{"go.mod", "go.sum", "sqlc.yaml", ".env.example", ".dockerignore", ".gitignore", "AGENTS.md", "README.md", "ARCHITECTURE.md", "openapi.yaml", "Dockerfile", "compose.yaml", "compose.acceptance.yaml", "compose.platform.yaml", ".github/workflows/ci.yml", "web/package.json", "web/pnpm-lock.yaml", "web/Dockerfile", "web/.dockerignore", "web/nginx.conf", "web/playwright.config.ts", "web/tsconfig.json", "web/vite.config.ts", "web/index.html", "web/pnpm-workspace.yaml", "docs/HARNESS.md", "docs/PLATFORM.md", knowledgePath}
	for _, dir := range []string{"cmd", "internal", "sql", "scripts", "tests", "web/src", "web/e2e"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if !d.IsDir() {
				rel, _ := filepath.Rel(root, path)
				files = append(files, filepath.ToSlash(rel))
			}
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	sort.Strings(files)
	h := sha256.New()
	for _, file := range files {
		raw, err := os.ReadFile(filepath.Join(root, file))
		if os.IsNotExist(err) { // Optional deployment/configuration files still affect the hash when added.
			continue
		}
		if err != nil {
			return "", err
		}
		fmt.Fprintf(h, "%s\x00", filepath.ToSlash(file))
		// Stable across Git's Windows line-ending conversion.
		h.Write([]byte(strings.ReplaceAll(string(raw), "\r\n", "\n")))
		h.Write([]byte{0})
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func saveJSON(path string, value any) error {
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(raw, '\n'), 0600)
}

func saveEvidence(out string, report Evidence) error {
	if err := saveJSON(out, report); err != nil {
		return err
	}
	return saveJSON(filepath.Join(".cache/harness/runs", report.RunID+".json"), report)
}

type TaskRecord struct {
	ID           string    `json:"id"`
	Kind         string    `json:"kind"`
	Baseline     string    `json:"baseline"`
	StartedAt    time.Time `json:"started_at"`
	FinishedAt   time.Time `json:"finished_at,omitempty"`
	Outcome      string    `json:"outcome,omitempty"`
	EvidenceID   string    `json:"evidence_run_id,omitempty"`
	ElapsedMS    int64     `json:"wall_clock_elapsed_ms,omitempty"`
	SourceDigest string    `json:"source_digest,omitempty"`
}

var taskName = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,63}$`)

func recordTask(root, id, action, kind, baseline, outcome, evidence string) error {
	if !taskName.MatchString(id) {
		return fmt.Errorf("task ID must be lowercase letters, digits and hyphens (max 64)")
	}
	dir := filepath.Join(root, ".cache/harness/tasks")
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	startPath := filepath.Join(dir, id+".start.json")
	finishPath := filepath.Join(dir, id+".finish.json")
	if action == "start" {
		if kind != "feature" && kind != "regression" && kind != "maintenance" || baseline != "reproduced" && baseline != "unknown" {
			return fmt.Errorf("kind must be feature/regression/maintenance; baseline must be reproduced/unknown")
		}
		return writeExclusive(startPath, TaskRecord{ID: id, Kind: kind, Baseline: baseline, StartedAt: time.Now().UTC()})
	}
	if action != "finish" || outcome != "passed" && outcome != "failed" && outcome != "blocked" {
		return fmt.Errorf("action must be start/finish; finish outcome must be passed/failed/blocked")
	}
	raw, err := os.ReadFile(startPath)
	if err != nil {
		return err
	}
	var record TaskRecord
	if err = json.Unmarshal(raw, &record); err != nil {
		return err
	}
	if outcome == "passed" {
		path := filepath.Clean(evidence)
		if !strings.HasPrefix(filepath.ToSlash(path), ".cache/harness/") || filepath.IsAbs(path) {
			return fmt.Errorf("passed task requires a local .cache/harness evidence file")
		}
		raw, err := os.ReadFile(filepath.Join(root, path))
		if err != nil {
			return err
		}
		var report Evidence
		if err := json.Unmarshal(raw, &report); err != nil {
			return err
		}
		digest, err := sourceDigest(root)
		if err != nil {
			return err
		}
		if !fullPass(report) || report.SourceDigest != digest || report.RecordedAt.Before(record.StartedAt) {
			return fmt.Errorf("evidence must be a complete passing full run after task start, matching current source")
		}
		record.EvidenceID, record.SourceDigest = report.RunID, digest
	}
	record.FinishedAt, record.Outcome = time.Now().UTC(), outcome
	record.ElapsedMS = record.FinishedAt.Sub(record.StartedAt).Milliseconds()
	return writeExclusive(finishPath, record)
}

func fullPass(report Evidence) bool {
	if report.Version != 1 || report.RunID == "" || report.Mode != "full" || !report.IntegrationRequired {
		return false
	}
	required := map[string]bool{"architecture": false, "knowledge": false, "format": false, "vet": false, "go-tests": false, "web-test": false, "web-build": false, "source-stable": false}
	for _, check := range report.Checks {
		if !check.Passed {
			return false
		}
		if _, ok := required[check.Name]; ok {
			required[check.Name] = true
		}
	}
	for _, passed := range required {
		if !passed {
			return false
		}
	}
	return true
}

func writeExclusive(path string, value any) error {
	raw, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = f.Write(append(raw, '\n'))
	return err
}

func historySummary(root string) (any, error) {
	files, err := filepath.Glob(filepath.Join(root, ".cache/harness/runs/*.json"))
	if err != nil {
		return nil, err
	}
	type totals struct {
		Runs    int   `json:"runs"`
		Passed  int   `json:"passed"`
		TotalMS int64 `json:"total_check_duration_ms"`
	}
	byMode := map[string]*totals{}
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var report Evidence
		if err := json.Unmarshal(raw, &report); err != nil || report.Version != 1 || report.RunID == "" {
			return nil, fmt.Errorf("invalid evidence history: %s", filepath.Base(file))
		}
		if byMode[report.Mode] == nil {
			byMode[report.Mode] = new(totals)
		}
		item := byMode[report.Mode]
		item.Runs++
		passed := len(report.Checks) > 0
		for _, check := range report.Checks {
			passed = passed && check.Passed
			item.TotalMS += check.DurationMS
		}
		if passed {
			item.Passed++
		}
	}
	files, err = filepath.Glob(filepath.Join(root, ".cache/harness/tasks/*.finish.json"))
	if err != nil {
		return nil, err
	}
	outcomes, kinds := map[string]int{}, map[string]int{}
	var elapsed int64
	for _, file := range files {
		raw, err := os.ReadFile(file)
		if err != nil {
			return nil, err
		}
		var record TaskRecord
		if err := json.Unmarshal(raw, &record); err != nil {
			return nil, err
		}
		outcomes[record.Outcome]++
		kinds[record.Kind]++
		elapsed += record.ElapsedMS
	}
	return map[string]any{"verification_by_mode": byMode, "completed_task_outcomes": outcomes, "completed_task_kinds": kinds, "total_task_wall_clock_ms": elapsed, "interpretation": "Observed verification runs and explicitly recorded tasks only. Wall-clock time includes waiting and interruptions. This is not a controlled measurement of AI productivity, user bug rate or human effort; repeated runs are not independent tasks."}, nil
}
