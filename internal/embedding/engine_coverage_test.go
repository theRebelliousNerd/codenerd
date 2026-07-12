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
	if cfg.OllamaModel != "embeddinggemma:300m" {
		t.Errorf("DefaultConfig().OllamaModel = %q, want %q", cfg.OllamaModel, "embeddinggemma:300m")
	}
	if cfg.GenAIModel != "gemini-embedding-001" {
		t.Errorf("DefaultConfig().GenAIModel = %q, want %q", cfg.GenAIModel, "gemini-embedding-001")
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
// FindTopK Tests
// =============================================================================

func TestFindTopK_WhenEmptyCorpus_ShouldReturnEmpty(t *testing.T) {
	query := []float32{1.0, 0.0, 0.0}
	corpus := [][]float32{}

	results, err := FindTopK(query, corpus, 5)
	if err != nil {
		t.Fatalf("FindTopK returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("FindTopK(empty corpus) returned %d results, want 0", len(results))
	}
}

func TestFindTopK_WhenKLargerThanCorpus_ShouldReturnAll(t *testing.T) {
	query := []float32{1.0, 0.0}
	corpus := [][]float32{
		{1.0, 0.0},
		{0.0, 1.0},
	}

	results, err := FindTopK(query, corpus, 10)
	if err != nil {
		t.Fatalf("FindTopK returned error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("FindTopK returned %d results, want 2", len(results))
	}
}

func TestFindTopK_WhenKIsZero_ShouldDefaultToTen(t *testing.T) {
	query := []float32{1.0, 0.0}
	// Create 15 corpus vectors
	corpus := make([][]float32, 15)
	for i := range corpus {
		corpus[i] = []float32{1.0, float32(i)}
	}

	results, err := FindTopK(query, corpus, 0)
	if err != nil {
		t.Fatalf("FindTopK returned error: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("FindTopK(k=0) returned %d results, want 10 (default)", len(results))
	}
}

func TestFindTopK_WhenNegativeK_ShouldDefaultToTen(t *testing.T) {
	query := []float32{1.0, 0.0}
	corpus := make([][]float32, 15)
	for i := range corpus {
		corpus[i] = []float32{1.0, float32(i)}
	}

	results, err := FindTopK(query, corpus, -5)
	if err != nil {
		t.Fatalf("FindTopK returned error: %v", err)
	}
	if len(results) != 10 {
		t.Errorf("FindTopK(k=-5) returned %d results, want 10 (default)", len(results))
	}
}

func TestFindTopK_WhenSorted_ShouldReturnDescendingSimilarity(t *testing.T) {
	query := []float32{1.0, 0.0, 0.0}
	corpus := [][]float32{
		{0.0, 1.0, 0.0}, // orthogonal (sim ~ 0)
		{1.0, 0.0, 0.0}, // identical (sim = 1)
		{0.5, 0.5, 0.0}, // somewhere in between
	}

	results, err := FindTopK(query, corpus, 3)
	if err != nil {
		t.Fatalf("FindTopK returned error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("FindTopK returned %d results, want 3", len(results))
	}

	// Verify descending order
	for i := 1; i < len(results); i++ {
		if results[i].Similarity > results[i-1].Similarity {
			t.Errorf("Results not sorted descending: [%d].Similarity=%f > [%d].Similarity=%f",
				i, results[i].Similarity, i-1, results[i-1].Similarity)
		}
	}

	// Best match should be the identical vector (index 1)
	if results[0].Index != 1 {
		t.Errorf("Best match index = %d, want 1 (identical vector)", results[0].Index)
	}
	if math.Abs(results[0].Similarity-1.0) > 1e-6 {
		t.Errorf("Best match similarity = %f, want 1.0", results[0].Similarity)
	}
}

func TestFindTopK_WhenDimensionMismatch_ShouldSkipMismatchedVectors(t *testing.T) {
	query := []float32{1.0, 0.0}
	corpus := [][]float32{
		{1.0, 0.0},      // matches dimensions
		{1.0, 0.0, 0.0}, // dimension mismatch - should be skipped
		{0.5, 0.5},      // matches dimensions
	}

	results, err := FindTopK(query, corpus, 5)
	if err != nil {
		t.Fatalf("FindTopK returned error: %v", err)
	}
	// Should only have 2 results (mismatched vector skipped)
	if len(results) != 2 {
		t.Errorf("FindTopK with mismatch returned %d results, want 2", len(results))
	}
}

func TestFindTopK_WhenKIsOne_ShouldReturnSingleBest(t *testing.T) {
	query := []float32{1.0, 0.0}
	corpus := [][]float32{
		{0.0, 1.0},
		{1.0, 0.0},
		{0.5, 0.5},
	}

	results, err := FindTopK(query, corpus, 1)
	if err != nil {
		t.Fatalf("FindTopK returned error: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("FindTopK(k=1) returned %d results, want 1", len(results))
	}
	if results[0].Index != 1 {
		t.Errorf("Best match index = %d, want 1", results[0].Index)
	}
}

func TestFindTopK_WhenAllMismatched_ShouldReturnEmpty(t *testing.T) {
	query := []float32{1.0, 0.0}
	corpus := [][]float32{
		{1.0, 0.0, 0.0}, // 3D vs 2D
		{1.0},           // 1D vs 2D
	}

	results, err := FindTopK(query, corpus, 5)
	if err != nil {
		t.Fatalf("FindTopK returned error: %v", err)
	}
	if len(results) != 0 {
		t.Errorf("FindTopK(all mismatched) returned %d results, want 0", len(results))
	}
}

// =============================================================================
// SimilarityResult Tests
// =============================================================================

func TestSimilarityResult_WhenCreated_ShouldHoldIndexAndSimilarity(t *testing.T) {
	r := SimilarityResult{
		Index:      42,
		Similarity: 0.95,
	}
	if r.Index != 42 {
		t.Errorf("SimilarityResult.Index = %d, want 42", r.Index)
	}
	if r.Similarity != 0.95 {
		t.Errorf("SimilarityResult.Similarity = %f, want 0.95", r.Similarity)
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
