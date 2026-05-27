package store

import (
	"context"
	"math"
	"testing"
)

// mockE2EEmbeddingEngine generates mathematically deterministic embeddings
// that are perfectly orthogonal (dot product 0) for testing exact recall.
type mockE2EEmbeddingEngine struct {
	dim int
}

func (e *mockE2EEmbeddingEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	// For "Doc A", "Doc B", "Doc C", generate orthogonal vectors
	vec := make([]float32, e.dim)
	if text == "Doc A" {
		vec[0] = 1.0
	} else if text == "Doc B" {
		vec[1] = 1.0
	} else if text == "Doc C" {
		vec[2] = 1.0
	} else if text == "Query A" {
		vec[0] = 1.0 // Matches A
	} else if text == "Query B" {
		vec[1] = 1.0 // Matches B
	} else {
		// Just something random for others
		for i := range vec {
			vec[i] = 0.01
		}
	}
	// normalize
	return normalizeVector(vec), nil
}

func (e *mockE2EEmbeddingEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	var out [][]float32
	for _, t := range texts {
		v, _ := e.Embed(ctx, t)
		out = append(out, v)
	}
	return out, nil
}

func (e *mockE2EEmbeddingEngine) Dimensions() int { return e.dim }
func (e *mockE2EEmbeddingEngine) Name() string    { return "mock_e2e" }
func (e *mockE2EEmbeddingEngine) Close() error    { return nil }

// Ensure it implements TaskTypeAwareEngine if needed
func (e *mockE2EEmbeddingEngine) EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error) {
	return e.Embed(ctx, text)
}
func (e *mockE2EEmbeddingEngine) EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	return e.EmbedBatch(ctx, texts)
}

func normalizeVector(v []float32) []float32 {
	var sum float32
	for _, val := range v {
		sum += val * val
	}
	mag := float32(math.Sqrt(float64(sum)))
	if mag == 0 {
		return v
	}
	out := make([]float32, len(v))
	for i, val := range v {
		out[i] = val / mag
	}
	return out
}

// TestE2E_VectorRecallSemantic rigorously validates the interaction between 
// our Store layer and the compiled sqlite-vec bindings.
func TestE2E_VectorRecallSemantic(t *testing.T) {
	ctx := context.Background()

	// Use an in-memory DB which inherently loads vec.Auto() because of CGO bindings
	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	if !s.vectorExt {
		t.Fatalf("sqlite-vec is NOT loaded (vectorExt is false). Test is invalid without CGO bindings!")
	}

	dim := 1536
	s.SetEmbeddingEngine(&mockE2EEmbeddingEngine{dim: dim})

	// Inject 3 perfectly orthogonal documents
	docs := []string{"Doc A", "Doc B", "Doc C"}
	meta := []map[string]interface{}{
		{"id": "a"}, {"id": "b"}, {"id": "c"},
	}

	for i, doc := range docs {
		err := s.StoreVectorWithEmbedding(ctx, doc, meta[i])
		if err != nil {
			t.Fatalf("Failed to store %s: %v", doc, err)
		}
	}

	// 1. Query for A -> Should return Doc A with distance ~0.0 (similarity ~1.0)
	results, err := s.VectorRecallSemantic(ctx, "Query A", 10)
	if err != nil {
		t.Fatalf("VectorRecallSemantic failed: %v", err)
	}
	if len(results) == 0 {
		t.Fatalf("Expected results, got 0")
	}

	if results[0].Content != "Doc A" {
		t.Errorf("Expected 'Doc A' as top result, got %q", results[0].Content)
	}

	// Check distance is virtually perfect (similarity = 1.0)
	sim := results[0].Metadata["similarity"].(float64)
	if sim < 0.99 {
		t.Errorf("Expected perfect similarity \u003e 0.99, got %v", sim)
	}

	// 2. Test semantic filtered
	filteredResults, err := s.VectorRecallSemanticFiltered(ctx, "Query B", 10, "id", "b")
	if err != nil {
		t.Fatalf("Filtered search failed: %v", err)
	}
	if len(filteredResults) != 1 {
		t.Fatalf("Expected exactly 1 filtered result, got %d", len(filteredResults))
	}
	if filteredResults[0].Content != "Doc B" {
		t.Errorf("Expected 'Doc B', got %q", filteredResults[0].Content)
	}
	
	// Ensure distance is also perfect for filtered
	sim2 := filteredResults[0].Metadata["similarity"].(float64)
	if sim2 < 0.99 {
		t.Errorf("Expected perfect similarity \u003e 0.99, got %v", sim2)
	}
}
