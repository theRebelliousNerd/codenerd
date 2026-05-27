package store

import (
	"context"
	"path/filepath"
	"sync"
	"testing"
)

// mockDimensionEngine implements a mock embedding engine that allows changing dimensions.
type mockDimensionEngine struct {
	dim int
	mu  sync.Mutex
}

func (m *mockDimensionEngine) Name() string { return "mock-dim-engine" }
func (m *mockDimensionEngine) Dimensions() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.dim
}

func (m *mockDimensionEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	m.mu.Lock()
	dim := m.dim
	m.mu.Unlock()
	vec := make([]float32, dim)
	for i := range vec {
		vec[i] = 0.1
	}
	return vec, nil
}

func (m *mockDimensionEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	result := make([][]float32, len(texts))
	for i, t := range texts {
		vec, _ := m.Embed(ctx, t)
		result[i] = vec
	}
	return result, nil
}

// TestVectorDimensionBoundaryTransition tests that the sqlite-vec tables are dynamically
// dropped and recreated when the embedding engine's dimensions change across boundaries
// (e.g. from Gemini's 3072 to Ollama's 768).
func TestVectorDimensionBoundaryTransition(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_boundary.db")

	// Phase 1: Initialize with 3072 dimensions (e.g. Gemini)
	engine3072 := &mockDimensionEngine{dim: 3072}
	
	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create LocalStore: %v", err)
	}
	defer store.Close()

	// Setting the engine triggers initVecIndex(3072)
	store.SetEmbeddingEngine(engine3072)

	// Verify we can insert a 3072-dimensional vector into the virtual table
	vec3072 := make([]float32, 3072)
	for i := range vec3072 {
		vec3072[i] = 0.5
	}
	
	// Encode and manually insert into vec_index to prove the schema matches
	encoded3072 := encodeFloat32Slice(vec3072)
	_, err = store.db.Exec("INSERT INTO vec_index (embedding, content, metadata) VALUES (?, ?, ?)", encoded3072, "test3072", "{}")
	if err != nil {
		t.Fatalf("Phase 1: Failed to insert 3072-dim vector into vec_index: %v", err)
	}

	// Phase 2: Switch to 768 dimensions (e.g. Ollama nomic-embed-text)
	engine768 := &mockDimensionEngine{dim: 768}
	
	// Setting the new engine MUST drop the old 3072 table and recreate as 768
	store.SetEmbeddingEngine(engine768)

	// The old 3072 vector should now fail to insert due to schema mismatch
	_, err = store.db.Exec("INSERT INTO vec_index (embedding, content, metadata) VALUES (?, ?, ?)", encoded3072, "test3072-fail", "{}")
	if err == nil {
		t.Fatal("Phase 2: Expected insertion of 3072-dim vector to FAIL on new 768-dim table, but it succeeded")
	}

	// Verify we can insert a 768-dimensional vector into the recreated virtual table
	vec768 := make([]float32, 768)
	for i := range vec768 {
		vec768[i] = 0.8
	}
	encoded768 := encodeFloat32Slice(vec768)
	_, err = store.db.Exec("INSERT INTO vec_index (embedding, content, metadata) VALUES (?, ?, ?)", encoded768, "test768", "{}")
	if err != nil {
		t.Fatalf("Phase 2: Failed to insert 768-dim vector into vec_index after engine swap: %v", err)
	}
}

// TestLearningDimensionBoundaryTransition tests that the learning/trace virtual tables
// also correctly dynamically adjust across engine dimension swaps.
func TestLearningDimensionBoundaryTransition(t *testing.T) {
	tmpDir := t.TempDir()
	dbPath := filepath.Join(tmpDir, "test_learning_boundary.db")

	engine3072 := &mockDimensionEngine{dim: 3072}
	
	ls, err := NewLearnedCorpusStore(dbPath, engine3072)
	if err != nil {
		t.Fatalf("Failed to create LearnedCorpusStore: %v", err)
	}
	defer ls.db.Close()

	// Setting the engine explicitly
	ls.SetEmbeddingEngine(engine3072)

	vec3072 := make([]float32, 3072)
	encoded3072 := encodeFloat32Slice(vec3072)

	// Should succeed on 3072 table
	_, err = ls.db.Exec("INSERT INTO vec_learned (embedding, pattern, verb) VALUES (?, ?, ?)", encoded3072, "pattern", "verb")
	if err != nil {
		t.Fatalf("Phase 1: Failed to insert 3072-dim vector into vec_learned: %v", err)
	}

	// Switch engine
	engine768 := &mockDimensionEngine{dim: 768}
	ls.SetEmbeddingEngine(engine768)

	// Should fail on 768 table
	_, err = ls.db.Exec("INSERT INTO vec_learned (embedding, pattern, verb) VALUES (?, ?, ?)", encoded3072, "pattern", "verb")
	if err == nil {
		t.Fatal("Phase 2: Expected 3072 insertion to fail on 768 table")
	}

	vec768 := make([]float32, 768)
	encoded768 := encodeFloat32Slice(vec768)
	
	// Should succeed on 768 table
	_, err = ls.db.Exec("INSERT INTO vec_learned (embedding, pattern, verb) VALUES (?, ?, ?)", encoded768, "pattern", "verb")
	if err != nil {
		t.Fatalf("Phase 2: Failed to insert 768-dim vector into vec_learned after engine swap: %v", err)
	}
}
