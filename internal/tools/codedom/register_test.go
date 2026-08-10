package codedom

import (
	"testing"

	"codenerd/internal/tools"
)

func TestRegisterAll(t *testing.T) {
	t.Parallel()

	registry := tools.NewRegistry()
	err := RegisterAll(registry)
	if err != nil {
		t.Fatalf("RegisterAll failed: %v", err)
	}

	expectedTools := []string{
		"get_elements",
		"get_element",
		"edit_lines",
		"insert_lines",
		"delete_lines",
		"apply_edits",
		"run_impacted_tests",
		"get_impacted_tests",
	}

	for _, name := range expectedTools {
		tool := registry.Get(name)
		if tool == nil {
			t.Errorf("expected tool %q to be registered", name)
		}
	}
}
