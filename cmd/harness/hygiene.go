package main

import (
	"context"
	"fmt"
	"os/exec"
	"path"
	"strings"
	"time"
)

// Inspect the public file set, including newly added files. Do not read file
// contents or enumerate ignored private data. Forced additions remain visible.
func repositoryHygiene(root string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "git", "-C", root, "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	raw, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("cannot inspect Git file inventory")
	}
	for _, file := range strings.Split(string(raw), "\x00") {
		if file == "" {
			continue
		}
		// Cached names stay public until removed from the index. Deleting only
		// the worktree file must not hide a staged private blob.
		if reason := repositoryPathViolation(file); reason != "" {
			// Never include path names: private filenames can carry account data.
			return fmt.Errorf("repository contains forbidden %s; review staged/untracked files", reason)
		}
	}
	return nil
}

func repositoryPathViolation(file string) string {
	file = strings.ToLower(strings.ReplaceAll(file, "\\", "/"))
	for _, segment := range strings.Split(file, "/") {
		switch segment {
		case ".cache", ".idea", ".vscode", ".aws", ".ssh", "node_modules":
			return "private or dependency directory"
		case ".env.example":
			// The reviewed public configuration template is versioned.
		default:
			if segment == ".env" || strings.HasPrefix(segment, ".env.") {
				return "local environment configuration"
			}
		}
	}
	for _, prefix := range []string{"data/", "reports/local/", "web/dist/", "web/coverage/", "web/test-results/", "web/playwright-report/"} {
		if strings.HasPrefix(file, prefix) {
			return "local data or generated output"
		}
	}
	base := path.Base(file)
	if base == "coverage.out" || strings.HasSuffix(base, ".tsbuildinfo") {
		return "generated output"
	}
	switch path.Ext(base) {
	case ".env":
		return "local environment configuration"
	case ".log", ".dump", ".exe", ".swp", ".swo", ".tmp", ".bak":
		return "temporary file, backup or binary output"
	case ".key", ".pem", ".p12", ".pfx":
		return "credential or certificate file"
	}
	return ""
}
