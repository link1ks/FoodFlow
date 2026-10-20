// Opt-in bounded live image evaluation; default operation is offline.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"foodflow/internal/agent"
	"github.com/joho/godotenv"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type sample struct {
	ID   string `json:"id"`
	Path string `json:"path"`
	Name string `json:"expected_name"`
}
type usage struct {
	Input  int `json:"prompt_tokens"`
	Output int `json:"completion_tokens"`
}
type row struct {
	ID       string `json:"id"`
	Digest   string `json:"sha256"`
	Expected string `json:"expected_name"`
	State    string `json:"state"`
	HTTP     int    `json:"http_status"`
	Millis   int64  `json:"latency_ms"`
	Content  string `json:"model_content"`
	Usage    usage  `json:"usage"`
	Accepted bool   `json:"accepted"`
	Correct  bool   `json:"correct"`
}

func digest(data []byte) string { h := sha256.Sum256(data); return hex.EncodeToString(h[:]) }
func main() {
	if err := run(); err != nil {
		fmt.Println("scope-eval:", err)
		os.Exit(1)
	}
}
func run() error {
	paid := flag.Bool("paid", false, "requires explicit task authorization")
	journal := flag.String("journal", "", "fresh ignored .cache/harness journal, never resumed")
	excludeJournal := flag.String("exclude-journal", "", "exclude every dispatched digest from a prior private trial; never resend")
	tokenLimit := flag.Int("authorized-token-limit", 0, "must explicitly authorize 320000 tokens")
	flag.Parse()
	manifest, err := os.ReadFile("tests/fixtures/ingredient-scope/manifest.json")
	if err != nil {
		return errors.New("manifest unavailable")
	}
	var samples []sample
	if json.Unmarshal(manifest, &samples) != nil || len(samples) != 16 {
		return errors.New("exactly 16 frozen image samples required")
	}
	var photos []struct {
		Name string `json:"name"`
	}
	catalogData, err := os.ReadFile("web/src/catalogPhotos.json")
	if err != nil || json.Unmarshal(catalogData, &photos) != nil {
		return errors.New("public catalogue unavailable")
	}
	known := map[string]bool{}
	for _, p := range photos {
		known[p.Name] = true
	}
	inputs := [][]byte{}
	seen := map[string]bool{}
	for _, s := range samples {
		p := filepath.ToSlash(filepath.Clean(s.Path))
		if s.ID == "" || seen[s.ID] || strings.Contains(p, "..") || !(strings.HasPrefix(p, "tests/fixtures/ingredient-scope/") || strings.HasPrefix(p, "web/public/ingredient-photos/") || p == "web/public/foodflow-icon.png") {
			return errors.New("invalid public sample path/id")
		}
		seen[s.ID] = true
		if s.Name != "" && !known[s.Name] {
			return errors.New("gold name outside catalogue")
		}
		b, e := os.ReadFile(p)
		if e != nil || len(b) == 0 || len(b) > 5<<20 {
			return errors.New("invalid sample bytes")
		}
		inputs = append(inputs, b)
	}
	if !*paid {
		fmt.Printf("offline: %d public samples, manifest SHA256 %s; no provider requests\n", len(samples), digest(manifest))
		return nil
	}
	excluded := map[string]bool{}
	if *excludeJournal != "" {
		b, e := os.ReadFile(*excludeJournal)
		var previous struct {
			Rows []row `json:"rows"`
		}
		if e != nil || json.Unmarshal(b, &previous) != nil || len(previous.Rows) == 0 {
			return errors.New("exclusion journal unavailable")
		}
		for _, r := range previous.Rows {
			if r.State != "not_dispatched" {
				excluded[r.Digest] = true
			}
		}
	}
	if *tokenLimit != 320000 {
		return errors.New("explicit 320000-token trial ceiling required")
	}
	root, _ := filepath.Abs(".cache/harness")
	target, e := filepath.Abs(*journal)
	if e != nil || !strings.HasPrefix(strings.ToLower(target), strings.ToLower(root+string(os.PathSeparator))) {
		return errors.New("private journal must be under .cache/harness")
	}
	cfg, err := godotenv.Read(".env")
	if err != nil {
		return errors.New("local config unavailable")
	}
	endpoint, key := cfg["VISION_MODEL_ENDPOINT"], cfg["VISION_MODEL_API_KEY"]
	if endpoint == "" && key == "" {
		endpoint, key = cfg["MODEL_ENDPOINT"], cfg["MODEL_API_KEY"]
	}
	u, err := url.Parse(endpoint)
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || key == "" || !(u.Hostname() == "dashscope.aliyuncs.com" || strings.HasSuffix(u.Hostname(), ".cn-beijing.maas.aliyuncs.com")) || cfg["VISION_MODEL_NAME"] != "qwen3-vl-plus" {
		return errors.New("approved Beijing vision endpoint/key/model unavailable")
	}
	if err = os.MkdirAll(filepath.Dir(target), 0700); err != nil {
		return errors.New("journal directory unavailable")
	}
	f, err := os.OpenFile(target, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return errors.New("journal exists; no automatic paid resume")
	}
	f.Close()
	rows := []row{}
	reserved := 0
	report := map[string]any{"model": cfg["VISION_MODEL_NAME"], "manifest_sha256": digest(manifest), "agent_runtime_sha256": agent.RuntimeDigest(), "max_requests": 16, "authorized_token_limit": *tokenLimit, "actual_invoice_verified": false, "scope": "public/synthetic adapter evaluation, no inventory/database/browser calls"}
	save := func() error {
		report["rows"] = rows
		report["reserved_tokens"] = reserved
		data, _ := json.MarshalIndent(report, "", "  ")
		return os.WriteFile(target, data, 0600)
	}
	if err = save(); err != nil {
		return errors.New("cannot persist pre-dispatch journal")
	}
	for i, s := range samples {
		if excluded[digest(inputs[i])] {
			fmt.Printf("%s: excluded previously dispatched digest\n", s.ID)
			continue
		}
		rows = append(rows, row{ID: s.ID, Digest: digest(inputs[i]), Expected: s.Name, State: "not_dispatched"})
		entry := &rows[len(rows)-1]
		model := agent.VisionModel{Endpoint: endpoint, Name: cfg["VISION_MODEL_NAME"], Key: key, Observe: func(status int, data []byte) {
			entry.HTTP = status
			var envelope struct {
				Usage   usage `json:"usage"`
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if json.Unmarshal(data, &envelope) == nil {
				entry.Usage = envelope.Usage
				if len(envelope.Choices) == 1 {
					entry.Content = envelope.Choices[0].Message.Content
				}
			}
		}}
		model.Guard = func(context.Context, int) (func(bool), error) {
			if reserved+20000 > *tokenLimit {
				return nil, errors.New("local token reservation exhausted")
			}
			reserved += 20000
			entry.State = "dispatched"
			if save() != nil {
				return nil, errors.New("journal failed before dispatch")
			}
			return func(complete bool) {
				if complete {
					entry.State = "returned"
				} else {
					entry.State = "uncertain"
				}
			}, nil
		}
		started := time.Now()
		result, callErr := model.Recognize(context.Background(), inputs[i], http.DetectContentType(inputs[i]))
		entry.Millis = time.Since(started).Milliseconds()
		entry.Accepted = callErr == nil && known[result.Name]
		entry.Correct = (s.Name == "" && !entry.Accepted) || (s.Name != "" && entry.Accepted && result.Name == s.Name)
		if err = save(); err != nil {
			return errors.New("journal settlement failed; do not resend")
		}
		fmt.Printf("%s: HTTP %d, accepted=%v, correct=%v, %d ms\n", s.ID, entry.HTTP, entry.Accepted, entry.Correct, entry.Millis)
		if entry.HTTP != 200 || entry.Content == "" || entry.State != "returned" || entry.Usage.Input+entry.Usage.Output > 20000 {
			return errors.New("trial stopped after failed/uncertain/over-bound response; do not resend")
		}
	}
	correct, input, output := 0, 0, 0
	for _, r := range rows {
		if r.Correct {
			correct++
		}
		input += r.Usage.Input
		output += r.Usage.Output
	}
	report["correct"] = correct
	report["total"] = len(rows)
	report["input_tokens"] = input
	report["output_tokens"] = output
	if save() != nil {
		return errors.New("final report persistence failed")
	}
	fmt.Printf("correct %d/%d, observed tokens %d + %d; provider invoice/free quota unverified\n", correct, len(rows), input, output)
	return nil
}
