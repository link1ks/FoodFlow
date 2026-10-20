package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type TestOutcome struct {
	ID      string `json:"id"`
	Project string `json:"project,omitempty"`
	Status  string `json:"status"`
}

type goResults struct {
	pending []byte
	results map[string]string
}

func (g *goResults) Write(p []byte) (int, error) {
	n := len(p)
	g.pending = append(g.pending, p...)
	for {
		end := bytes.IndexByte(g.pending, '\n')
		if end < 0 {
			break
		}
		line := g.pending[:end]
		var event struct{ Action, Package, Test string }
		if json.Unmarshal(line, &event) == nil && event.Test != "" && !strings.Contains(event.Test, "/") && (event.Action == "pass" || event.Action == "fail" || event.Action == "skip") {
			if g.results == nil {
				g.results = map[string]string{}
			}
			g.results[strings.TrimPrefix(event.Package, "foodflow/")+"#"+event.Test] = event.Action
		}
		g.pending = g.pending[end+1:]
	}
	if len(g.pending) > 1<<20 {
		return n, fmt.Errorf("test event line exceeds bound")
	}
	return n, nil
}

func (g *goResults) outcomes(root string) ([]TestOutcome, error) {
	inventory, err := discoverTests(root)
	if err != nil {
		return nil, err
	}
	var results []TestOutcome
	for id, kind := range inventory {
		if kind != "go" {
			continue
		}
		path, name, _ := strings.Cut(id, "#")
		status := g.results[filepath.ToSlash(filepath.Dir(path))+"#"+name]
		if status == "" {
			status = "missing"
		}
		results = append(results, TestOutcome{ID: id, Status: status})
	}
	return results, nil
}

func webResults(root, path string) ([]TestOutcome, error) {
	raw, err := os.ReadFile(filepath.Join(root, path))
	if err != nil {
		return nil, err
	}
	var report struct {
		TestResults []struct {
			Name             string
			AssertionResults []struct{ Title, Status string }
		}
	}
	if err := json.Unmarshal(raw, &report); err != nil {
		return nil, err
	}
	abs, _ := filepath.Abs(root)
	var results []TestOutcome
	for _, suite := range report.TestResults {
		rel, err := filepath.Rel(abs, suite.Name)
		if err != nil {
			return nil, err
		}
		for _, test := range suite.AssertionResults {
			status := test.Status
			if status == "passed" {
				status = "pass"
			}
			results = append(results, TestOutcome{ID: filepath.ToSlash(rel) + "#" + test.Title, Status: status})
		}
	}
	return results, nil
}

type browserSuite struct {
	File   string         `json:"file"`
	Suites []browserSuite `json:"suites"`
	Specs  []struct {
		File  string `json:"file"`
		Title string `json:"title"`
		Tests []struct {
			ProjectName string `json:"projectName"`
			Status      string `json:"status"`
			Results     []struct {
				Status string `json:"status"`
			} `json:"results"`
		} `json:"tests"`
	} `json:"specs"`
}

func browserResults(root string) ([]TestOutcome, error) {
	raw, err := os.ReadFile(filepath.Join(root, ".cache/harness/browser.json"))
	if err != nil {
		return nil, err
	}
	var report struct {
		Suites []browserSuite `json:"suites"`
	}
	if err = json.Unmarshal(raw, &report); err != nil {
		return nil, err
	}
	var results []TestOutcome
	var visit func(browserSuite)
	visit = func(s browserSuite) {
		for _, spec := range s.Specs {
			path := filepath.ToSlash(spec.File)
			if !strings.HasPrefix(path, "web/e2e/") {
				path = "web/e2e/" + path
			}
			for _, test := range spec.Tests {
				status := "fail"
				if test.Status == "expected" && len(test.Results) == 1 && test.Results[0].Status == "passed" {
					status = "pass"
				}
				results = append(results, TestOutcome{ID: path + "#" + spec.Title, Project: test.ProjectName, Status: status})
			}
		}
		for _, child := range s.Suites {
			visit(child)
		}
	}
	for _, s := range report.Suites {
		visit(s)
	}
	return results, nil
}

func verifyNamedResults(root, mode string, results []TestOutcome) error {
	inventory, err := discoverTests(root)
	if err != nil {
		return err
	}
	actual := map[string]string{}
	for _, result := range results {
		key := result.ID + "#" + result.Project
		if actual[key] != "" || inventory[result.ID] == "" || result.Status != "pass" {
			return fmt.Errorf("duplicate, unknown or unsuccessful named test: %s", result.ID)
		}
		actual[key] = result.Status
	}
	for id, kind := range inventory {
		projects := []string{}
		if mode == "full" && (kind == "go" || kind == "web") {
			projects = []string{""}
		}
		if mode == "acceptance" && kind == "browser" {
			projects = []string{"desktop", "mobile"}
		}
		for _, project := range projects {
			if actual[id+"#"+project] != "pass" {
				return fmt.Errorf("missing passed result for %s (%s)", id, project)
			}
		}
	}
	return nil
}
