package app

import (
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"testing"
)

func TestOpenAPIDocument(t *testing.T) {
	raw, e := os.ReadFile(filepath.Join("..", "..", "openapi.yaml"))
	if e != nil {
		t.Fatal(e)
	}
	var spec struct {
		OpenAPI string         `yaml:"openapi"`
		Paths   map[string]any `yaml:"paths"`
	}
	if e = yaml.Unmarshal(raw, &spec); e != nil {
		t.Fatal(e)
	}
	if spec.OpenAPI == "" || len(spec.Paths) < 25 {
		t.Fatalf("incomplete OpenAPI document: %d paths", len(spec.Paths))
	}
}
