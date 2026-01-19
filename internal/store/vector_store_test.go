package store

import (
	"context"
	"testing"
)

func TestVectorStore_KeywordOnly(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	ctx := context.Background()

	// Store with no embedding engine (should fallback to keyword)
	meta := map[string]interface{}{"type": "test"}
	err = store.StoreVectorWithEmbedding(ctx, "hello world", meta)
	if err != nil {
		t.Fatalf("StoreVectorWithEmbedding failed: %v", err)
	}

	// Recall
	results, err := store.VectorRecallSemantic(ctx, "hello", 10)
	if err != nil {
		t.Fatalf("VectorRecallSemantic failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("Expected at least one result")
	}

	if results[0].Content != "hello world" {
		t.Errorf("Expected content 'hello world', got '%s'", results[0].Content)
	}
}

func TestVectorStore_WithEmbedding(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			if text == "cat" {
				return []float32{1, 0, 0, 0}, nil
			}
			if text == "dog" {
				return []float32{0.9, 0.1, 0, 0}, nil // Close to cat
			}
			if text == "car" {
				return []float32{0, 0, 1, 0}, nil // Different
			}
			return []float32{0, 0, 0, 0}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	store.SetEmbeddingEngine(mockEngine)

	ctx := context.Background()

	// Store vectors
	store.StoreVectorWithEmbedding(ctx, "cat", nil)
	store.StoreVectorWithEmbedding(ctx, "dog", nil)
	store.StoreVectorWithEmbedding(ctx, "car", nil)

	// Search for "cat"
	results, err := store.VectorRecallSemantic(ctx, "cat", 3)
	if err != nil {
		t.Fatalf("VectorRecallSemantic failed: %v", err)
	}

	if len(results) != 3 {
		t.Fatalf("Expected 3 results, got %d", len(results))
	}

	// Expect cat (similarity 1.0) then dog (high sim) then car (low sim)
	if results[0].Content != "cat" {
		t.Errorf("Top result should be 'cat', got '%s'", results[0].Content)
	}
	if results[1].Content != "dog" {
		t.Errorf("Second result should be 'dog', got '%s'", results[1].Content)
	}
}

func TestVectorStore_Batch(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	store.SetEmbeddingEngine(&MockEmbeddingEngine{})
	ctx := context.Background()

	contents := []string{"a", "b", "c"}
	metas := []map[string]interface{}{nil, nil, nil}

	count, err := store.StoreVectorBatchWithEmbedding(ctx, contents, metas)
	if err != nil {
		t.Fatalf("StoreVectorBatchWithEmbedding failed: %v", err)
	}

	if count != 3 {
		t.Errorf("Expected 3 stored vectors, got %d", count)
	}

	stats, err := store.GetVectorStats()
	if err != nil {
		t.Fatalf("GetVectorStats failed: %v", err)
	}

	if stats["total_vectors"] != int64(3) {
		t.Errorf("Expected 3 vectors in stats, got %v", stats["total_vectors"])
	}
}

func TestVectorRecallSemanticFiltered(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()
	store.SetEmbeddingEngine(&MockEmbeddingEngine{})
	ctx := context.Background()

	// Store items with metadata
	store.StoreVectorWithEmbedding(ctx, "item1", map[string]interface{}{"type": "A"})
	store.StoreVectorWithEmbedding(ctx, "item2", map[string]interface{}{"type": "B"})
	store.StoreVectorWithEmbedding(ctx, "item3", map[string]interface{}{"type": "A"})

	// Filter by type=A
	results, err := store.VectorRecallSemanticFiltered(ctx, "query", 10, "type", "A")
	if err != nil {
		t.Fatalf("VectorRecallSemanticFiltered failed: %v", err)
	}

	if len(results) != 2 {
		t.Errorf("Expected 2 results, got %d", len(results))
	}

	for _, r := range results {
		meta := r.Metadata
		if meta["type"] != "A" {
			t.Errorf("Expected type A, got %v", meta["type"])
		}
	}
}

func TestVectorRecallSemanticByPaths(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()
	store.SetEmbeddingEngine(&MockEmbeddingEngine{})
	ctx := context.Background()

	// Store items with path metadata
	store.StoreVectorWithEmbedding(ctx, "file1 content", map[string]interface{}{"path": "/src/file1.go"})
	store.StoreVectorWithEmbedding(ctx, "file2 content", map[string]interface{}{"path": "/src/file2.go"})

	// Search allowed paths
	allowed := []string{"/src/file1.go"}
	results, err := store.VectorRecallSemanticByPaths(ctx, "content", 10, allowed)
	if err != nil {
		t.Fatalf("VectorRecallSemanticByPaths failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}

	if results[0].Content != "file1 content" {
		t.Errorf("Expected 'file1 content', got '%s'", results[0].Content)
	}
}

func TestReembedAllVectors(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	// 1. Store without engine (keyword only)
	ctx := context.Background()
	store.StoreVectorWithEmbedding(ctx, "reembed me", nil)

	// Check it has no embedding
	stats, _ := store.GetVectorStats()
	if stats["with_embeddings"] != int64(0) {
		t.Error("Expected 0 embeddings initially")
	}

	// 2. Set engine
	store.SetEmbeddingEngine(&MockEmbeddingEngine{})

	// 3. Reembed
	err = store.ReembedAllVectors(ctx)
	if err != nil {
		t.Fatalf("ReembedAllVectors failed: %v", err)
	}

	// 4. Check
	stats, _ = store.GetVectorStats()
	if stats["with_embeddings"] != int64(1) {
		t.Errorf("Expected 1 embedding after reembed, got %v", stats["with_embeddings"])
	}
}

func TestVectorRecallSemanticWithTask(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()

	// Mock engine that changes embedding based on task
	mockEngine := &MockEmbeddingEngine{
		EmbedWithTaskFunc: func(ctx context.Context, text string, taskType string) ([]float32, error) {
			if taskType == "RETRIEVAL_QUERY" {
				return []float32{1, 1, 1, 1}, nil
			}
			return []float32{0, 0, 0, 0}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	store.SetEmbeddingEngine(mockEngine)
	ctx := context.Background()

	// Store data
	store.StoreVectorWithEmbedding(ctx, "data", nil)

	// Query with task
	_, err = store.VectorRecallSemanticWithTask(ctx, "query", 1, "RETRIEVAL_QUERY")
	if err != nil {
		t.Fatalf("VectorRecallSemanticWithTask failed: %v", err)
	}
}

func TestVectorContentsByMetadata(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	store.StoreVectorWithEmbedding(ctx, "c1", map[string]interface{}{"k": "v"})
	store.StoreVectorWithEmbedding(ctx, "c2", map[string]interface{}{"k": "v"})
	store.StoreVectorWithEmbedding(ctx, "c3", map[string]interface{}{"k": "other"})

	contents, err := store.VectorContentsByMetadata("k", "v")
	if err != nil {
		t.Fatalf("VectorContentsByMetadata failed: %v", err)
	}

	if len(contents) != 2 {
		t.Errorf("Expected 2 contents, got %d", len(contents))
	}
	if _, ok := contents["c1"]; !ok {
		t.Error("Missing c1")
	}
	if _, ok := contents["c2"]; !ok {
		t.Error("Missing c2")
	}
}

func TestDeleteVectorsByMetadata(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()

	store.StoreVectorWithEmbedding(ctx, "c1", map[string]interface{}{"tag": "del"})
	store.StoreVectorWithEmbedding(ctx, "c2", map[string]interface{}{"tag": "keep"})

	deleted, err := store.DeleteVectorsByMetadata("tag", "del")
	if err != nil {
		t.Fatalf("DeleteVectorsByMetadata failed: %v", err)
	}

	if deleted != 1 {
		t.Errorf("Expected 1 deleted row, got %d", deleted)
	}

	stats, _ := store.GetVectorStats()
	if stats["total_vectors"] != int64(1) {
		t.Errorf("Expected 1 remaining vector, got %v", stats["total_vectors"])
	}
}

func TestMatchesMetadata(t *testing.T) {
	// Since matchesMetadata is unexported, we can't test it directly from a separate test package
	// effectively unless we export it or test via public API (which we did in Filtered tests).
	// However, since we are in package store (same package), we can test it.

	// But wait, my test files are `package store`, so I can access unexported members.

	tests := []struct {
		meta  map[string]interface{}
		key   string
		value interface{}
		want  bool
	}{
		{map[string]interface{}{"a": "b"}, "a", "b", true},
		{map[string]interface{}{"a": "b"}, "a", "c", false},
		{map[string]interface{}{"a": 1}, "a", 1, true},
		{map[string]interface{}{"a": 1}, "a", "1", true}, // String conversion check
		{nil, "a", "b", false},
		{map[string]interface{}{"a": "b"}, "", "b", true}, // Empty key returns true
	}

	for _, tt := range tests {
		if got := matchesMetadata(tt.meta, tt.key, tt.value); got != tt.want {
			t.Errorf("matchesMetadata(%v, %q, %v) = %v, want %v", tt.meta, tt.key, tt.value, got, tt.want)
		}
	}
}

func TestMetadataJsonHandling(t *testing.T) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create local store: %v", err)
	}
	defer store.Close()
	ctx := context.Background()
	store.SetEmbeddingEngine(&MockEmbeddingEngine{})

	complexMeta := map[string]interface{}{
		"nested": map[string]interface{}{"foo": "bar"},
		"list":   []interface{}{1, 2, 3},
		"bool":   true,
	}

	err = store.StoreVectorWithEmbedding(ctx, "complex", complexMeta)
	if err != nil {
		t.Fatalf("Failed to store complex metadata: %v", err)
	}

	results, err := store.VectorRecallSemantic(ctx, "complex", 1)
	if err != nil {
		t.Fatalf("Recall failed: %v", err)
	}

	if len(results) == 0 {
		t.Fatal("No results")
	}

	gotMeta := results[0].Metadata

	// Check nested map
	nested, ok := gotMeta["nested"].(map[string]interface{})
	if !ok {
		t.Error("Failed to retrieve nested map")
	} else if nested["foo"] != "bar" {
		t.Errorf("Nested value mismatch: %v", nested["foo"])
	}

	// Check bool
	if b, ok := gotMeta["bool"].(bool); !ok || !b {
		t.Error("Failed to retrieve bool")
	}
}
