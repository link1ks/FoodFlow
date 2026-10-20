package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Scope and thresholds are bound before final validation; a missing applicable
// result is pending, not a successful exemption.
type FeedbackScope struct {
	Evaluations         []EvaluationRequirement `json:"evaluations"`
	DeviceRequired      bool                    `json:"device_required"`
	DeviceExemption     string                  `json:"device_exemption"`
	RAGRequired         bool                    `json:"rag_required"`
	RAGExemption        string                  `json:"rag_exemption"`
	EvaluationExemption string                  `json:"evaluation_exemption"`
}
type EvaluationRequirement struct {
	Domain          string  `json:"domain"`
	Live            bool    `json:"live"`
	MinimumSamples  int     `json:"minimum_samples"`
	MinimumAccuracy float64 `json:"minimum_accuracy"`
	MaxP95MS        int64   `json:"max_p95_ms"`
}
type EvaluationResult struct {
	Domain          string         `json:"domain"`
	Live            bool           `json:"live"`
	Samples         int            `json:"samples"`
	Correct         int            `json:"correct"`
	ServiceFailures int            `json:"service_failures"`
	P95MS           int64          `json:"p95_ms"`
	CorpusDigest    string         `json:"corpus_digest"`
	Model           string         `json:"model"`
	ModelVersion    string         `json:"model_version"`
	PromptDigest    string         `json:"prompt_digest"`
	ConfigDigest    string         `json:"config_digest"`
	Usage           map[string]any `json:"usage"`
	CostStatus      string         `json:"cost_status"`
	Cost            *float64       `json:"cost"`
	CostBasis       string         `json:"cost_basis"`
	Journal         string         `json:"journal"`
}
type DeviceResult struct {
	Status      string `json:"status"`
	Environment string `json:"environment"`
	Operator    string `json:"operator"`
	Steps       string `json:"steps"`
	Expected    string `json:"expected"`
	Actual      string `json:"actual"`
	Evidence    string `json:"evidence"`
}
type FeedbackReport struct {
	Version      int                `json:"version"`
	Task         string             `json:"task"`
	SourceDigest string             `json:"source_digest"`
	StartedAt    time.Time          `json:"started_at"`
	RecordedAt   time.Time          `json:"recorded_at"`
	Evaluations  []EvaluationResult `json:"evaluations"`
	Device       DeviceResult       `json:"device"`
	RAGReport    string             `json:"rag_report"`
	Metrics      string             `json:"metrics"`
	Failures     []string           `json:"failures"`
}

