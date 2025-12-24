package chat

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseGeneratedRequirements(t *testing.T) {
	response := `
# Requirements
- [F] Support auth (must)
- [NF] Fast responses (nice)
* [C] Use Go
- Provide docs
`
	reqs := parseGeneratedRequirements(response, 0)
	if len(reqs) != 4 {
		t.Fatalf("expected 4 requirements, got %d", len(reqs))
	}
	if reqs[0].Type != "functional" || reqs[0].Priority != "must-have" {
		t.Fatalf("unexpected first requirement: %+v", reqs[0])
	}
	if reqs[1].Type != "non-functional" || reqs[1].Priority != "nice-to-have" {
		t.Fatalf("unexpected second requirement: %+v", reqs[1])
	}
	if reqs[2].Type != "constraint" {
		t.Fatalf("unexpected third requirement: %+v", reqs[2])
	}
	if !strings.HasPrefix(reqs[0].ID, "REQ-") {
		t.Fatalf("expected requirement ID to be set")
	}
}

func TestLoadExistingNorthstar(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("failed to create .nerd: %v", err)
	}

	wizard := &NorthstarWizardState{
		Mission: "ship",
	}
	data, err := json.Marshal(wizard)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	path := filepath.Join(workspace, ".nerd", "northstar.json")
	if err := os.WriteFile(path, data, 0644); err != nil {
		t.Fatalf("failed to write file: %v", err)
	}

	loaded, ok := loadExistingNorthstar(workspace)
	if !ok || loaded == nil {
		t.Fatalf("expected to load existing northstar")
	}
	if loaded.Mission != "ship" {
		t.Fatalf("expected mission to load")
	}
	if loaded.Personas == nil || loaded.Requirements == nil || loaded.Constraints == nil {
		t.Fatalf("expected slices to be initialized")
	}
}
