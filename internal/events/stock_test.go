package events

import (
	"encoding/json"
	"testing"
	"time"
)

func TestStockValidation(t *testing.T) {
	event := Stock{Version: 1, ID: "8a2c781b-7bb6-4dd3-81f8-985a97b9d55a", HouseholdID: "8917d14e-c1b8-454c-9f10-f033bb427788", Ingredient: "番茄", Unit: "g", Delta: 1000, Reason: "purchase", OccurredAt: time.Now()}
	raw, _ := json.Marshal(event)
	if _, err := DecodeStock(raw); err != nil {
		t.Fatal(err)
	}
	for _, mutate := range []func(*Stock){func(e *Stock) { e.Version = 2 }, func(e *Stock) { e.ID = "invalid" }, func(e *Stock) { e.Delta = -1 }, func(e *Stock) { e.Reason = "sql" }, func(e *Stock) { e.OccurredAt = time.Time{} }} {
		bad := event
		mutate(&bad)
		raw, _ := json.Marshal(bad)
		if _, err := DecodeStock(raw); err == nil {
			t.Fatal("invalid event accepted")
		}
	}
	raw, _ = json.Marshal(event)
	raw = append(raw, []byte(" {}")...)
	if _, err := DecodeStock(raw); err == nil {
		t.Fatal("trailing data accepted")
	}
}
