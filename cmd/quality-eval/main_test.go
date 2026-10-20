package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

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
