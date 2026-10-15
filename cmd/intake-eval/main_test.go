package main

import (
	"foodflow/internal/engine/intake"
	"testing"
)

func TestFrozenCorpusAndScoring(t *testing.T) {
	c, _, e := loadCorpus("../../tests/fixtures/intake-semantic-100.json")
	if e != nil {
		t.Fatal(e)
	}
	categories := map[string]int{}
	for _, v := range c.Cases {
		categories[v.Category]++
	}
	if len(categories) != 10 {
		t.Fatal("missing category")
	}
	for _, n := range categories {
		if n != 10 {
			t.Fatal("unbalanced corpus")
		}
	}
	sample := evalCase{Expected: expected{Decision: "draft", Items: []expectedItem{{"egg", "4.000"}}}}
	if got := score(sample, intake.ProposalResult{Decision: "draft", Items: []intake.Draft{{CatalogID: "egg", Quantity: "3.000"}}}, true); !got.WrongDraft || got.Strict {
		t.Fatal("wrong correction counted as correct")
	}
	if got := score(sample, intake.ProposalResult{Decision: "clarify"}, true); !got.SafeFallback || got.WrongDraft || got.Strict {
		t.Fatal("safe fallback counted as semantic success")
	}
	sample.Expected.Decision = "clarify"
	if got := score(sample, intake.ProposalResult{Decision: "clarify"}, false); got.Strict {
		t.Fatal("invalid JSON rewarded as correct clarification")
	}
	sample.Expected.Decision = "noop"
	if got := score(sample, intake.ProposalResult{Decision: "draft", Items: []intake.Draft{{CatalogID: "egg", Quantity: "3.000"}}}, true); !got.WrongDraft {
		t.Fatal("cancelled addition not counted as error")
	}
}

func TestRecordedWrongIntentCannotPassNumericGroundingAlone(t *testing.T) {
	c, _, err := loadCorpus("../../tests/fixtures/intake-semantic-100.json")
	if err != nil {
		t.Fatal(err)
	}
	byID := map[string]evalCase{}
	for _, sample := range c.Cases {
		byID[sample.ID] = sample
	}
	// Actual responses from the first frozen paid trial. Each amount exists in
	// the utterance, but the proposal violates its intent/clarification rubric.
	responses := map[string]string{
		"negation-01":   `{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":"三个"}]}`,
		"negation-06":   `{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":"两个"},{"catalog_id":"tomato","amount_quote":"500克"}]}`,
		"negation-09":   `{"decision":"draft","items":[{"catalog_id":"tomato","amount_quote":"500克"},{"catalog_id":"egg","amount_quote":"六个"}]}`,
		"ambiguity-01":  `{"decision":"draft","items":[{"catalog_id":"tomato","amount_quote":"500克"},{"catalog_id":"egg","amount_quote":"四个"}]}`,
		"ambiguity-03":  `{"decision":"draft","items":[{"catalog_id":"tomato","amount_quote":"500克"}]}`,
		"ambiguity-04":  `{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":"三个"}]}`,
		"non_intake-05": `{"decision":"draft","items":[{"catalog_id":"potato","amount_quote":"500克"}]}`,
	}
	for id, raw := range responses {
		sample := byID[id]
		result, err := intake.ValidateProposal(sample.Text, raw, c.Catalog)
		if err != nil {
			t.Fatalf("%s should be numerically grounded: %v", id, err)
		}
		if scored := score(sample, result, true); scored.Strict || !scored.WrongDraft {
			t.Fatalf("%s: grounded quantities incorrectly treated as correct intent", id)
		}
	}
}
