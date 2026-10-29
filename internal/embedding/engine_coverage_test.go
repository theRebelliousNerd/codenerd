package embedding

import (
	"math"
	"testing"
)

// =============================================================================
// DefaultConfig Tests
// =============================================================================

func TestDefaultConfig_WhenCalled_ShouldReturnSensibleDefaults(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Provider != "ollama" {
		t.Errorf("DefaultConfig().Provider = %q, want %q", cfg.Provider, "ollama")
	}
	if cfg.OllamaEndpoint != "http://localhost:11434" {
		t.Errorf("DefaultConfig().OllamaEndpoint = %q, want %q", cfg.OllamaEndpoint, "http://localhost:11434")
	}
	// A model is the user's to name; the defaults hold none.
	if cfg.OllamaModel != "" || cfg.GenAIModel != "" {
		t.Errorf("DefaultConfig() invented a model: ollama=%q genai=%q", cfg.OllamaModel, cfg.GenAIModel)
	}
	if cfg.TaskType != "SEMANTIC_SIMILARITY" {
		t.Errorf("DefaultConfig().TaskType = %q, want %q", cfg.TaskType, "SEMANTIC_SIMILARITY")
	}
	if cfg.GenAIAPIKey != "" {
		t.Errorf("DefaultConfig().GenAIAPIKey = %q, want empty", cfg.GenAIAPIKey)
	}
}

// =============================================================================
// NewEngine Factory Tests
// =============================================================================

func TestNewEngine_WhenUnsupportedProvider_ShouldReturnError(t *testing.T) {
	cfg := Config{Provider: "nonexistent"}
	engine, err := NewEngine(cfg)
	if err == nil {
		t.Fatal("NewEngine with unsupported provider should return error")
	}
	if engine != nil {
		t.Fatal("NewEngine with unsupported provider should return nil engine")
	}
	expectedMsg := "unsupported embedding provider: nonexistent (use 'ollama' or 'genai')"
	if err.Error() != expectedMsg {
		t.Errorf("error message = %q, want %q", err.Error(), expectedMsg)
	}
}

func TestNewEngine_WhenEmptyProvider_ShouldReturnError(t *testing.T) {
	cfg := Config{Provider: ""}
	engine, err := NewEngine(cfg)
	if err == nil {
		t.Fatal("NewEngine with empty provider should return error")
	}
	if engine != nil {
		t.Fatal("NewEngine with empty provider should return nil engine")
	}
}

func TestNewEngine_WhenOllamaProvider_ShouldCreateEngine(t *testing.T) {
	cfg := Config{
		Provider:       "ollama",
		OllamaEndpoint: "http://localhost:11434",
		OllamaModel:    "test-model",
		Dimensions:     768,
	}
	engine, err := NewEngine(cfg)
	if err != nil {
		t.Fatalf("NewEngine(ollama) returned error: %v", err)
	}
	if engine == nil {
		t.Fatal("NewEngine(ollama) returned nil engine")
	}
	if engine.Name() != "ollama:test-model" {
		t.Errorf("engine.Name() = %q, want %q", engine.Name(), "ollama:test-model")
	}
}

func TestNewEngine_WhenGenAIProviderNoKey_ShouldReturnError(t *testing.T) {
	cfg := Config{
		Provider:   "genai",
		GenAIModel: "gemini-embedding-001",
		// No API key
	}
	engine, err := NewEngine(cfg)
	if err == nil {
		t.Fatal("NewEngine(genai) with no API key should return error")
	}
	if engine != nil {
		t.Fatal("NewEngine(genai) with no API key should return nil engine")
	}
}

// =============================================================================
// CosineSimilarity Tests
// =============================================================================

func TestCosineSimilarity_WhenIdenticalVectors_ShouldReturnOne(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{1.0, 2.0, 3.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("CosineSimilarity(identical) = %f, want 1.0", sim)
	}
}

