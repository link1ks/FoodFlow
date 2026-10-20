package main

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

const businessPath = "docs/quality/business.json"

// Requirements are reviewed assertions, not automatically inferred semantics.
type BusinessRequirement struct {
	ID           string   `json:"id"`
	Context      string   `json:"context"`
	Guide        string   `json:"guide"`
	Sources      []string `json:"sources"`
	ReviewDigest string   `json:"review_digest"`
	Browser      []string `json:"browser"`
	Data         string   `json:"data"`
	Limits       string   `json:"limits"`
}

type TestBinding struct {
	ID           string   `json:"id"`
	Requirements []string `json:"requirements"`
}

type BusinessMap struct {
	Version      int                   `json:"version"`
	Requirements []BusinessRequirement `json:"requirements"`
	Tests        []TestBinding         `json:"tests"`
}

func readBusiness(root string) (BusinessMap, error) {
	var b BusinessMap
	raw, err := os.ReadFile(filepath.Join(root, businessPath))
	if err == nil {
		err = json.Unmarshal(raw, &b)
	}
	return b, err
}

func repositoryFile(root, path string) (string, error) {
	clean := filepath.Clean(path)
	if path == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) || strings.Contains(path, `\`) {
		return "", fmt.Errorf("expected repository-relative forward-slash path")
	}
	full := filepath.Join(root, clean)
	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		return "", err
	}
	base, err := filepath.Abs(root)
	if err != nil {
		return "", err
	}
	resolved, err = filepath.Abs(resolved)
	if err != nil {
		return "", err
	}
	rel, err := filepath.Rel(base, resolved)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", fmt.Errorf("path escapes repository")
	}
	return full, nil
}

func reviewDigest(root string, r BusinessRequirement) (string, error) {
	paths := append([]string{r.Guide}, r.Sources...)
	sort.Strings(paths)
	h := sha256.New()
	for _, path := range paths {
		full, err := repositoryFile(root, path)
		if err != nil {
			return "", err
		}
		err = filepath.WalkDir(full, func(file string, entry os.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if entry.Type()&os.ModeSymlink != 0 {
				return fmt.Errorf("review sources cannot contain links")
			}
			if entry.IsDir() {
				return nil
			}
			raw, err := os.ReadFile(file)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, file)
			fmt.Fprintf(h, "%s\x00%s\x00", filepath.ToSlash(rel), strings.ReplaceAll(string(raw), "\r\n", "\n"))
			return nil
		})
		if err != nil {
			return "", err
		}
	}
	// Altering the asserted context/data/limitations also requires review.
	fmt.Fprintf(h, "%s\x00%s\x00%s", r.Context, r.Data, r.Limits)
	return hex.EncodeToString(h.Sum(nil)), nil
}

var jsTest = regexp.MustCompile(`(?m)^\s*(?:test|it)\(\s*["']([^"'\r\n]+)["']`)
var unsupportedJSTest = regexp.MustCompile(`(?m)^\s*(?:test|it)\s*\.`)

// Named registrations are the mapping unit; subtests/data rows inherit their
// parent's requirement. Helpers and benchmarks are not executable acceptance.
func discoverTests(root string) (map[string]string, error) {
	tests := map[string]string{}
	for _, dir := range []string{"cmd", "internal", "web/src", "web/e2e", "tests"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			add := func(name, kind string) error {
				id := rel + "#" + name
				if tests[id] != "" {
					return fmt.Errorf("duplicate test identity: %s", id)
				}
				tests[id] = kind
				return nil
			}
			if strings.HasSuffix(path, "_test.go") {
				f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
				if err != nil {
					return err
				}
				for _, decl := range f.Decls {
					fn, ok := decl.(*ast.FuncDecl)
					if !ok || fn.Recv != nil || !(strings.HasPrefix(fn.Name.Name, "Test") || strings.HasPrefix(fn.Name.Name, "Fuzz")) || fn.Name.Name == "TestMain" {
						continue
					}
					if err := add(fn.Name.Name, "go"); err != nil {
						return err
					}
				}
			} else if strings.HasSuffix(path, ".test.ts") || strings.HasSuffix(path, ".test.tsx") || strings.HasSuffix(path, ".spec.ts") || strings.HasSuffix(path, ".spec.tsx") {
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				// Hooks are allowed. Modified/dynamic registrations must first get
				// an inventory adapter, rather than silently losing coverage.
				clean := regexp.MustCompile(`(?m)^\s*test\.(?:beforeEach|afterEach|beforeAll|afterAll|describe|use|setTimeout)\b.*`).ReplaceAllString(string(raw), "")
				if unsupportedJSTest.MatchString(clean) {
					return fmt.Errorf("unsupported test registration in %s; extend inventory adapter", rel)
				}
				matches := jsTest.FindAllStringSubmatch(string(raw), -1)
				if len(matches) == 0 {
					return fmt.Errorf("test file has no discoverable named tests: %s", rel)
				}
				kind := "web"
				if strings.HasPrefix(rel, "web/e2e/") {
					kind = "browser"
				}
				for _, match := range matches {
					if err := add(match[1], kind); err != nil {
						return err
					}
				}
			} else if dir == "tests" && strings.HasSuffix(path, ".ps1") && !strings.HasSuffix(path, "-helper.ps1") {
				return add("script", "script")
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	return tests, nil
}

func business(root string, update bool) error {
	b, err := readBusiness(root)
	if err != nil {
		return err
	}
	tests, err := discoverTests(root)
	if err != nil {
		return err
	}
	if b.Version != 1 || len(b.Requirements) == 0 {
		return fmt.Errorf("invalid business map")
	}
	requirements := map[string]bool{}
	for i := range b.Requirements {
		r := &b.Requirements[i]
		if r.ID == "" || requirements[r.ID] || r.Context == "" || r.Data == "" || r.Limits == "" || len(r.Sources) == 0 || len(r.Browser) == 0 {
			return fmt.Errorf("incomplete/duplicate business requirement: %s", r.ID)
		}
		requirements[r.ID] = true
		digest, err := reviewDigest(root, *r)
		if err != nil {
			return fmt.Errorf("%s context: %w", r.ID, err)
		}
		if update {
			r.ReviewDigest = digest
		} else if r.ReviewDigest != digest {
			return fmt.Errorf("%s context changed: review guide, requirements and tests, then run -mode business -update", r.ID)
		}
		for _, id := range r.Browser {
			if tests[id] != "browser" {
				return fmt.Errorf("%s missing browser recheck %s", r.ID, id)
			}
		}
	}
	bound := map[string]bool{}
	used := map[string]bool{}
	for _, binding := range b.Tests {
		if tests[binding.ID] == "" || bound[binding.ID] || len(binding.Requirements) == 0 {
			return fmt.Errorf("unknown/duplicate/unbound test: %s", binding.ID)
		}
		bound[binding.ID] = true
		for _, id := range binding.Requirements {
			if !requirements[id] {
				return fmt.Errorf("test %s references unknown requirement %s", binding.ID, id)
			}
			used[id] = true
		}
	}
	for id := range tests {
		if !bound[id] {
			return fmt.Errorf("test needs explicit business binding: %s", id)
		}
	}
	for id := range requirements {
		if !used[id] {
			return fmt.Errorf("requirement has no tests: %s", id)
		}
	}
	// Do not let a newly added implementation file evade context review simply
	// because no existing requirement happened to list it.
	if err := sourceCoverage(root, b); err != nil {
		return err
	}
	if update {
		return saveJSON(filepath.Join(root, businessPath), b)
	}
	return nil
}

func sourceCoverage(root string, b BusinessMap) error {
	for _, dir := range []string{"internal", "cmd", "web/src", "sql"} {
		err := filepath.WalkDir(filepath.Join(root, dir), func(path string, entry os.DirEntry, err error) error {
			if err != nil || entry.IsDir() {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			rel = filepath.ToSlash(rel)
			if strings.HasSuffix(rel, "_test.go") || strings.Contains(rel, ".test.") {
				return nil // Named tests have their own exhaustive mapping.
			}
			for _, r := range b.Requirements {
				for _, source := range r.Sources {
					if rel == source || strings.HasPrefix(rel, source+"/") {
						return nil
					}
				}
			}
			return fmt.Errorf("implementation needs reviewed business context: %s", rel)
		})
		if err != nil {
			return err
		}
	}
	return nil
}
