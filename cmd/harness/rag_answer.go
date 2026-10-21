package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"time"

	"foodflow/internal/agent"
	"github.com/joho/godotenv"
)

// Generate from freshly retrieved evidence, then restrict the output to a
// cited, verified historical fix. Neither source prose nor model text executes.
func answerExperience(root, input, out string, paid bool, provider string) error {
	if !paid {
		return fmt.Errorf("answer requires -paid and explicit task authorization")
	}
	var q QueryRecord
	if err := readLocalJSON(root, input, &q); err != nil {
		return err
	}
	if q.Task == "" || q.Query == "" {
		return fmt.Errorf("query requires task and question")
	}
	q, err := queryExperience(root, q)
	if err != nil {
		return err
	}
	evidence := []map[string]any{}
	if err = agent.RunRAG(map[string]any{"action": "evidence", "results": q.Results}, &evidence); err != nil {
		return err
	}
	if len(evidence) == 0 {
		return saveJSON(out, map[string]any{"status": "insufficient_evidence", "retrieval": q, "cost": 0, "cost_status": "actual", "cost_basis": "no model dispatch"})
	}
	cfg, err := godotenv.Read(filepath.Join(root, ".env"))
	if err != nil {
		return fmt.Errorf("model configuration unavailable")
	}
	if provider != "" {
		if provider != "vision" {
			return fmt.Errorf("unsupported provider override")
		}
		for _, key := range []string{"ENDPOINT", "NAME", "API_KEY"} {
			cfg["MODEL_"+key] = cfg["VISION_MODEL_"+key]
		}
	}
	u, err := url.Parse(cfg["MODEL_ENDPOINT"])
	if err != nil || u.Scheme != "https" || u.User != nil || u.RawQuery != "" {
		return fmt.Errorf("configured HTTPS provider required")
	}
	if cfg["MODEL_API_KEY"] == "" || cfg["MODEL_NAME"] == "" {
		return fmt.Errorf("model not configured")
	}
	journal := filepath.Join(root, ".cache/harness/experience/answers", runID()+".json")
	record := map[string]any{"task": q.Task, "started_at": time.Now().UTC(), "model": cfg["MODEL_NAME"], "model_version": "unknown",
		"prompt_version": "python-grounded-selection-v2", "index_digest": q.IndexDigest, "cost": nil, "cost_status": "unknown", "cost_basis": "provider invoice unavailable", "state": "prepared"}
	guard := func(context.Context, int) (func(bool), error) {
		record["state"] = "dispatched"
		if e := saveJSON(journal, record); e != nil {
			return nil, e
		}
		return func(bool) {}, nil
	}
	observe := func(status int, b []byte) {
		record["http"] = status
		var v struct {
			Usage map[string]any `json:"usage"`
			Model string         `json:"model"`
		}
		if json.Unmarshal(b, &v) == nil {
			record["usage"] = v.Usage
			if v.Model != "" {
				record["model_version"] = v.Model
			}
		}
	}
	model := agent.OpenAIModel{Endpoint: cfg["MODEL_ENDPOINT"], Name: cfg["MODEL_NAME"], Key: cfg["MODEL_API_KEY"], MaxTokens: 600, Guard: guard, Observe: observe, Task: "rag", Context: map[string]any{"question": q.Query, "evidence": evidence}}
	start := time.Now()
	msg, err := model.Generate(context.Background(), nil)
	record["latency_ms"] = time.Since(start).Milliseconds()
	record["recorded_at"] = time.Now().UTC()
	if err != nil {
		record["state"] = "failed_or_uncertain"
		_ = saveJSON(journal, record)
		return fmt.Errorf("model answer unavailable; inspect redacted journal, no automatic retry")
	}
	var a struct {
		Citation string `json:"citation"`
		Answer   string `json:"answer"`
	}
	if err := json.Unmarshal([]byte(msg.Content), &a); err != nil {
		return fmt.Errorf("Python RAG returned an invalid protocol result")
	}
	record["state"] = "returned"
	record["citation"] = a.Citation
	record["answer"] = a.Answer
	record["warning"] = "Historical suggestion only; revalidate against current code, especially version mismatches."
	record["retrieval"] = q
	if err = saveJSON(journal, record); err != nil {
		return err
	}
	if info, e := os.Stat(out); e == nil && info.IsDir() {
		return fmt.Errorf("output must be file")
	}
	return saveJSON(out, record)
}
