package embedding

import (
	"testing"
)

func TestNewGenAIEngine_Defaults(t *testing.T) {
	// Passing a non-empty API key to allow client initialization to succeed
	// (or at least not fail on the validation check).
	engine, err := NewGenAIEngine("fake-api-key", "", "")
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if engine.model != "gemini-embedding-001" {
		t.Errorf("Expected default model 'gemini-embedding-001', got '%s'", engine.model)
	}

	if engine.taskType != "SEMANTIC_SIMILARITY" {
		t.Errorf("Expected default task type 'SEMANTIC_SIMILARITY', got '%s'", engine.taskType)
	}
}

func TestNewGenAIEngine_EmptyAPIKey(t *testing.T) {
	_, err := NewGenAIEngine("", "", "")
	if err == nil {
		t.Fatal("Expected error when API key is empty")
	}
	expectedErr := "GenAI API key is required"
	if err.Error() != expectedErr {
		t.Errorf("Expected error '%s', got '%v'", expectedErr, err)
	}
}

func TestGenAIEngine_Dimensions(t *testing.T) {
	engine := &GenAIEngine{}
	if engine.Dimensions() != 3072 {
		t.Errorf("Expected Dimensions to be 3072, got %d", engine.Dimensions())
	}
}

func TestGenAIEngine_Name(t *testing.T) {
	engine := &GenAIEngine{model: "gemini-embedding-001"}
	expectedName := "genai:gemini-embedding-001"
	if engine.Name() != expectedName {
		t.Errorf("Expected Name to be '%s', got '%s'", expectedName, engine.Name())
	}
}
