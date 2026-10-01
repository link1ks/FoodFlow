package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func knowledgeFixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	for _, path := range []string{"internal/app/app.go", "internal/insights/http.go", "internal/events/stock.go", "go.mod", "web/package.json", ".env.example", "compose.acceptance.yaml", "compose.platform.yaml", "sql/schema/028_stock_outbox.sql"} {
		raw, err := os.ReadFile(filepath.Join("../..", path))
		if err != nil {
			t.Fatal(err)
		}
		writeFixture(t, root, path, string(raw))
	}
	for _, path := range []string{"README.md", "AGENTS.md", "ARCHITECTURE.md", "docs/HARNESS.md"} {
		writeFixture(t, root, path, "# Fixture\n")
	}
	for _, dir := range []string{"cmd", "scripts", "tests", "web/src", "web/e2e", "web/public", "docs/quality"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func writeFixture(t *testing.T, root, path, content string) {
	t.Helper()
	path = filepath.Join(root, path)
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
}

func TestKnowledgeRejectsRouteAndConfigDrift(t *testing.T) {
	for _, changed := range []string{"route", "config"} {
		t.Run(changed, func(t *testing.T) {
			root := knowledgeFixture(t)
			if err := knowledge(root, true); err != nil {
				t.Fatal(err)
			}
			if err := knowledge(root, false); err != nil {
				t.Fatal(err)
			}
			if changed == "route" {
				raw, _ := os.ReadFile(filepath.Join(root, "internal/app/app.go"))
				writeFixture(t, root, "internal/app/app.go", strings.Replace(string(raw), `v.Group("/households/:household")`, `v.Group("/families/:household")`, 1))
			} else {
				writeFixture(t, root, ".env.example", "NEW_KEY=never-export-this-value\n")
			}
			if err := knowledge(root, false); err == nil || !strings.Contains(err.Error(), "drift") {
				t.Fatalf("source drift accepted: %v", err)
			}
			if err := knowledge(root, true); err != nil {
				t.Fatal(err)
			}
			raw, _ := os.ReadFile(filepath.Join(root, knowledgePath))
			if strings.Contains(string(raw), "never-export-this-value") {
				t.Fatal("configuration values leaked into inventory")
			}
		})
	}
}

func TestKnowledgeRejectsBrokenAndEscapingLinks(t *testing.T) {
	for _, target := range []string{"docs/missing.md", "../outside.md"} {
		root := knowledgeFixture(t)
		writeFixture(t, root, "README.md", "[bad]("+target+")\n")
		if err := knowledge(root, true); err == nil {
			t.Fatalf("accepted bad link %s", target)
		}
	}
}

func TestTaskRequiresFreshFullEvidenceAndRejectsOverwrite(t *testing.T) {
	root := knowledgeFixture(t)
	if err := recordTask(root, "repair", "start", "regression", "reproduced", "", ""); err != nil {
		t.Fatal(err)
	}
	if err := recordTask(root, "repair", "start", "regression", "reproduced", "", ""); err == nil {
		t.Fatal("duplicate start overwrote original time")
	}
	digest, err := sourceDigest(root)
	if err != nil {
		t.Fatal(err)
	}
	report := Evidence{Version: 1, RunID: "test", Mode: "full", IntegrationRequired: true, SourceDigest: digest, RecordedAt: time.Now().UTC()}
	for _, name := range []string{"architecture", "knowledge", "quality-map", "dbgen-regression", "dbgen-check", "format", "vet", "go-tests", "web-api-check", "web-test", "web-build", "source-stable"} {
		report.Checks = append(report.Checks, Check{Name: name, Passed: true})
	}
	evidence := ".cache/harness/test.json"
	write := func() {
		if err := saveJSON(filepath.Join(root, evidence), report); err != nil {
			t.Fatal(err)
		}
	}
	report.Mode = "fast"
	write()
	if err := recordTask(root, "repair", "finish", "", "", "passed", evidence); err == nil {
		t.Fatal("fast evidence accepted as full verification")
	}
	report.Mode = "full"
	report.RecordedAt = time.Time{}
	write()
	if err := recordTask(root, "repair", "finish", "", "", "passed", evidence); err == nil {
		t.Fatal("old evidence accepted")
	}
	report.RecordedAt = time.Now().UTC()
	report.SourceDigest = "different-source"
	write()
	if err := recordTask(root, "repair", "finish", "", "", "passed", evidence); err == nil {
		t.Fatal("different source accepted")
	}
	report.SourceDigest = digest
	write()
	if err := recordTask(root, "repair", "finish", "", "", "passed", evidence); err != nil {
		t.Fatal(err)
	}
	if err := recordTask(root, "repair", "finish", "", "", "failed", ""); err == nil {
		t.Fatal("task outcome could be overwritten")
	}
}
