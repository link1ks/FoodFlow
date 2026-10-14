package core

import (
	"regexp"
	"strings"
	"testing"
)

func FuzzQuantityRejectsMalformedDecimals(f *testing.F) {
	for _, input := range []string{"1.-1", "1.+1", "0.+1", "+1", "1.250", "0.001", "1.0001", "１.２"} {
		f.Add(input)
	}
	syntax := regexp.MustCompile(`^[0-9]{1,15}(\.[0-9]{1,3})?$`)
	f.Fuzz(func(t *testing.T, input string) {
		q, err := Quantity(input)
		if err == nil {
			if !syntax.MatchString(strings.TrimSpace(input)) || q <= 0 {
				t.Fatalf("invalid positive decimal accepted: %q -> %d", input, q)
			}
			if roundTrip, err := Quantity(Format(q)); err != nil || roundTrip != q {
				t.Fatalf("accepted quantity lost precision: %q", input)
			}
		}
	})
}

func FuzzQuantityRoundTrip(f *testing.F) {
	for _, seed := range []uint64{1, 999, 1000, 300001, 999999999999999} {
		f.Add(seed)
	}
	f.Fuzz(func(t *testing.T, seed uint64) {
		q := int64(seed%999999999999999) + 1
		actual, err := Quantity(Format(q))
		if err != nil || actual != q {
			t.Fatalf("round trip %d: %d %v", q, actual, err)
		}
	})
}

func FuzzConversionAndServingConservation(f *testing.F) {
	f.Add(uint32(300), uint8(2), uint8(2))
	f.Add(uint32(1), uint8(3), uint8(7))
	f.Fuzz(func(t *testing.T, raw uint32, count, baseRaw uint8) {
		q := int64(raw%1000000+1) * 1000
		for _, units := range [][2]string{{"kg", "g"}, {"l", "ml"}} {
			converted, err := Convert(q, units[0], units[1])
			if err != nil {
				t.Fatal(err)
			}
			back, err := Convert(converted, units[1], units[0])
			if err != nil || back != q {
				t.Fatal("conversion lost quantity")
			}
		}
		for _, units := range [][2]string{{"g", "ml"}, {"ml", "个"}, {"个", "只"}} {
			if _, err := Convert(q, units[0], units[1]); err == nil {
				t.Fatal("incompatible unit conversion accepted")
			}
		}
		servings, base := int(count%20)+1, int(baseRaw%20)+1
		scaled, err := Scale(q, servings, base)
		if err != nil {
			t.Fatal(err)
		}
		if scaled*int64(base) < q*int64(servings) || (scaled-1)*int64(base) >= q*int64(servings) {
			t.Fatal("serving rounding violated minimal nonnegative coverage")
		}
	})
}
