package intake

import "testing"

func TestProposalGroundingAndReviewBoundary(t *testing.T) {
	catalog := []CatalogItem{{ID: "egg", Name: "鸡蛋", Unit: "个"}, {ID: "milk", Name: "牛奶", Unit: "ml"}}
	text := "鸡蛋三个，不对，改成四个；牛奶半升。"
	good := `{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":"四个"},{"catalog_id":"milk","amount_quote":"半升"}]}`
	got, err := ValidateProposal(text, good, catalog)
	if err != nil || got.Items[0].Quantity != "4.000" || got.Items[1].Quantity != "500.000" {
		t.Fatal("source quantities not derived precisely", got, err)
	}
	for _, raw := range []string{
		`{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":"五个"}]}`,
		`{"decision":"draft","items":[{"catalog_id":"other","amount_quote":"四个"}]}`,
		`{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":"四个","quantity":"40"}]}`,
		`{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":"四个"},{"catalog_id":"egg","amount_quote":"三个"}]}`,
		`{"decision":"draft","items":[]}`,
		`{"decision":"noop","items":[{"catalog_id":"egg","amount_quote":"四个"}]}`,
		`{"decision":"draft","items":null}`,
		`{"decision":"draft","items":[{"catalog_id":"milk","amount_quote":"四个"}]}`,
		`{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":""}]}`,
		`{"decision":"draft","items":[{"catalog_id":"egg","amount_quote":"四个"}]} {}`,
	} {
		if out, err := ValidateProposal(text, raw, catalog); err == nil || out.Decision != "clarify" || len(out.Items) != 0 {
			t.Fatalf("untrusted proposal accepted: %s", raw)
		}
	}
	for _, raw := range []string{`{"decision":"noop","items":[]}`, `{"decision":"clarify","items":[{"catalog_id":"egg","amount_quote":"四个"}]}`} {
		out, err := ValidateProposal(text, raw, catalog)
		if err != nil {
			t.Fatal(err)
		}
		for _, item := range out.Items {
			if item.Quantity != "" {
				t.Fatal("clarification prefilled stock")
			}
		}
	}
}
