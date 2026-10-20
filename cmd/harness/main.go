// Harness is the repository's executable feedback loop, shared by agents and CI.
package main

import (
	"context"
	"flag"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Check struct {
	Name       string `json:"name"`
	Passed     bool   `json:"passed"`
	DurationMS int64  `json:"duration_ms"`
	Detail     string `json:"detail,omitempty"`
}

func architecture(root string) error {
	required := []string{"AGENTS.md", "docs/HARNESS.md", "docs/PLATFORM.md", "docs/exec-plans/platform.md", "api/openapi.yaml"}
	for _, p := range required {
		if _, err := os.Stat(filepath.Join(root, p)); err != nil {
			return fmt.Errorf("required knowledge entry missing: %s", p)
		}
	}
	return filepath.WalkDir(filepath.Join(root, "internal"), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, _ := filepath.Rel(root, path)
		rel = filepath.ToSlash(rel)
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, 0)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			value, _ := strconv.Unquote(imp.Path.Value)
			if msg := importViolation(rel, value); msg != "" {
				return fmt.Errorf("%s imports %s: %s", rel, value, msg)
			}
		}
		if strings.HasPrefix(rel, "internal/app/") {
			if msg := pooledObjectContextViolation(f); msg != "" {
				return fmt.Errorf("%s: %s", rel, msg)
			}
		}
		return nil
	})
}
func importViolation(path, value string) string {
	if strings.HasPrefix(path, "internal/engine/") || strings.HasPrefix(path, "internal/core/") {
		if strings.HasPrefix(value, "github.com/") || value == "net" || strings.HasPrefix(value, "net/") || value == "database/sql" || value == "os" || strings.HasPrefix(value, "os/") || value == "syscall" || strings.HasPrefix(value, "foodflow/internal/") && value != "foodflow/internal/core" && !strings.HasPrefix(value, "foodflow/internal/engine/") {
			return "pure computation cannot depend on I/O services"
		}
	}
	if strings.HasPrefix(path, "internal/insights/") && (strings.HasPrefix(value, "foodflow/internal/") && value != "foodflow/internal/events") {
		return "statistics may only share versioned event contracts"
	}
	if strings.HasPrefix(path, "internal/app/") && (strings.HasPrefix(value, "foodflow/internal/insights") || strings.Contains(value, "franz-go")) {
		return "API cannot query projection internals or synchronously publish Kafka"
	}
	return ""
}

// Preserve no user-controlled shell execution; all subprocess argument vectors
// are fixed in code and environment is sanitized to prevent touching live data.
func command(name string, args ...string) (string, error) {
	return observedCommand(nil, name, args...)
}

func observedCommand(observer io.Writer, name string, args ...string) (string, error) {
	if name == "pwsh" {
		args = append([]string{"-NoProfile", "-NonInteractive"}, args...)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()
	cmd := exec.CommandContext(ctx, name, args...)
	blocked := map[string]bool{"TRUSTED_PROXIES": true, "TEST_DATABASE_URL": true, "COMMUNITY_REVIEW_MODEL_ENDPOINT": true, "COMMUNITY_REVIEW_MODEL_NAME": true, "COMMUNITY_REVIEW_MODEL_API_KEY": true, "MODEL_API_KEY": true, "MODEL_ENDPOINT": true, "MODEL_NAME": true, "VISION_MODEL_API_KEY": true, "VISION_MODEL_ENDPOINT": true, "VISION_MODEL_NAME": true, "REDIS_URL": true, "INSIGHTS_URL": true, "INSIGHTS_SERVICE_TOKEN": true, "AI_ALLOWANCE_ENABLED": true, "AI_MONTHLY_ALLOWANCE_CNY": true, "AI_TEXT_CALL_ALLOWANCE_CNY": true, "AI_IMAGE_CALL_ALLOWANCE_CNY": true, "AI_DAILY_CALL_LIMIT": true, "AI_CONCURRENT_CALL_LIMIT": true}
	for _, key := range []string{"SPEECH_MODEL_ENDPOINT", "SPEECH_MODEL_NAME", "SPEECH_MODEL_API_KEY", "AI_SPEECH_CALL_ALLOWANCE_CNY"} {
		blocked[key] = true
	}
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if !blocked[key] {
			cmd.Env = append(cmd.Env, entry)
		}
	}
	cmd.Env = append(cmd.Env, "REQUIRE_INTEGRATION=true")
	output := new(boundedOutput)
	cmd.Stdout, cmd.Stderr = output, output
	if observer != nil {
		cmd.Stdout = io.MultiWriter(output, observer)
	}
	err := cmd.Run()
	if ctx.Err() != nil {
		err = fmt.Errorf("verification command timed out: %w", ctx.Err())
	}
	return string(output.data), err
}

