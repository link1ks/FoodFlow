package intake

import (
	"encoding/json"
	"errors"
	"io"
	"strings"
	"unicode/utf8"
)

// Proposal is an experimental model suggestion, never an inventory command.
// AmountQuote must quote source text; quantities are derived by the pure parser.
type Proposal struct {
	Decision string         `json:"decision"`
	Items    []ProposalItem `json:"items"`
}

type ProposalItem struct {
	CatalogID   string `json:"catalog_id"`
	AmountQuote string `json:"amount_quote"`
}

type ProposalResult struct {
	Decision string  `json:"decision"`
	Items    []Draft `json:"items"`
}

// ValidateProposal only validates a proposal's structure and quantity provenance.
// A quoted amount does not prove that its ingredient or correction scope is right.
// Callers must preserve clarification and explicit user confirmation.
func ValidateProposal(text, raw string, catalog []CatalogItem) (ProposalResult, error) {
	invalid := func() (ProposalResult, error) {
		return ProposalResult{Decision: "clarify", Items: []Draft{}}, errors.New("invalid or ungrounded intake proposal")
	}
	if len(raw) > 16384 || !utf8.ValidString(raw) {
		return invalid()
	}
	decoder := json.NewDecoder(strings.NewReader(raw))
	decoder.DisallowUnknownFields()
	var proposal Proposal
	if decoder.Decode(&proposal) != nil || decoder.Decode(new(any)) != io.EOF || proposal.Items == nil || len(proposal.Items) > 20 {
		return invalid()
	}
	if proposal.Decision != "draft" && proposal.Decision != "clarify" && proposal.Decision != "noop" {
		return invalid()
	}
	if (proposal.Decision == "noop" && len(proposal.Items) != 0) || (proposal.Decision == "draft" && len(proposal.Items) == 0) {
		return invalid()
	}
	items := map[string]CatalogItem{}
	for _, item := range catalog {
		items[item.ID] = item
	}
	result := ProposalResult{Decision: proposal.Decision, Items: []Draft{}}
	seen := map[string]bool{}
	for _, proposed := range proposal.Items {
		item, ok := items[proposed.CatalogID]
		quote := proposed.AmountQuote
		if !ok || seen[item.ID] || len([]rune(quote)) > 80 || (quote != "" && !strings.Contains(text, quote)) {
			return invalid()
		}
		seen[item.ID] = true
		draft := Draft{CatalogID: item.ID, Name: item.Name, Unit: item.Unit, Heard: quote}
		if proposal.Decision == "clarify" {
			draft.Warning = "模型要求澄清，请核对原文后填写。"
		} else {
			if quote == "" {
				return invalid()
			}
			parsed := Parse(item.Name+quote, []CatalogItem{item})
			if len(parsed.Items) != 1 || parsed.Items[0].Quantity == "" {
				return invalid()
			}
			draft.Quantity = parsed.Items[0].Quantity
		}
		result.Items = append(result.Items, draft)
	}
	return result, nil
}