func TestCosineSimilarity_WhenOppositeVectors_ShouldReturnNegativeOne(t *testing.T) {
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{-1.0, 0.0, 0.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(sim-(-1.0)) > 1e-6 {
		t.Errorf("CosineSimilarity(opposite) = %f, want -1.0", sim)
	}
}

func TestCosineSimilarity_WhenOrthogonalVectors_ShouldReturnZero(t *testing.T) {
	a := []float32{1.0, 0.0, 0.0}
	b := []float32{0.0, 1.0, 0.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(sim) > 1e-6 {
		t.Errorf("CosineSimilarity(orthogonal) = %f, want 0.0", sim)
	}
}

func TestCosineSimilarity_WhenDimensionMismatch_ShouldReturnError(t *testing.T) {
	a := []float32{1.0, 2.0}
	b := []float32{1.0, 2.0, 3.0}

	sim, err := CosineSimilarity(a, b)
	if err == nil {
		t.Fatal("CosineSimilarity with mismatched dimensions should return error")
	}
	if sim != 0 {
		t.Errorf("CosineSimilarity with error should return 0, got %f", sim)
	}
}

func TestCosineSimilarity_WhenZeroVectorA_ShouldReturnZero(t *testing.T) {
	a := []float32{0.0, 0.0, 0.0}
	b := []float32{1.0, 2.0, 3.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity with zero vector should not error: %v", err)
	}
	if sim != 0 {
		t.Errorf("CosineSimilarity with zero vector = %f, want 0.0", sim)
	}
}

func TestCosineSimilarity_WhenZeroVectorB_ShouldReturnZero(t *testing.T) {
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{0.0, 0.0, 0.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity with zero vector should not error: %v", err)
	}
	if sim != 0 {
		t.Errorf("CosineSimilarity with zero vector = %f, want 0.0", sim)
	}
}

func TestCosineSimilarity_WhenBothZeroVectors_ShouldReturnZero(t *testing.T) {
	a := []float32{0.0, 0.0}
	b := []float32{0.0, 0.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity with both zero should not error: %v", err)
	}
	if sim != 0 {
		t.Errorf("CosineSimilarity(both zero) = %f, want 0.0", sim)
	}
}

func TestCosineSimilarity_WhenEmptyVectors_ShouldReturnZero(t *testing.T) {
	a := []float32{}
	b := []float32{}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity with empty vectors should not error: %v", err)
	}
	if sim != 0 {
		t.Errorf("CosineSimilarity(empty) = %f, want 0.0", sim)
	}
}

func TestCosineSimilarity_WhenSingleDimension_ShouldWorkCorrectly(t *testing.T) {
	a := []float32{3.0}
	b := []float32{5.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("CosineSimilarity(same direction, 1D) = %f, want 1.0", sim)
	}
}

func TestCosineSimilarity_WhenNegativeSingleDimension_ShouldReturnNegativeOne(t *testing.T) {
	a := []float32{3.0}
	b := []float32{-5.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(sim-(-1.0)) > 1e-6 {
		t.Errorf("CosineSimilarity(opposite direction, 1D) = %f, want -1.0", sim)
	}
}

func TestCosineSimilarity_WhenScaledVectors_ShouldReturnOne(t *testing.T) {
	// Cosine similarity is magnitude-invariant
	a := []float32{1.0, 2.0, 3.0}
	b := []float32{2.0, 4.0, 6.0}

	sim, err := CosineSimilarity(a, b)
	if err != nil {
		t.Fatalf("CosineSimilarity returned error: %v", err)
	}
	if math.Abs(sim-1.0) > 1e-6 {
		t.Errorf("CosineSimilarity(scaled) = %f, want 1.0", sim)
	}
}

// =============================================================================
// Config Tests
// =============================================================================

func TestConfig_WhenAllFieldsSet_ShouldRetainValues(t *testing.T) {
	cfg := Config{
		Provider:       "genai",
		OllamaEndpoint: "http://custom:1234",
		OllamaModel:    "custom-model",
		GenAIAPIKey:    "test-key-123",
		GenAIModel:     "custom-genai-model",
		TaskType:       "RETRIEVAL_QUERY",
	}

	if cfg.Provider != "genai" {
		t.Errorf("Provider = %q", cfg.Provider)
	}
	if cfg.OllamaEndpoint != "http://custom:1234" {
		t.Errorf("OllamaEndpoint = %q", cfg.OllamaEndpoint)
	}
	if cfg.OllamaModel != "custom-model" {
		t.Errorf("OllamaModel = %q", cfg.OllamaModel)
	}
	if cfg.GenAIAPIKey != "test-key-123" {
		t.Errorf("GenAIAPIKey = %q", cfg.GenAIAPIKey)
	}
	if cfg.GenAIModel != "custom-genai-model" {
		t.Errorf("GenAIModel = %q", cfg.GenAIModel)
	}
	if cfg.TaskType != "RETRIEVAL_QUERY" {
		t.Errorf("TaskType = %q", cfg.TaskType)
	}
}

func TestValidateEmbeddingVectorRejectsInvalidProviderOutput(t *testing.T) {
	tests := []struct {
		name    string
		values  []float32
		wantErr bool
	}{
		{name: "valid", values: []float32{0.1, -0.2, 0.3}},
		{name: "empty", values: nil, wantErr: true},
		{name: "nan", values: []float32{1, float32(math.NaN())}, wantErr: true},
		{name: "positive infinity", values: []float32{float32(math.Inf(1))}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmbeddingVector(tt.values)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateEmbeddingVector(%v) error = %v, wantErr %v", tt.values, err, tt.wantErr)
			}
		})
	}
}

func TestValidateEmbeddingBatchResponseEnforcesCardinalityAndShape(t *testing.T) {
	tests := []struct {
		name       string
		want       int
		embeddings [][]float32
		wantErr    bool
	}{
		{name: "empty input", want: 0, embeddings: nil},
		{name: "valid", want: 2, embeddings: [][]float32{{1, 2}, {3, 4}}},
		{name: "truncated response", want: 2, embeddings: [][]float32{{1, 2}}, wantErr: true},
		{name: "empty vector", want: 1, embeddings: [][]float32{{}}, wantErr: true},
		{name: "mixed dimensions", want: 2, embeddings: [][]float32{{1, 2}, {3}}, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateEmbeddingBatchResponse(tt.want, tt.embeddings)
			if (err != nil) != tt.wantErr {
				t.Fatalf("validateEmbeddingBatchResponse(%d, %v) error = %v, wantErr %v", tt.want, tt.embeddings, err, tt.wantErr)
			}
		})
	}
}
