// Explicit, bounded live adapter evaluation. No database, retries or resumes.
package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"foodflow/internal/agent"
	"github.com/joho/godotenv"
)

type Sample struct {
	ID       string         `json:"id"`
	Domain   string         `json:"domain"`
	Path     string         `json:"path"`
	MIME     string         `json:"mime"`
	Expected string         `json:"expected"`
	Reject   bool           `json:"reject"`
	Context  map[string]any `json:"context"`
}
type Row struct {
	ID            string         `json:"id"`
	State         string         `json:"state"`
	Correct       bool           `json:"correct"`
	HTTP          int            `json:"http"`
	LatencyMS     int64          `json:"latency_ms"`
	InputDigest   string         `json:"input_digest"`
	Usage         map[string]any `json:"usage"`
	ReportedModel string         `json:"reported_model"`
	OutputDigest  string         `json:"output_digest,omitempty"`
	RAGResponse   *ragResponse   `json:"rag_response,omitempty"`
}

type ragResponse struct {
	Citation string `json:"citation"`
	Answer   string `json:"answer"`
}

func digest(b []byte) string { h := sha256.Sum256(b); return hex.EncodeToString(h[:]) }

func scoreRAG(content string, s Sample) bool {
	a, err := parseRAG(content)
	return err == nil && a.Citation == s.Expected && a.Answer == fmt.Sprint(s.Context["expected_answer"])
}

