package main

import (
	"os"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

// Exercise the event policy from the real YAML so broadening push back to all
// branches reproduces the duplicate branch-push/PR verification regression.
func TestCIEventPolicyAvoidsDuplicateBranchRuns(t *testing.T) {
	b, err := os.ReadFile("../../.github/workflows/ci.yml")
	if err != nil {
		t.Fatal(err)
	}
	var workflow struct {
		On          any `yaml:"on"`
		Concurrency struct {
			Group  string `yaml:"group"`
			Cancel bool   `yaml:"cancel-in-progress"`
		} `yaml:"concurrency"`
	}
	if err = yaml.Unmarshal(b, &workflow); err != nil {
		t.Fatal(err)
	}
	handles := func(event, ref string) bool {
		if events, ok := workflow.On.([]any); ok {
			for _, v := range events {
				if v == event {
					return true
				}
			}
			return false
		}
		events, ok := workflow.On.(map[string]any)
		if !ok {
			t.Fatal("unsupported workflow trigger shape")
			return false
		}
		v, ok := events[event]
		if !ok {
			return false
		}
		filter, ok := v.(map[string]any)
		if !ok {
			return true
		}
		if event == "push" && strings.HasPrefix(ref, "refs/tags/") {
			_, tags := filter["tags"]
			return tags
		}
		branches, ok := filter["branches"].([]any)
		if !ok {
			return true
		}
		branch := strings.TrimPrefix(ref, "refs/heads/")
		for _, v := range branches {
			if v == branch {
				return true
			}
		}
		return false
	}
	for _, tc := range []struct {
		event, ref string
		want       bool
	}{
		{"push", "refs/heads/main", true},
		{"push", "refs/heads/codex/intake-scope-semantics-20261016", false},
		{"push", "refs/heads/feature/example", false},
		{"push", "refs/tags/v1.2.3", false},
		{"pull_request", "refs/pull/2/merge", true},
		{"workflow_dispatch", "refs/heads/main", true},
		{"workflow_dispatch", "refs/heads/feature/example", true},
	} {
		if got := handles(tc.event, tc.ref); got != tc.want {
			t.Errorf("%s %s: runs=%v want %v", tc.event, tc.ref, got, tc.want)
		}
	}
	if !workflow.Concurrency.Cancel {
		t.Error("new revisions must supersede older runs")
	}
	if !strings.Contains(workflow.Concurrency.Group, "github.workflow") || !strings.Contains(workflow.Concurrency.Group, "github.event.pull_request.number || github.ref") {
		t.Error("concurrency must isolate workflows/PRs and provide branch fallback")
	}
}
