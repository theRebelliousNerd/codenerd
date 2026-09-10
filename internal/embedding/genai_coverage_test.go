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

// Interface compliance is a compile-time property. These were runtime tests
// that assigned the engine to an interface variable and then checked it for
// nil — but the assignment is what proves compliance, and it fails the build if
// it does not hold. The nil check could never fire, so it asserted nothing and
// the real guarantee only ran when someone ran the tests.
var (
	_ EmbeddingEngine          = (*GenAIEngine)(nil)
	_ TaskTypeAwareEngine      = (*GenAIEngine)(nil)
	_ TaskTypeBatchAwareEngine = (*GenAIEngine)(nil)
)

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
