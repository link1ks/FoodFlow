package nutrition

import (
	"math/big"
	"testing"
)

func TestReferenceProfiles(t *testing.T) {
	if len(profiles) != 200 {
		t.Fatal(len(profiles))
	}
	count := 0
	for name, p := range profiles {
		if p.Note == "" || p.Basis != "每100克可食部" {
			t.Fatal(name, p)
		}
		if p.Status == "reference" {
			count++
			if p.SourceID == "" || p.ReferenceFood == "" || p.SourceURL == "" {
				t.Fatal(name, p)
			}
			for key, v := range p.Nutrients {
				n, ok := new(big.Rat).SetString(v)
				if !ok || n.Sign() < 0 {
					t.Fatal(name, key, v)
				}
			}
		} else if len(p.Nutrients) != 0 {
			t.Fatal("unknown values invented")
		}
	}
	if count != 198 {
		t.Fatal(count)
	}
	if Lookup("番茄").Nutrients["energy_kcal"] != "18" {
		t.Fatal("wrong USDA tomato mapping")
	}
	if Lookup("鱼肉").Status != "unavailable" || len(Lookup("自定义食材").Nutrients) != 0 {
		t.Fatal("unknown composition fabricated")
	}
}

func TestExtendedReferenceProvenance(t *testing.T) {
	for _, name := range []string{"芦笋", "杨梅", "腐竹", "千张", "荠菜", "山楂", "白玉菇", "料酒", "馒头"} {
		p := Lookup(name)
		if p.Status != "reference" || p.SourceID == "" || p.Nutrients["energy_kcal"] == "" {
			t.Fatal(name, p)
		}
	}
	if Lookup("腐竹").Nutrients["energy_kcal"] != "461" || Lookup("腐竹").Nutrients["protein_g"] != "44.6" {
		t.Fatal("wrong CFS reference")
	}
	if Lookup("腐竹").Nutrients["fiber_g"] != "" || Lookup("腐竹").Nutrients["insoluble_fiber_g"] != "1.0" {
		t.Fatal("insoluble fibre mislabeled as total fibre")
	}
	if Lookup("料酒").Nutrients["carbs_g"] != "" {
		t.Fatal("missing measurement replaced by invented zero")
	}
}
