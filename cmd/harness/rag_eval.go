package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

// Uses a disposable repository-shaped corpus so acceptance never mutates the
// user's knowledge. Fixtures contain no real user logs or production data.
func evaluateRAG(root, out string) error {
	raw, err := os.ReadFile(filepath.Join(root, "docs/quality/rag-eval.json"))
	if err != nil {
		return err
	}
	var suite struct {
		Cases []struct {
			ID, Query, Business, Expected string
			Mismatch                      bool
		}
	}
	if err = json.Unmarshal(raw, &suite); err != nil {
		return err
	}
	tmp, err := os.MkdirTemp("", "foodflow-rag-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmp)
	proof := ".cache/harness/proof.json"
	if err = saveJSON(filepath.Join(tmp, proof), map[string]bool{"fixture": true}); err != nil {
		return err
	}
	f := Failure{ID: "stock-replay", Revision: 1, Task: "fixture", Category: "code", Business: []string{"STOCK"}, Tests: []string{"fixture#stock"},
		CodeVersion: "old", ModelVersion: "not-applicable", Environment: "synthetic", Conditions: "duplicate inventory request", Symptom: "stock added twice",
		Reproduction: "repeat request", RootCause: "missing transaction idempotency", Verified: true, Fix: "Check transaction idempotency key.",
		Scope: "stock writes", Status: "resolved", Severity: 4, Owner: "fixture", Evidence: []string{proof}, Regression: []string{proof}, PageEvidence: []string{proof}, RedactionReviewed: true}
	if err = ingestFailure(tmp, f); err != nil {
		return err
	}
	type result struct {
		ID     string `json:"id"`
		Passed bool   `json:"passed"`
	}
	results := []result{}
	passed := true
	for _, c := range suite.Cases {
		q, e := queryExperience(tmp, QueryRecord{Query: c.Query, Business: c.Business, CodeVersion: "new"})
		ok := e == nil
		if c.Expected == "" {
			ok = ok && len(q.Results) == 0
		} else {
			ok = ok && len(q.Results) > 0 && q.Results[0].ID == c.Expected && len(q.Results[0].Citations) > 0
			if c.Mismatch {
				ok = ok && q.Results[0].VersionMismatch
			}
		}
		results = append(results, result{c.ID, ok})
		passed = passed && ok
	}
	// Evaluate conflict visibility, uncertainty, deletion and stale-index refusal.
	f.Revision++
	f.Conflicts = []string{"contradictory-fix"}
	err = ingestFailure(tmp, f)
	q, e := queryExperience(tmp, QueryRecord{Query: "duplicate"})
	ok := err == nil && e == nil && len(q.Results) > 0 && len(q.Results[0].Conflicts) > 0
	results = append(results, result{"conflict-visible", ok})
	passed = passed && ok
	f.Revision++
	f.Verified = false
	f.Status = "open"
	err = ingestFailure(tmp, f)
	q, e = queryExperience(tmp, QueryRecord{Query: "duplicate", VerifiedOnly: true})
	ok = err == nil && e == nil && len(q.Results) == 0
	results = append(results, result{"hypothesis-excluded", ok})
	passed = passed && ok
	f.Revision++
	f.Status = "obsolete"
	err = ingestFailure(tmp, f)
	q, e = queryExperience(tmp, QueryRecord{Query: "duplicate"})
	ok = err == nil && e == nil && len(q.Results) == 0
	results = append(results, result{"obsolete-excluded", ok})
	passed = passed && ok
	digest, err := sourceDigest(root)
	if err != nil {
		return err
	}
	report := map[string]any{"version": 1, "passed": passed, "source_digest": digest, "corpus_digest": hashJSON(suite), "retrieval_version": retrievalVersion,
		"cases": len(results), "results": results, "recorded_at": time.Now().UTC(), "scope": "retrieval, citation and lifecycle; live grounded generation is a separate paid evaluation"}
	if err = saveJSON(out, report); err != nil {
		return err
	}
	if !passed {
		return fmt.Errorf("RAG acceptance failed")
	}
	return nil
}
