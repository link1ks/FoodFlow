// Package nutrition exposes versioned reference composition, never batch measurements.
package nutrition

import (
	_ "embed"
	"encoding/json"
)

//go:embed profiles.json
var data []byte

type Profile struct {
	Status        string            `json:"status"`
	Basis         string            `json:"basis"`
	ReferenceFood string            `json:"reference_food,omitempty"`
	Source        string            `json:"source,omitempty"`
	SourceID      string            `json:"source_id,omitempty"`
	SourceURL     string            `json:"source_url,omitempty"`
	Note          string            `json:"note"`
	Nutrients     map[string]string `json:"nutrients"`
}

var profiles = func() map[string]Profile {
	var p map[string]Profile
	if err := json.Unmarshal(data, &p); err != nil {
		panic(err)
	}
	return p
}()

func Lookup(name string) Profile {
	if p, ok := profiles[name]; ok {
		return p
	}
	return Profile{Status: "unavailable", Basis: "每100克可食部", Note: "尚未匹配已核验的参考食材，请核对名称或产品营养标签。", Nutrients: map[string]string{}}
}
