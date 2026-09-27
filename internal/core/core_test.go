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
