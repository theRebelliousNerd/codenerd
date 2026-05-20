package store

import (
	"context"
	"testing"
)

func TestVectorStore_ExtraMethods(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// 1. Test CountVectorsByMetadata
	meta1 := map[string]interface{}{"doc_id": "d1"}
	meta2 := map[string]interface{}{"doc_id": "d2"}
	store.storeVectorKeywordOnly("content 1", meta1)
	store.storeVectorKeywordOnly("content 2", meta2)

	count, err := store.CountVectorsByMetadata("doc_id", "d1")
	if err != nil {
		t.Errorf("CountVectorsByMetadata failed: %v", err)
	}
	if count != 1 {
		t.Errorf("Expected 1 vector, got %d", count)
	}

	// 2. Test VectorRecallForPromptAtoms
	atoms, err := store.VectorRecallForPromptAtoms(ctx, "query", 5)
	if err != nil {
		t.Errorf("VectorRecallForPromptAtoms failed: %v", err)
	}
	if len(atoms) != 0 {
		t.Errorf("Expected 0 atoms initially, got %d", len(atoms))
	}

	// 3. Test ReembedAllVectorsForce
	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	store.SetEmbeddingEngine(mockEngine)

	reembeddedCount, err := store.ReembedAllVectorsForce(ctx)
	if err != nil {
		t.Errorf("ReembedAllVectorsForce failed: %v", err)
	}
	if reembeddedCount != 2 {
		t.Errorf("Expected 2 reembedded, got %d", reembeddedCount)
	}

	// Test VectorRecallForPromptAtoms with embedding engine
	metaAtom := map[string]interface{}{"content_type": "prompt_atom"}
	err = store.StoreVectorWithEmbedding(ctx, "atom content", metaAtom)
	if err != nil {
		t.Fatalf("StoreVectorWithEmbedding failed: %v", err)
	}

	atomsWithEngine, err := store.VectorRecallForPromptAtoms(ctx, "query", 5)
	if err != nil {
		t.Errorf("VectorRecallForPromptAtoms with engine failed: %v", err)
	}
	if len(atomsWithEngine) == 0 {
		t.Errorf("Expected >= 1 atoms with engine, got 0")
	}

	// Test with vectorExt = false temporarily to hit the brute-force branch
	store.mu.Lock()
	origVecExt := store.vectorExt
	store.vectorExt = false
	store.mu.Unlock()

	atomsBruteForce, err := store.VectorRecallForPromptAtoms(ctx, "query", 5)
	if err != nil {
		t.Errorf("VectorRecallForPromptAtoms with engine and brute force failed: %v", err)
	}
	if len(atomsBruteForce) == 0 {
		t.Errorf("Expected >= 1 atoms with brute force, got 0")
	}

	store.mu.Lock()
	store.vectorExt = origVecExt
	store.mu.Unlock()

	// 4. Test storeVectorBatchKeywordOnly
	batchMeta := []map[string]interface{}{
		{"doc_id": "d3"},
		{"doc_id": "d4"},
	}
	added, err := store.storeVectorBatchKeywordOnly([]string{"batch1", "batch2"}, batchMeta)
	if err != nil {
		t.Errorf("storeVectorBatchKeywordOnly failed: %v", err)
	}
	if added != 2 {
		t.Errorf("Expected 2 added, got %d", added)
	}

	count, _ = store.CountVectorsByMetadata("doc_id", "d3")
	if count != 1 {
		t.Errorf("Expected 1 vector for d3, got %d", count)
	}
}
