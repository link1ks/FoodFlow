package core

import "testing"

func TestQuantityAndUnits(t *testing.T) {
	q, e := Quantity("1.2345")
	if e == nil || q != 0 {
		t.Fatal("precision accepted")
	}
	q, e = Quantity("1.25")
	if e != nil || q != 1250 {
		t.Fatal(q, e)
	}
	if _, e = Convert(1000, "个", "g"); e == nil {
		t.Fatal("invalid conversion")
	}
	if _, e = Convert(1000, "个", "只"); e == nil {
		t.Fatal("count conversion")
	}
	q, e = Convert(1250, "kg", "g")
	if e != nil || q != 1250000 {
		t.Fatal(q, e)
	}
}
func TestScale(t *testing.T) {
	q, e := Scale(1001, 3, 2)
	if e != nil || q != 1502 {
		t.Fatal(q, e)
	}
}

func TestQuantityDecimalSyntax(t *testing.T) {
	for _, input := range []string{"1.-1", "1.+1", "0.+1", "1.-01", "+1", "-1", "1e3", ".5", "1.", "1.2.3", "１.２", "1.0001", "1000000000000000", "0", "0.000"} {
		t.Run(input, func(t *testing.T) {
			if q, err := Quantity(input); err == nil || q != 0 {
				t.Fatalf("invalid decimal %q accepted: %d, %v", input, q, err)
			}
		})
	}
	for input, want := range map[string]int64{"1": 1000, "1.2": 1200, "1.23": 1230, "1.234": 1234, "0.001": 1, "0001.250": 1250, " 1.250 ": 1250, "999999999999999.999": 999999999999999999} {
		if got, err := Quantity(input); err != nil || got != want {
			t.Fatalf("valid decimal %q: got %d, %v; want %d", input, got, err, want)
		}
	}
}
