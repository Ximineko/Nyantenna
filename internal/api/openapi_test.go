package api

import (
	"os"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestOpenAPINyantennaYAMLValid(t *testing.T) {
	data, err := os.ReadFile("openapi.nyantenna.yaml")
	if err != nil {
		t.Fatalf("read openapi.nyantenna.yaml: %v", err)
	}
	var doc map[string]any
	if err := yaml.Unmarshal(data, &doc); err != nil {
		t.Fatalf("openapi.nyantenna.yaml is invalid YAML: %v", err)
	}
	if doc["openapi"] == "" {
		t.Fatalf("openapi.nyantenna.yaml missing openapi version")
	}
}