// Decoder.DisallowUnknownFields alone does not reject duplicate JSON keys.
func parseRAG(content string) (*ragResponse, error) {
	d := json.NewDecoder(strings.NewReader(content))
	if t, err := d.Token(); err != nil || t != json.Delim('{') {
		return nil, fmt.Errorf("RAG answer must be an object")
	}
	values := map[string]string{}
	for d.More() {
		t, err := d.Token()
		key, ok := t.(string)
		_, duplicate := values[key]
		if err != nil || !ok || duplicate || key != "citation" && key != "answer" {
			return nil, fmt.Errorf("invalid RAG answer fields")
		}
		value, err := d.Token()
		text, ok := value.(string)
		if err != nil || !ok || len(text) > 4096 {
			return nil, fmt.Errorf("invalid RAG answer text")
		}
		values[key] = text
	}
	if t, err := d.Token(); err != nil || t != json.Delim('}') || len(values) != 2 {
		return nil, fmt.Errorf("incomplete RAG answer")
	}
	if _, err := d.Token(); err != io.EOF {
		return nil, fmt.Errorf("trailing RAG answer")
	}
	return &ragResponse{Citation: values["citation"], Answer: values["answer"]}, nil
}
func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	paid := flag.Bool("paid", false, "explicit live authorization")
	domain := flag.String("domain", "", "vision, speech, menu or rag")
	out := flag.String("out", "", "fresh private report under .cache/harness")
	corpus := flag.String("corpus", "tests/fixtures/quality-eval.json", "frozen public/synthetic corpus")
	maxCalls := flag.Int("max-calls", 4, "bounded number of calls; no automatic retries")
	provider := flag.String("provider", "", "optional existing vision credential pair for menu/rag evaluation only")
	ragPrompt := flag.String("rag-prompt", "strict", "strict or baseline; baseline preserves the original v0.3 raw-model prompt")
	retainRAG := flag.Bool("retain-rag-response", false, "retain bounded structured RAG answers in the private journal for reviewed diagnostics")
	flag.Parse()
	if !strings.Contains("|vision|speech|menu|rag|", "|"+*domain+"|") || *ragPrompt != "strict" && *ragPrompt != "baseline" {
		return fmt.Errorf("unsupported domain or RAG prompt version")
	}
	raw, err := os.ReadFile(*corpus)
	if err != nil {
		return fmt.Errorf("corpus unavailable")
	}
	var all []Sample
	if json.Unmarshal(raw, &all) != nil {
		return fmt.Errorf("invalid corpus")
	}
	samples := []Sample{}
	seen := map[string]bool{}
	for _, s := range all {
		if seen[s.ID] || s.ID == "" {
			return fmt.Errorf("duplicate sample")
		}
		seen[s.ID] = true
		if s.Domain == *domain {
			samples = append(samples, s)
		}
	}
	if len(samples) == 0 || len(samples) > *maxCalls || *maxCalls > 20 {
		return fmt.Errorf("invalid corpus or call bound")
	}
	if err := validateInputs(samples, *paid); err != nil {
		return err
	}
	if !*paid {
		fmt.Printf("offline: %d frozen samples, digest %s\n", len(samples), digest(raw))
		return nil
	}
	target, err := filepath.Abs(*out)
	base, _ := filepath.Abs(".cache/harness")
	if err != nil || !strings.HasPrefix(strings.ToLower(target), strings.ToLower(base+string(os.PathSeparator))) {
		return fmt.Errorf("journal must be private")
	}
	cfg, err := godotenv.Read(".env")
	if err != nil {
		return fmt.Errorf("model configuration unavailable")
	}
	prefix := "MODEL"
	if *domain == "vision" {
		prefix = "VISION_MODEL"
	}
	if *domain == "speech" {
		prefix = "SPEECH_MODEL"
	}
	if *provider != "" {
		if *provider != "vision" || (*domain != "menu" && *domain != "rag") {
			return fmt.Errorf("provider override only supports existing vision pair for text evaluations")
		}
		prefix = "VISION_MODEL"
	}
	endpoint, key, name := cfg[prefix+"_ENDPOINT"], cfg[prefix+"_API_KEY"], cfg[prefix+"_NAME"]
	u, e := url.Parse(endpoint)
	if e != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" || key == "" || name == "" {
		return fmt.Errorf("configured HTTPS provider required")
	}
	// Hash only public configuration, never keys.
	configHash := digest([]byte(endpoint + "|" + name))
	fingerprint, e := exec.Command("go", "run", "./cmd/harness", "-mode", "fingerprint").Output()
	if e != nil {
		return fmt.Errorf("source fingerprint unavailable")
	}
	source := strings.TrimSpace(string(fingerprint))
	if e = os.MkdirAll(filepath.Dir(target), 0700); e != nil {
		return e
	}
	f, e := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0600)
	if e != nil {
		return fmt.Errorf("journal already exists; inspect uncertain attempts, never auto-resume")
	}
	f.Close()
	rows := []Row{}
	report := map[string]any{"version": 1, "domain": *domain, "source_digest": source, "started_at": time.Now().UTC(),
		"corpus_digest": digest(raw), "model": name, "model_version": "unknown unless reported per row", "prompt_digest": agent.RuntimeDigest(),
		"config_digest": configHash, "cost_status": "unknown", "cost": nil, "cost_basis": "provider invoice not available; tokens are not currency",
		"live": true, "max_calls": *maxCalls, "scope": "public/synthetic adapter cases; no physical-device or stock-write claim"}
	if *domain == "rag" {
		report["rag_prompt"] = *ragPrompt
		report["retained_rag_response"] = *retainRAG
	}
	save := func() error {
		report["rows"] = rows
		b, _ := json.MarshalIndent(report, "", "  ")
		return os.WriteFile(target, b, 0600)
	}
	if err = save(); err != nil {
		return err
	}
	for _, s := range samples {
		var data []byte
		if s.Path != "" {
			p := filepath.ToSlash(filepath.Clean(s.Path))
			if strings.Contains(p, "..") || !(strings.HasPrefix(p, "tests/fixtures/") || strings.HasPrefix(p, "web/public/") || strings.HasPrefix(p, ".cache/harness/live-inputs/")) {
				return fmt.Errorf("unapproved sample path")
			}
			data, err = os.ReadFile(p)
			if err != nil {
				return fmt.Errorf("sample unavailable")
			}
			if len(data) > 5<<20 {
				return fmt.Errorf("sample too large")
			}
		}
		b, _ := json.Marshal(s)
		rows = append(rows, Row{ID: s.ID, State: "not_dispatched", InputDigest: digest(append(b, data...)), Usage: map[string]any{}})
		r := &rows[len(rows)-1]
		validRejection := false
		guard := func(context.Context, int) (func(bool), error) {
			r.State = "dispatched"
			if e := save(); e != nil {
				return nil, e
			}
			return func(bool) {}, nil
		}
		observe := func(status int, body []byte) {
			r.HTTP = status
			var envelope struct {
				Usage   map[string]any `json:"usage"`
				Model   string         `json:"model"`
				Choices []struct {
					Message struct {
						Content string `json:"content"`
					} `json:"message"`
				} `json:"choices"`
			}
			if json.Unmarshal(body, &envelope) == nil {
				if envelope.Usage != nil {
					r.Usage = envelope.Usage
				}
				r.ReportedModel = envelope.Model
				if len(envelope.Choices) == 1 {
					content := envelope.Choices[0].Message.Content
					r.OutputDigest = digest([]byte(content))
					if *domain == "rag" && *retainRAG {
						// Save only bounded structured fields, never the authenticated envelope.
						redacted := strings.ReplaceAll(content, key, "[redacted]")
						if !containsCredentialMarker(redacted) {
							r.RAGResponse, _ = parseRAG(redacted)
						}
					}
					var observation struct {
						Scene string   `json:"scene"`
						Foods []string `json:"foods"`
					}
					if json.Unmarshal([]byte(envelope.Choices[0].Message.Content), &observation) == nil {
						validRejection = (observation.Scene == "text" || observation.Scene == "drawing" || observation.Scene == "screenshot" || observation.Scene == "mixed_dish" || observation.Scene == "uncertain") && len(observation.Foods) == 0 || observation.Scene == "photo" && len(observation.Foods) != 1
					}
				}
			}
		}
		start := time.Now()
		ctx := context.Background()
		switch *domain {
		case "vision":
			m := agent.VisionModel{Endpoint: endpoint, Name: name, Key: key, Guard: guard, Observe: observe}
			got, e := m.Recognize(ctx, data, s.MIME)
			err = e
			// Rejection is successful only after a valid provider response, never a timeout.
			r.Correct = r.HTTP == 200 && ((s.Reject && e != nil && validRejection) || (!s.Reject && e == nil && got.Name == s.Expected))
		case "speech":
			m := agent.SpeechModel{Endpoint: endpoint, Name: name, Key: key, Guard: guard, Observe: observe}
			got, e := m.Transcribe(ctx, data, s.MIME)
			err = e
			clean := func(x string) string { return strings.Trim(x, " \r\n。，！!,.") }
			r.Correct = e == nil && clean(got) == clean(s.Expected)
		case "menu":
			m := agent.OpenAIModel{Endpoint: endpoint, Name: name, Key: key, Guard: guard, Observe: observe, Task: "plan", Context: s.Context}
			got, e := m.Generate(ctx, nil)
			err = e
			if e == nil {
				id, e := agent.ParseRecipeID(got.Content)
				r.Correct = e == nil && id == s.Expected
			}
		case "rag":
			// Gold answers are evaluator-only; never leak them into model input.
			contextOnly := map[string]any{}
			for key, value := range s.Context {
				if key != "expected_answer" {
					contextOnly[key] = value
				}
			}
			task := "rag_eval_strict"
			if *ragPrompt == "baseline" {
				task = "rag_eval"
			}
			m := agent.OpenAIModel{Endpoint: endpoint, Name: name, Key: key, Guard: guard, Observe: observe, MaxTokens: 500, Task: task, Context: contextOnly}
			got, e := m.Generate(ctx, nil)
			err = e
			if e == nil {
				r.Correct = scoreRAG(got.Content, s)
			}
		default:
			return fmt.Errorf("unsupported domain")
		}
		r.LatencyMS = time.Since(start).Milliseconds()
		r.State = "returned"
		if r.HTTP != 200 {
			r.State = "service_failure"
		} else if err != nil && !s.Reject {
			r.State = "model_failure"
		}
		if err = save(); err != nil {
			return err
		}
		fmt.Printf("%s state=%s correct=%v http=%d\n", s.ID, r.State, r.Correct, r.HTTP)
		if r.HTTP == 402 || r.HTTP == 429 || r.HTTP == 403 {
			report["stopped"] = "provider rejected; inspect quota/billing without automatic retry"
			break
		}
	}
	correct, failures := 0, 0
	latencies := []int64{}
	usage := map[string]any{"availability": "per-row provider usage; missing fields unknown"}
	for _, r := range rows {
		if r.Correct {
			correct++
		}
		if r.State == "service_failure" {
			failures++
		}
		latencies = append(latencies, r.LatencyMS)
	}
	sort.Slice(latencies, func(i, j int) bool { return latencies[i] < latencies[j] })
	report["samples"] = len(samples)
	report["attempted"] = len(rows)
	report["correct"] = correct
	report["service_failures"] = failures
	if len(latencies) > 0 {
		report["p95_ms"] = latencies[(95*len(latencies)+99)/100-1]
	}
	report["usage"] = usage
	report["recorded_at"] = time.Now().UTC()
	report["journal"] = filepath.ToSlash(*out)
	if err := save(); err != nil {
		return err
	}
	return evaluationOutcome(correct, len(rows), len(samples))
}

