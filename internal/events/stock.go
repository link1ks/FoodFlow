// Package events defines versioned contracts shared by independent services.
package events

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"
)

const StockTopic = "foodflow.stock.v1"

type Stock struct {
	Version     int       `json:"version"`
	ID          string    `json:"id"`
	HouseholdID string    `json:"household_id"`
	Ingredient  string    `json:"ingredient"`
	Category    string    `json:"category"`
	Unit        string    `json:"unit"`
	Delta       int64     `json:"delta_milli,string"`
	Reason      string    `json:"reason"`
	OccurredAt  time.Time `json:"occurred_at"`
}

func UUID(s string) bool {
	if len(s) != 36 || s[8] != '-' || s[13] != '-' || s[18] != '-' || s[23] != '-' {
		return false
	}
	_, err := hex.DecodeString(strings.ReplaceAll(s, "-", ""))
	return err == nil
}
func DecodeStock(raw []byte) (Stock, error) {
	var e Stock
	if len(raw) > 64<<10 {
		return e, errors.New("event exceeds 64KiB")
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&e); err != nil {
		return e, err
	}
	if err := dec.Decode(new(any)); err != io.EOF {
		return e, errors.New("trailing event content")
	}
	if e.Version != 1 || !UUID(e.ID) || !UUID(e.HouseholdID) || strings.TrimSpace(e.Ingredient) == "" || len(e.Ingredient) > 256 || len(e.Category) > 128 || len(e.Unit) > 32 || e.Unit == "" || e.OccurredAt.IsZero() || e.Delta == 0 || e.Delta == -1<<63 {
		return e, errors.New("invalid stock event")
	}
	switch e.Reason {
	case "purchase", "manual":
		if e.Delta < 0 {
			return e, errors.New("invalid inbound sign")
		}
	case "consume", "waste":
		if e.Delta > 0 {
			return e, errors.New("invalid outbound sign")
		}
	case "correction":
	default:
		return e, errors.New("unknown stock reason")
	}
	return e, nil
}
