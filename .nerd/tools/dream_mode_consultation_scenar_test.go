package tools

import (
	"context"
	"testing"
)

// mockRegistry is a mock implementation of ToolRegistry for testing
type mockRegistry struct {
	registeredTool Tool
	registerError  error
}

func (m *mockRegistry) Register(tool Tool) error {
	m.registeredTool = tool
	return m.registerError
}

func TestDreamModeConsultationScenar(t *testing.T) {
	ctx := context.Background()

	t.Run("Happy Path", func(t *testing.T) {
		input := "any input"
		result, err := dreamModeConsultationScenar(ctx, input)

		if err != nil {
			t.Fatalf("Expected no error, got %v", err)
		}

		if result == "" {
			t.Fatal("Expected non-empty result")
		}

		// Verify key sections are present in the consultation
		expectedSections := []string{
			"DREAM MODE CONSULTATION",
			"HYPOTHETICAL ACTION",
			"WHAT EACH agents.md WOULD CONTAIN",
			"IMPLICATIONS",
			"POTENTIAL CHALLENGES",
			"RECOMMENDATION",
			"no files were created or modified",
		}

		for _, section := range expectedSections {
			if !contains(result, section) {
				t.Errorf("Expected result to contain section '%s'", section)
			}
		}
	})

	t.Run("Empty Input", func(t *testing.T) {
		result, err := dreamModeConsultationScenar(ctx, "")

		if err != nil {
			t.Fatalf("Expected no error with empty input, got %v", err)
		}

		if result == "" {
			t.Fatal("Expected non-empty result with empty input")
		}
	})

	t.Run("Nil Context", func(t *testing.T) {
		result, err := dreamModeConsultationScenar(nil, "input")

		if err != nil {
			t.Fatalf("Expected no error with nil context, got %v", err)
		}

		if result == "" {
			t.Fatal("Expected non-empty result with nil context")
		}
	})
}

func TestRegisterDreamModeConsultationScenar(t *testing.T) {
	t.Run("Successful Registration", func(t *testing.T) {
		registry := &mockRegistry{registerError: nil}

		err := RegisterDreamModeConsultationScenar(registry)

		if err != nil {
			t.Fatalf("Expected no error during registration, got %v", err)
		}

		// Verify the tool was registered with correct properties
		if registry.registeredTool.Name != "dream_mode_consultation_scenar" {
			t.Errorf("Expected tool name 'dream_mode_consultation_scenar', got '%s'", registry.registeredTool.Name)
		}

		if registry.registeredTool.Description != DreamModeConsultationScenarDescription {
			t.Error("Tool description does not match expected constant")
		}

		if registry.registeredTool.Handler == nil {
			t.Error("Expected non-nil handler")
		}
	})

	t.Run("Nil Registry Error", func(t *testing.T) {
		err := RegisterDreamModeConsultationScenar(nil)

		if err == nil {
			t.Fatal("Expected error for nil registry")
		}

		expectedError := "tool registry cannot be nil"
		if err.Error() != expectedError {
			t.Errorf("Expected error '%s', got '%s'", expectedError, err.Error())
		}
	})

	t.Run("Registry Register Error", func(t *testing.T) {
		expectedErr := "registration failed"
		registry := &mockRegistry{registerError: fmt.Errorf(expectedErr)}

		err := RegisterDreamModeConsultationScenar(registry)

		if err == nil {
			t.Fatal("Expected error from registry")
		}

		if err.Error() != expectedErr {
			t.Errorf("Expected error '%s', got '%s'", expectedErr, err.Error())
		}
	})
}

// contains is a helper function to check if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || 
		(len(s) > len(substr) && indexOf(s, substr) >= 0))
}

// indexOf is a helper function to find the index of a substring
func indexOf(s, substr string) int {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}