func evaluationOutcome(correct, attempted, samples int) error {
	if samples <= 0 || correct != samples || attempted != samples {
		return fmt.Errorf("evaluation failed: %d/%d correct, %d attempted; original journal retained", correct, samples, attempted)
	}
	return nil
}

func containsCredentialMarker(value string) bool {
	value = strings.ToLower(value)
	for _, marker := range []string{"bearer ", "api_key", "password=", "authorization:", "-----begin", "sk-"} {
		if strings.Contains(value, marker) {
			return true
		}
	}
	return false
}

func validateInputs(samples []Sample, live bool) error {
	for _, s := range samples {
		if s.Path == "" {
			continue
		}
		p := filepath.ToSlash(filepath.Clean(s.Path))
		if strings.Contains(p, "..") || !(strings.HasPrefix(p, "tests/fixtures/") || strings.HasPrefix(p, "web/public/") || strings.HasPrefix(p, ".cache/harness/live-inputs/")) {
			return fmt.Errorf("unapproved sample path")
		}
		if !live && strings.HasPrefix(p, ".cache/") {
			continue
		}
		info, err := os.Stat(p)
		if err != nil || info.IsDir() || info.Size() == 0 || info.Size() > 5<<20 {
			return fmt.Errorf("sample preflight failed for %s; no requests dispatched", s.ID)
		}
	}
	return nil
}
