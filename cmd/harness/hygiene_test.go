package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

func TestRepositoryPathsKeepFixturesAndRejectPrivateArtifacts(t *testing.T) {
	for _, file := range []string{".env.example", "sql/schema/001_init.sql", "internal/market/testdata/report.html", "internal/engine/scheduler/testdata/fuzz/FuzzSchedule/seed", "web/public/ingredient-photos/tomato.webp", "docs/validation/learning-release.json", "docs/exec-plans/platform.md", "web/pnpm-lock.yaml"} {
		if reason := repositoryPathViolation(file); reason != "" {
			t.Errorf("valid delivery file rejected: %s (%s)", file, reason)
		}
	}
	for _, file := range []string{".env", ".env.production", "learning.env", ".cache/learning-backups/manifest.json", "data/images/family.jpg", ".idea/workspace.xml", "web/node_modules/package/index.js", "web/dist/index.html", "web/test-results/result.json", "web/coverage/report.json", "reports/local/auth.json", "coverage.out", "backup.dump", "access.log", "server.key", "certificate.PEM", "web/tsconfig.tsbuildinfo", "work.tmp", ".AWS/credentials"} {
		if repositoryPathViolation(file) == "" {
			t.Errorf("private/generated file accepted: %s", file)
		}
	}
}

func TestRepositoryHygieneCatchesForcedIgnoredFileWithoutReadingSecrets(t *testing.T) {
	root := t.TempDir()
	git := func(args ...string) {
		t.Helper()
		if raw, err := exec.Command("git", append([]string{"-C", root}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("fixture Git command failed: %v (%s)", err, raw)
		}
	}
	git("init", "--quiet")
	writeFixture(t, root, ".gitignore", ".env\n.cache/\n")
	writeFixture(t, root, ".env.example", "KEY=\n")
	writeFixture(t, root, ".env", "KEY=never-export-fixture-secret\n")
	if err := repositoryHygiene(root); err != nil {
		t.Fatalf("ignored private configuration affected public inventory: %v", err)
	}
	git("add", "--force", ".env")
	err := repositoryHygiene(root)
	if err == nil || !strings.Contains(err.Error(), "forbidden") || strings.Contains(err.Error(), "never-export-fixture-secret") || strings.Contains(err.Error(), ".env") {
		t.Fatalf("forced private configuration not rejected safely: %v", err)
	}
	if err := os.Remove(filepath.Join(root, ".env")); err != nil {
		t.Fatal(err)
	}
	if err := repositoryHygiene(root); err == nil {
		t.Fatal("private configuration remains staged even when its worktree file was deleted")
	}
	git("rm", "--cached", ".env")
	writeFixture(t, root, "untracked.dump", "private-backup-fixture")
	if err := repositoryHygiene(root); err == nil {
		t.Fatal("untracked private backup would pass the public file check")
	}
}