func validateFeedbackScope(s *FeedbackScope) error {
	if s == nil {
		return fmt.Errorf("task requires explicit feedback scope")
	}
	if len(s.Evaluations) == 0 && s.EvaluationExemption == "" || !s.DeviceRequired && s.DeviceExemption == "" || !s.RAGRequired && s.RAGExemption == "" {
		return fmt.Errorf("inapplicable feedback checks require reasons")
	}
	seen := map[string]bool{}
	for _, r := range s.Evaluations {
		if r.Domain == "" || seen[r.Domain] || r.MinimumSamples < 1 || r.MinimumAccuracy <= 0 || r.MinimumAccuracy > 1 || r.MaxP95MS < 1 {
			return fmt.Errorf("invalid evaluation thresholds")
		}
		seen[r.Domain] = true
	}
	return nil
}
func validateFeedback(root, task string, c TaskContract, path, digest string) error {
	if err := validateFeedbackScope(c.Feedback); err != nil {
		return err
	}
	var f FeedbackReport
	if err := readLocalJSON(root, path, &f); err != nil {
		return fmt.Errorf("feedback: %w", err)
	}
	if f.Version != 1 || f.Task != task || f.SourceDigest != digest || f.StartedAt.Before(c.BoundAt) || f.RecordedAt.Before(f.StartedAt) || f.RecordedAt.After(time.Now().Add(time.Minute)) {
		return fmt.Errorf("stale/foreign feedback")
	}
	for _, req := range c.Feedback.Evaluations {
		count := 0
		for _, r := range f.Evaluations {
			if r.Domain != req.Domain {
				continue
			}
			count++
			if r.Live != req.Live || r.Samples < req.MinimumSamples || r.Correct < 0 || r.Correct > r.Samples || r.ServiceFailures < 0 ||
				r.ServiceFailures > r.Samples || float64(r.Correct)/float64(r.Samples) < req.MinimumAccuracy || r.P95MS <= 0 || r.P95MS > req.MaxP95MS ||
				r.CorpusDigest == "" || r.PromptDigest == "" || r.ConfigDigest == "" || r.Model == "" || r.ModelVersion == "" || r.Usage == nil ||
				!contains([]string{"unknown", "estimated", "actual"}, r.CostStatus) || r.CostBasis == "" ||
				r.CostStatus == "unknown" && r.Cost != nil || r.CostStatus != "unknown" && (r.Cost == nil || *r.Cost < 0) {
				return fmt.Errorf("evaluation %s failed thresholds/provenance/cost", req.Domain)
			}
			var journal map[string]any
			if err := readLocalJSON(root, r.Journal, &journal); err != nil {
				return err
			}
			if journal["source_digest"] != digest || journal["domain"] != r.Domain {
				return fmt.Errorf("evaluation journal mismatch")
			}
			start, e := time.Parse(time.RFC3339Nano, fmt.Sprint(journal["started_at"]))
			if e != nil || start.Before(c.BoundAt) || journal["samples"] != float64(r.Samples) || journal["correct"] != float64(r.Correct) ||
				journal["p95_ms"] != float64(r.P95MS) || journal["corpus_digest"] != r.CorpusDigest || journal["live"] != r.Live ||
				journal["model"] != r.Model || journal["prompt_digest"] != r.PromptDigest || journal["config_digest"] != r.ConfigDigest ||
				journal["cost_status"] != r.CostStatus {
				return fmt.Errorf("evaluation aggregate differs from original journal")
			}
			rows, ok := journal["rows"].([]any)
			if !ok || len(rows) != r.Samples {
				return fmt.Errorf("incomplete evaluation coverage")
			}
			correct := 0
			seen := map[string]bool{}
			for _, entry := range rows {
				row, ok := entry.(map[string]any)
				if !ok {
					return fmt.Errorf("invalid evaluation row")
				}
				id, _ := row["id"].(string)
				if id == "" || seen[id] || row["state"] == "not_dispatched" || row["state"] == "dispatched" {
					return fmt.Errorf("duplicate or uncertain evaluation row")
				}
				seen[id] = true
				if row["correct"] == true {
					if row["http"] != float64(200) {
						return fmt.Errorf("service failure counted as correct")
					}
					correct++
				}
			}
			if correct != r.Correct {
				return fmt.Errorf("evaluation correctness count mismatch")
			}
		}
		if count != 1 {
			return fmt.Errorf("missing/duplicate evaluation %s", req.Domain)
		}
	}
	if c.Feedback.DeviceRequired {
		d := f.Device
		if d.Status != "passed" || d.Environment == "" || d.Operator == "" || d.Steps == "" || d.Expected == "" || d.Actual == "" {
			return fmt.Errorf("physical device acceptance pending")
		}
		p, e := repositoryFile(root, d.Evidence)
		if e != nil {
			return e
		}
		if _, e = os.Stat(p); e != nil {
			return e
		}
	}
	if c.Feedback.RAGRequired {
		var r struct {
			Version      int       `json:"version"`
			Passed       bool      `json:"passed"`
			SourceDigest string    `json:"source_digest"`
			Cases        int       `json:"cases"`
			RecordedAt   time.Time `json:"recorded_at"`
		}
		if err := readLocalJSON(root, f.RAGReport, &r); err != nil {
			return err
		}
		if r.Version != 1 || !r.Passed || r.Cases < 5 || r.SourceDigest != digest || r.RecordedAt.Before(c.BoundAt) {
			return fmt.Errorf("RAG evaluation missing/stale/failed")
		}
	}
	var metrics map[string]any
	if err := readLocalJSON(root, f.Metrics, &metrics); err != nil {
		return err
	}
	when, err := time.Parse(time.RFC3339Nano, fmt.Sprint(metrics["recorded_at"]))
	if err != nil || when.Before(c.BoundAt) || metrics["version"] != float64(1) {
		return fmt.Errorf("fresh metrics required")
	}
	if when.After(time.Now().Add(time.Minute)) {
		return fmt.Errorf("future metrics rejected")
	}
	current, err := learningMetrics(root)
	if err != nil {
		return err
	}
	computed := current.(map[string]any)
	delete(computed, "recorded_at")
	delete(metrics, "recorded_at")
	if hashJSON(computed) != hashJSON(metrics) {
		return fmt.Errorf("metrics must be recomputed from complete current history")
	}
	records, err := knowledgeRecords(root)
	if err != nil {
		return err
	}
	for _, record := range records {
		if record.Task != task {
			continue
		}
		if !contains(f.Failures, record.ID) || record.Status != "resolved" {
			return fmt.Errorf("unresolved/unreported task failure %s", record.ID)
		}
		if err = validateFailure(root, record); err != nil {
			return err
		}
		for _, group := range []struct {
			paths []string
			mode  string
		}{{record.Regression, "full"}, {record.PageEvidence, "acceptance"}} {
			verified := false
			for _, p := range group.paths {
				var r Evidence
				if e := readLocalJSON(root, p, &r); e != nil {
					continue
				}
				if e := requiredReport(r, group.mode, digest, c.BoundAt, []string{"named-tests"}); e != nil {
					continue
				}
				if group.mode == "full" {
					executed := map[string]bool{}
					for _, test := range r.Tests {
						executed[test.ID] = test.Status == "pass"
					}
					matched := true
					for _, id := range record.Tests {
						matched = matched && executed[id]
					}
					if !matched {
						continue
					}
				}
				verified = true
			}
			if !verified {
				return fmt.Errorf("failure %s lacks current executed %s regression evidence", record.ID, group.mode)
			}
		}
	}
	files, _ := filepath.Glob(filepath.Join(root, ".cache/harness/experience/queries/*.json"))
	queried := false
	for _, p := range files {
		var q QueryRecord
		raw, e := os.ReadFile(p)
		if e != nil {
			return e
		}
		if e = json.Unmarshal(raw, &q); e != nil {
			return e
		}
		if q.Task == task && q.Stage == "start" && q.Disposition != "" && q.RecordedAt.Before(c.BoundAt) {
			queried = true
		}
	}
	if !queried {
		return fmt.Errorf("pre-binding historical experience query required")
	}
	return nil
}