// Retain a bounded diagnostic tail even when a subprocess writes excessively.
type boundedOutput struct {
	mu   sync.Mutex
	data []byte
}

func (b *boundedOutput) Write(p []byte) (int, error) {
	b.mu.Lock()
	defer b.mu.Unlock()
	const limit = 64 << 10
	n := len(p)
	if n >= limit {
		b.data = append(b.data[:0], p[n-limit:]...)
	} else {
		if len(b.data)+n > limit {
			b.data = b.data[len(b.data)+n-limit:]
		}
		b.data = append(b.data, p...)
	}
	return n, nil
}
func main() {
	started := time.Now().UTC()
	mode := flag.String("mode", "full", "full, fast, architecture, knowledge, diagnose, acceptance, task, summary, fingerprint, or images")
	web := flag.Bool("web", true, "also test and build frontend in full mode")
	out := flag.String("out", ".cache/harness/latest.json", "evidence path")
	update := flag.Bool("update", false, "regenerate source inventory in knowledge mode")
	platformImages := flag.Bool("platform-images", false, "require relay and insights image bindings")
	skipBuild := flag.Bool("skip-build", false, "use existing acceptance images")
	task := flag.String("task", "", "stable task ID for task mode")
	action := flag.String("action", "", "start or finish")
	kind := flag.String("kind", "maintenance", "feature, regression or maintenance")
	baseline := flag.String("baseline", "unknown", "reproduced or unknown; records an operator assertion")
	outcome := flag.String("outcome", "", "passed, failed or blocked")
	evidence := flag.String("evidence", "", "passing full report for task completion")
	contract := flag.String("contract", "", "repository task contract for action bind")
	failureCategory := flag.String("failure-category", "unknown", "product, fixture, infrastructure or unknown for failed/blocked tasks")
	paid := flag.Bool("paid", false, "explicitly authorized live RAG answer; never used by full/CI")
	provider := flag.String("provider", "", "vision: explicitly reuse existing vision pair for a RAG answer")
	flag.Parse()
	var releaseLock func()
	if *mode == "full" || *mode == "fast" || *mode == "acceptance" {
		var err error
		releaseLock, err = verificationLock(".")
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		defer releaseLock()
	}
	if !strings.Contains("|full|fast|architecture|knowledge|business|diagnose|acceptance|main-site|cloud|task|summary|fingerprint|images|experience|", "|"+*mode+"|") {
		fmt.Fprintln(os.Stderr, "invalid mode")
		os.Exit(2)
	}
	if *update && *mode != "knowledge" && *mode != "business" {
		fmt.Fprintln(os.Stderr, "-update is only valid in knowledge/business mode")
		os.Exit(2)
	}
	if *mode == "experience" {
		var err error
		if *action == "answer" {
			err = answerExperience(".", *evidence, *out, *paid, *provider)
		} else {
			err = experienceCommand(".", *action, *evidence, *out)
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println("experience " + *action + " complete")
		return
	}
	if *mode == "task" || *mode == "summary" {
		var err error
		if *mode == "task" {
			if *action == "bind" {
				err = bindTask(".", *task, *contract)
			} else {
				err = recordTask(".", *task, *action, *kind, *baseline, *outcome, *evidence, *failureCategory)
			}
		} else {
			var summary any
			summary, err = historySummary(".")
			if err == nil {
				err = saveJSON(*out, summary)
			}
		}
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		fmt.Println(*mode + " recorded successfully")
		return
	}
	initialDigest, err := sourceDigest(".")
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if *mode == "fingerprint" {
		fmt.Println(initialDigest)
		return
	}
	checks := []Check{}
	var testOutcomes []TestOutcome
	run := func(name string, fn func() (string, error)) {
		start := time.Now()
		detail, err := fn()
		checks = append(checks, Check{name, err == nil, time.Since(start).Milliseconds(), detail})
		fmt.Printf("%s passed=%v\n", name, err == nil)
		if err != nil {
			fmt.Print(detail)
			fmt.Println(err)
		}
	}
	run("architecture", func() (string, error) { return "", architecture(".") })
	run("repository-hygiene", func() (string, error) { return "", repositoryHygiene(".") })
	if *mode != "architecture" {
		run("knowledge", func() (string, error) { return "", knowledge(".", *update && *mode == "knowledge") })
	}
	if *mode == "business" || *mode == "full" || *mode == "fast" || *mode == "acceptance" {
		run("business-map", func() (string, error) { return "", business(".", *update && *mode == "business") })
	}
	if *mode == "full" || *mode == "fast" || *mode == "acceptance" {
		run("quality-map", func() (string, error) { return "", quality(".") })
	}
	pnpm := "pnpm"
	if p, err := exec.LookPath("pnpm.cmd"); err == nil {
		pnpm = p
	}
	if *mode == "images" {
		run("acceptance-images", func() (string, error) { return verifyAcceptanceImages(".", initialDigest, *platformImages) })
	}
	if *mode == "diagnose" {
		run("runtime-diagnostics", func() (string, error) { return command("pwsh", "-File", "scripts/harness-diagnose.ps1") })
	}
	if *mode == "main-site" {
		run("main-site-browser", func() (string, error) { return command("node", "web/scripts/verify-main.mjs") })
	}
	if *mode == "cloud" {
		run("commit-ci", func() (string, error) { return command("pwsh", "-File", "scripts/verify-task-ci.ps1") })
	}
	if *mode == "acceptance" {
		run("read-recovery-regression", func() (string, error) { return command("pwsh", "-File", "tests/read-model-recovery.ps1") })
		if !*skipBuild {
			run("acceptance-stack", func() (string, error) { return command("pwsh", "-File", "scripts/local-acceptance.ps1", "up") })
		}
		imagesReady := false
		run("acceptance-images", func() (string, error) {
			detail, err := verifyAcceptanceImages(".", initialDigest, false)
			imagesReady = err == nil
			return detail, err
		})
		if imagesReady {
			run("platform", func() (string, error) {
				return command("pwsh", "-File", "scripts/platform-acceptance.ps1", "-SkipBuild")
			})
			run("kitchen-flow", func() (string, error) {
				return command("pwsh", "-File", "tests/smoke.ps1", "-Base", "http://127.0.0.1:18080")
			})
			run("gateway-fixture", func() (string, error) { return command("pwsh", "-File", "scripts/prepare-browser-fixture.ps1") })
			run("gateway-recovery", func() (string, error) { return command("pwsh", "-File", "tests/gateway-recovery.ps1") })
			run("browser-fixture", func() (string, error) { return command("pwsh", "-File", "scripts/prepare-browser-fixture.ps1") })
			run("browser", func() (string, error) {
				detail, err := command(pnpm, "--dir", "web", "test:e2e")
				if err == nil {
					testOutcomes, err = browserResults(".")
				}
				return detail, err
			})
			run("named-tests", func() (string, error) { return "", verifyNamedResults(".", "acceptance", testOutcomes) })
			run("runtime-diagnostics", func() (string, error) { return command("pwsh", "-File", "scripts/harness-diagnose.ps1") })
		}
	}
	if *mode == "full" || *mode == "fast" {
		run("update-regression", func() (string, error) { return command("pwsh", "-File", "tests/update-project.ps1") })
		if *mode == "full" {
			run("release-regression", func() (string, error) { return command("pwsh", "-File", "tests/release-bundle.ps1") })
			run("dbgen-regression", func() (string, error) { return command("pwsh", "-File", "tests/generated-freshness.ps1") })
			run("dbgen-check", func() (string, error) { return command("pwsh", "-File", "scripts/check-dbgen.ps1") })
		}
		run("format", func() (string, error) {
			var files []string
			for _, dir := range []string{"cmd", "internal"} {
				err := filepath.WalkDir(dir, func(path string, d os.DirEntry, err error) error {
					if err != nil {
						return err
					}
					if !d.IsDir() && strings.HasSuffix(path, ".go") {
						files = append(files, path)
					}
					return nil
				})
				if err != nil {
					return "", err
				}
			}
			result, err := command("gofmt", append([]string{"-l"}, files...)...)
			if err == nil && strings.TrimSpace(result) != "" {
				err = fmt.Errorf("run gofmt on listed files")
			}
			return result, err
		})
		run("vet", func() (string, error) { return command("go", "vet", "./...") })
		run("go-tests", func() (string, error) {
			args := []string{"test", "-json", "-count=1", "./..."}
			if *mode == "fast" {
				args = append(args, "-short")
			}
			observer := &goResults{}
			detail, err := observedCommand(observer, "go", args...)
			if err == nil {
				testOutcomes, err = observer.outcomes(".")
			}
			return detail, err
		})
		if *mode == "full" && *web {
			for _, action := range []string{"api:check", "test", "build"} {
				step := action
				run("web-"+strings.ReplaceAll(step, ":", "-"), func() (string, error) {
					if step != "test" {
						return command(pnpm, "--dir", "web", step)
					}
					detail, err := command(pnpm, "--dir", "web", "exec", "vitest", "run", "src", "--reporter=json", "--outputFile=../.cache/harness/vitest.json")
					if err == nil {
						results, parseErr := webResults(".", ".cache/harness/vitest.json")
						testOutcomes = append(testOutcomes, results...)
						err = parseErr
					}
					return detail, err
				})
			}
			run("named-tests", func() (string, error) { return "", verifyNamedResults(".", "full", testOutcomes) })
			run("data-scale", func() (string, error) { return validateDataScale(".", started) })
		}
	}
	if !*update {
		run("source-stable", func() (string, error) {
			digest, err := sourceDigest(".")
			if err == nil && digest != initialDigest {
				err = fmt.Errorf("source changed during verification; rerun on stable inputs")
			}
			return "", err
		})
	} else {
		initialDigest, _ = sourceDigest(".")
	}
	revision, _ := command("git", "rev-parse", "HEAD")
	status, _ := command("git", "status", "--porcelain")
	toolchain := map[string]string{"go": runtime.Version(), "os": runtime.GOOS, "arch": runtime.GOARCH}
	for _, tool := range []struct {
		name, exe string
		args      []string
	}{
		{"node", "node", []string{"--version"}}, {"pnpm", pnpm, []string{"--version"}},
	} {
		if value, err := command(tool.exe, tool.args...); err == nil {
			toolchain[tool.name] = strings.TrimSpace(value)
		}
	}
	report := Evidence{Task: *task, Version: 1, RunID: runID(), StartedAt: started, RecordedAt: time.Now().UTC(), Revision: strings.TrimSpace(revision), WorktreeDirty: strings.TrimSpace(status) != "", SourceDigest: initialDigest, Toolchain: toolchain, Mode: *mode, IntegrationRequired: *mode == "full" || *mode == "acceptance", Checks: checks, Tests: testOutcomes}
	if err := saveEvidence(*out, report); err != nil {
		if releaseLock != nil {
			releaseLock()
		}
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	if err := captureFailures(".", report); err != nil {
		fmt.Fprintln(os.Stderr, "failure capture:", err)
		if releaseLock != nil {
			releaseLock()
		}
		os.Exit(1)
	}
	for _, check := range checks {
		if !check.Passed {
			if releaseLock != nil {
				releaseLock()
			}
			os.Exit(1)
		}
	}
}
