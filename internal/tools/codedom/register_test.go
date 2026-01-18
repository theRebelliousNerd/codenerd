package codedom

import (
	"codenerd/internal/tools"
	"testing"
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
