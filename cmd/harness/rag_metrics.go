package main

import (
	"encoding/json"
	"os"
	"path/filepath"
)

func ragHistory(root string) (map[string]any, error) {
	read := func(pattern string) ([]map[string]any, error) {
		files, err := filepath.Glob(filepath.Join(root, pattern))
		if err != nil {
			return nil, err
		}
		values := []map[string]any{}
		for _, p := range files {
			raw, e := os.ReadFile(p)
			if e != nil {
				return nil, e
			}
			var v map[string]any
			if e = json.Unmarshal(raw, &v); e != nil {
				return nil, e
			}
			values = append(values, v)
		}
		return values, nil
	}
	queries, err := read(".cache/harness/experience/queries/*.json")
	if err != nil {
		return nil, err
	}
	answers, err := read(".cache/harness/experience/answers/*.json")
	if err != nil {
		return nil, err
	}
	evals, err := read(".cache/harness/experience/evaluations/*.json")
	if err != nil {
		return nil, err
	}
	nonempty, mismatches, returned, unknownCost := 0, 0, 0, 0
	var queryMS, answerMS float64
	for _, q := range queries {
		if n, ok := q["duration_ms"].(float64); ok {
			queryMS += n
		}
		results, _ := q["results"].([]any)
		if len(results) > 0 {
			nonempty++
		}
		for _, entry := range results {
			r, _ := entry.(map[string]any)
			if r["version_mismatch"] == true {
				mismatches++
			}
		}
	}
	for _, a := range answers {
		if a["state"] == "returned" {
			returned++
		}
		if a["cost_status"] == "unknown" {
			unknownCost++
		}
		if n, ok := a["latency_ms"].(float64); ok {
			answerMS += n
		}
	}
	return map[string]any{"queries": len(queries), "queries_with_results": nonempty, "version_mismatch_results": mismatches,
		"query_duration_ms": queryMS, "generation_attempts": len(answers), "grounded_answers": returned, "generation_duration_ms": answerMS,
		"generation_unknown_cost": unknownCost, "frozen_evaluations": evals,
		"interpretation": "Nonempty retrieval is not relevance/accuracy. Frozen-case outcomes supply quality evidence. Version mismatches need review. Missing prior query/answer histories are not backfilled; unknown costs are not zero."}, nil
}
