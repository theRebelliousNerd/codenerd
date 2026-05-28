package embedding

import "testing"

// =============================================================================
// NewGenAIEngine Tests
// =============================================================================

func TestNewGenAIEngine_WhenNoAPIKey_ShouldReturnError(t *testing.T) {
	engine, err := NewGenAIEngine("", "gemini-embedding-001", "SEMANTIC_SIMILARITY")
	if err == nil {
		t.Fatal("NewGenAIEngine with empty API key should return error")
	}
	if engine != nil {
		t.Fatal("NewGenAIEngine with empty API key should return nil")
	}
	expectedMsg := "GenAI API key is required"
	if err.Error() != expectedMsg {
		t.Errorf("error = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestGenAIEngine_Dimensions_ShouldReturn3072(t *testing.T) {
	// We can't create a real engine without API key, so test the method directly
	engine := &GenAIEngine{model: "test-model"}
	if engine.Dimensions() != 3072 {
		t.Errorf("Dimensions() = %d, want 3072", engine.Dimensions())
	}
}

func TestGenAIEngine_Name_ShouldIncludeModel(t *testing.T) {
	tests := []struct {
		name     string
		model    string
		expected string
	}{
		{"default model", "gemini-embedding-001", "genai:gemini-embedding-001"},
		{"custom model", "text-embedding-004", "genai:text-embedding-004"},
		{"empty model", "", "genai:"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			engine := &GenAIEngine{model: tt.model}
			if engine.Name() != tt.expected {
				t.Errorf("Name() = %q, want %q", engine.Name(), tt.expected)
			}
		})
	}
}

func TestGenAIEngine_Close_ShouldReturnNil(t *testing.T) {
	engine := &GenAIEngine{model: "test"}
	err := engine.Close()
	if err != nil {
		t.Fatalf("Close() returned error: %v", err)
	}
}

// =============================================================================
// GenAI Interface Compliance
// =============================================================================

func TestGenAIEngine_ImplementsEmbeddingEngine(t *testing.T) {
	engine := &GenAIEngine{model: "test"}
	var ee EmbeddingEngine = engine
	if ee == nil {
		t.Fatal("GenAIEngine should implement EmbeddingEngine")
	}
}

func TestGenAIEngine_ImplementsTaskTypeAwareEngine(t *testing.T) {
	engine := &GenAIEngine{model: "test"}
	var tta TaskTypeAwareEngine = engine
	if tta == nil {
		t.Fatal("GenAIEngine should implement TaskTypeAwareEngine")
	}
}

func TestGenAIEngine_ImplementsTaskTypeBatchAwareEngine(t *testing.T) {
	engine := &GenAIEngine{model: "test"}
	var ttba TaskTypeBatchAwareEngine = engine
	if ttba == nil {
		t.Fatal("GenAIEngine should implement TaskTypeBatchAwareEngine")
	}
}

// =============================================================================
// int32Ptr Tests
// =============================================================================

func TestInt32Ptr_ShouldReturnPointerToValue(t *testing.T) {
	tests := []struct {
		name  string
		input int32
	}{
		{"zero", 0},
		{"positive", 3072},
		{"negative", -1},
		{"max", 2147483647},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ptr := new(tt.input)
			if ptr == nil {
				t.Fatal("int32Ptr returned nil")
			}
			if *ptr != tt.input {
				t.Errorf("*int32Ptr(%d) = %d", tt.input, *ptr)
			}
		})
	}
}

// =============================================================================
// maxBatchSize Tests
// =============================================================================

func TestMaxBatchSize_ShouldBe100(t *testing.T) {
	if maxBatchSize != 100 {
		t.Errorf("maxBatchSize = %d, want 100", maxBatchSize)
	}
}
