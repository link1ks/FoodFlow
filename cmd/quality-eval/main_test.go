package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestRAGEvaluationRejectsMalformedAndUnsupportedAnswers(t *testing.T) {
	s := Sample{Expected: "safe", Context: map[string]any{"expected_answer": "Check readiness."}}
	for _, content := range []string{
		`{"citation":"safe","answer":"Check readiness.","extra":true}`,
		`{"citation":"unsafe","citation":"safe","answer":"Check readiness."}`,
		`{"citation":"safe","answer":"wrong","answer":"Check readiness."}`,
		`{"citation":"safe","answer":"Check readiness."} {}`,
		`{"citation":"safe","answer":"Check readiness.","answer":null}`,
		`{"citation":"safe"}`,
		`{"citation":null,"answer":"Check readiness."}`,
	} {
		if scoreRAG(content, s) {
			t.Fatalf("malformed output counted as correct: %s", content)
		}
	}
	if !scoreRAG(`{"answer":"Check readiness.","citation":"safe"}`, s) {
		t.Fatal("valid exact repair rejected")
	}
	s.Expected, s.Context["expected_answer"] = "", "insufficient_evidence"
	if !scoreRAG(`{"citation":"","answer":"insufficient_evidence"}`, s) || scoreRAG(`{"citation":"conflict","answer":"Restart."}`, s) {
		t.Fatal("conflict abstention criterion changed")
	}
}

func TestEvaluationFailureAndIncompleteCoverageReturnError(t *testing.T) {
	for _, counts := range [][3]int{{2, 3, 3}, {2, 2, 3}, {3, 2, 3}, {0, 0, 0}} {
		if evaluationOutcome(counts[0], counts[1], counts[2]) == nil {
			t.Fatalf("failed/incomplete evaluation reported success: %v", counts)
		}
	}
	if err := evaluationOutcome(3, 3, 3); err != nil {
		t.Fatal(err)
	}
	for _, value := range []string{"Bearer private", "sk-private", "AUTHORIZATION: value", "api_key=value"} {
		if !containsCredentialMarker(value) {
			t.Fatal("credential marker admitted into diagnostic response")
		}
	}
}

func TestQualityCorpusPreflightRejectsMissingMediaBeforeDispatch(t *testing.T) {
	// t.Chdir also restores the original directory, independent of test order.
	t.Chdir(filepath.Join("..", ".."))
	raw, err := os.ReadFile("tests/fixtures/quality-eval.json")
	if err != nil {
		t.Fatal(err)
	}
	var samples []Sample
	if err = json.Unmarshal(raw, &samples); err != nil {
		t.Fatal(err)
	}
	if err = validateInputs(samples, false); err != nil {
		t.Fatal(err)
	}
	if validateInputs([]Sample{{ID: "missing", Path: "tests/fixtures/nonexistent.png"}}, true) == nil {
		t.Fatal("missing media admitted")
	}
	if validateInputs([]Sample{{ID: "private", Path: "../../secret"}}, true) == nil {
		t.Fatal("private path admitted")
	}
}
