package nutrition

import (
	"math/big"
	"testing"
)

func TestReferenceProfiles(t *testing.T) {
	if len(profiles) != 91 {
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
	if count != 79 {
		t.Fatal(count)
	}
	if Lookup("番茄").Nutrients["energy_kcal"] != "18" {
		t.Fatal("wrong USDA tomato mapping")
	}
	if Lookup("馒头").Status != "unavailable" || len(Lookup("自定义食材").Nutrients) != 0 {
		t.Fatal("unknown composition fabricated")
	}
}
