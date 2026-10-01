package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type QualityCheck struct {
	Path   string `json:"path"`
	Anchor string `json:"anchor"`
}
type QualityDebt struct {
	ID          string `json:"id"`
	Priority    string `json:"priority"`
	Status      string `json:"status"`
	Description string `json:"description"`
	Completion  string `json:"completion"`
	Evidence    string `json:"evidence,omitempty"`
}
type QualityDomain struct {
	ID         string         `json:"id"`
	Owner      string         `json:"owner"`
	Reviewed   string         `json:"reviewed"`
	Acceptance []string       `json:"acceptance"`
	Checks     []QualityCheck `json:"checks"`
	Debts      []QualityDebt  `json:"debts"`
}

func quality(root string) error {
	raw, err := os.ReadFile(filepath.Join(root, "docs/quality/domains.json"))
	if err != nil {
		return err
	}
	var domains []QualityDomain
	if err = json.Unmarshal(raw, &domains); err != nil {
		return err
	}
	expected := map[string]bool{"inventory": false, "permissions": false, "menus": false, "procurement": false, "ai": false, "insights": false, "nutrition": false, "frontend": false, "harness": false}
	for _, domain := range domains {
		if _, ok := expected[domain.ID]; !ok || expected[domain.ID] {
			return fmt.Errorf("unknown/duplicate quality domain: %s", domain.ID)
		}
		expected[domain.ID] = true
	}
	for id, present := range expected {
		if !present {
			return fmt.Errorf("missing quality domain: %s", id)
		}
	}
	return validateQuality(root, domains)
}

func validateQuality(root string, domains []QualityDomain) error {
	debts := map[string]bool{}
	for _, domain := range domains {
		if domain.Owner == "" || len(domain.Acceptance) == 0 || len(domain.Checks) == 0 {
			return fmt.Errorf("quality domain %s needs ownership, review date, acceptance and checks", domain.ID)
		}
		if _, err := time.Parse("2006-01-02", domain.Reviewed); err != nil {
			return fmt.Errorf("quality %s: invalid review date", domain.ID)
		}
		for _, criterion := range domain.Acceptance {
			if strings.TrimSpace(criterion) == "" {
				return fmt.Errorf("quality %s: empty acceptance criterion", domain.ID)
			}
		}
		for _, check := range domain.Checks {
			if check.Path == "" || check.Anchor == "" {
				return fmt.Errorf("quality %s: check needs file and scenario anchor", domain.ID)
			}
		}
		paths := append([]QualityCheck{{Path: domain.Owner}}, domain.Checks...)
		for _, check := range paths {
			clean := filepath.Clean(check.Path)
			if filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
				return fmt.Errorf("quality path escapes repository: %s", check.Path)
			}
			info, err := os.Stat(filepath.Join(root, clean))
			if err != nil {
				return fmt.Errorf("quality %s: missing %s", domain.ID, check.Path)
			}
			if check.Anchor != "" {
				if info.IsDir() {
					return fmt.Errorf("check must be file: %s", check.Path)
				}
				raw, err := os.ReadFile(filepath.Join(root, clean))
				if err != nil {
					return err
				}
				if !strings.Contains(string(raw), check.Anchor) {
					return fmt.Errorf("quality %s: missing scenario %q in %s", domain.ID, check.Anchor, check.Path)
				}
			}
		}
		for _, debt := range domain.Debts {
			if debt.ID == "" || debts[debt.ID] || debt.Description == "" || debt.Completion == "" || !strings.Contains("|P1|P2|P3|", "|"+debt.Priority+"|") || !strings.Contains("|open|closed|", "|"+debt.Status+"|") {
				return fmt.Errorf("invalid quality debt in %s: %s", domain.ID, debt.ID)
			}
			debts[debt.ID] = true
			if debt.Status == "closed" {
				clean := filepath.Clean(debt.Evidence)
				if debt.Evidence == "" || filepath.IsAbs(clean) || clean == ".." || strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
					return fmt.Errorf("closed debt %s needs repository evidence", debt.ID)
				}
				info, err := os.Stat(filepath.Join(root, clean))
				if err != nil || info.IsDir() {
					return fmt.Errorf("closed debt %s: missing evidence file", debt.ID)
				}
			}
		}
	}
	return nil
}
