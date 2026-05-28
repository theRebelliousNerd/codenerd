package store

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

type mockTaskAndBatchAwareEngine struct {
	mockSimpleEngine
	embedWithTaskFunc      func(ctx context.Context, text string, taskType string) ([]float32, error)
	embedBatchWithTaskFunc func(ctx context.Context, texts []string, taskType string) ([][]float32, error)
}

func (e *mockTaskAndBatchAwareEngine) EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error) {
	if e.embedWithTaskFunc != nil {
		return e.embedWithTaskFunc(ctx, text, taskType)
	}
	return e.Embed(ctx, text)
}

func (e *mockTaskAndBatchAwareEngine) EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	if e.embedBatchWithTaskFunc != nil {
		return e.embedBatchWithTaskFunc(ctx, texts, taskType)
	}
	return e.EmbedBatch(ctx, texts)
}

func TestVectorRecallSemanticByPaths_Comprehensive(t *testing.T) {
	ctx := context.Background()

	t.Run("EmptyAllowedPaths", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		s.SetEmbeddingEngine(&mockSimpleEngine{})
		err = s.StoreVectorWithEmbedding(ctx, "content", map[string]any{"path": "/src/file.go"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		// When allowedPaths is empty, it should recall the item
		results, err := s.VectorRecallSemanticByPaths(ctx, "content", 10, nil)
		if err != nil {
			t.Fatalf("VectorRecallSemanticByPaths failed: %v", err)
		}
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}
	})

	t.Run("EngineNilKeywordSearch", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		// Store keyword-only vectors
		err = s.storeVectorKeywordOnly("hello file1", map[string]any{"path": "/src/file1.go"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}
		err = s.storeVectorKeywordOnly("hello file2", map[string]any{"path": "/src/file2.go"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		// Query path /src/file1.go
		results, err := s.VectorRecallSemanticByPaths(ctx, "hello", 10, []string{"/src/file1.go"})
		if err != nil {
			t.Fatalf("VectorRecallSemanticByPaths failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Content != "hello file1" {
			t.Errorf("Expected 'hello file1', got '%s'", results[0].Content)
		}
	})

	t.Run("EngineNilKeywordSearchQueryError", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		// Close DB to force error on query
		s.Close()

		_, err = s.VectorRecallSemanticByPaths(ctx, "hello", 10, []string{"/src/file1.go"})
		if err == nil {
			t.Error("Expected error on closed database query, got nil")
		}
	})

	t.Run("EngineReturnsError", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engineErr := errors.New("embed error")
		engine := &mockSimpleEngine{
			embedFunc: func(ctx context.Context, text string) ([]float32, error) {
				return nil, engineErr
			},
		}
		s.SetEmbeddingEngine(engine)

		_, err = s.VectorRecallSemanticByPaths(ctx, "hello", 10, []string{"/src/file1.go"})
		if !errors.Is(err, engineErr) {
			t.Errorf("Expected error %v, got %v", engineErr, err)
		}
	})

	t.Run("BruteForceSearchWithPaths", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		s.SetEmbeddingEngine(&mockSimpleEngine{})

		// Force brute force by setting vectorExt = false
		s.mu.Lock()
		s.vectorExt = false
		s.mu.Unlock()

		err = s.StoreVectorWithEmbedding(ctx, "hello file1", map[string]any{"path": "/src/file1.go"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}
		err = s.StoreVectorWithEmbedding(ctx, "hello file2", map[string]any{"path": "/src/file2.go"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		results, err := s.VectorRecallSemanticByPaths(ctx, "hello", 10, []string{"/src/file1.go"})
		if err != nil {
			t.Fatalf("VectorRecallSemanticByPaths failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Content != "hello file1" {
			t.Errorf("Expected 'hello file1', got '%s'", results[0].Content)
		}
	})
}

func TestVectorRecallSemanticFiltered_Comprehensive(t *testing.T) {
	ctx := context.Background()

	t.Run("EngineNilKeywordSearch", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		// Store keyword-only vectors
		err = s.storeVectorKeywordOnly("hello campaign1", map[string]any{"campaign": "c1"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}
		err = s.storeVectorKeywordOnly("hello campaign2", map[string]any{"campaign": "c2"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		// Query key campaign value c1
		results, err := s.VectorRecallSemanticFiltered(ctx, "hello", 10, "campaign", "c1")
		if err != nil {
			t.Fatalf("VectorRecallSemanticFiltered failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Content != "hello campaign1" {
			t.Errorf("Expected 'hello campaign1', got '%s'", results[0].Content)
		}
	})

	t.Run("EngineNilKeywordSearchQueryError", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		// Close DB to force error on query
		s.Close()

		_, err = s.VectorRecallSemanticFiltered(ctx, "hello", 10, "campaign", "c1")
		if err == nil {
			t.Error("Expected error on closed database query, got nil")
		}
	})

	t.Run("EngineReturnsError", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engineErr := errors.New("embed error")
		engine := &mockSimpleEngine{
			embedFunc: func(ctx context.Context, text string) ([]float32, error) {
				return nil, engineErr
			},
		}
		s.SetEmbeddingEngine(engine)

		_, err = s.VectorRecallSemanticFiltered(ctx, "hello", 10, "campaign", "c1")
		if !errors.Is(err, engineErr) {
			t.Errorf("Expected error %v, got %v", engineErr, err)
		}
	})

	t.Run("BruteForceSearchWithFilter", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		s.SetEmbeddingEngine(&mockSimpleEngine{})

		// Force brute force by setting vectorExt = false
		s.mu.Lock()
		s.vectorExt = false
		s.mu.Unlock()

		err = s.StoreVectorWithEmbedding(ctx, "hello campaign1", map[string]any{"campaign": "c1"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}
		err = s.StoreVectorWithEmbedding(ctx, "hello campaign2", map[string]any{"campaign": "c2"})
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		results, err := s.VectorRecallSemanticFiltered(ctx, "hello", 10, "campaign", "c1")
		if err != nil {
			t.Fatalf("VectorRecallSemanticFiltered failed: %v", err)
		}
		if len(results) != 1 {
			t.Fatalf("Expected 1 result, got %d", len(results))
		}
		if results[0].Content != "hello campaign1" {
			t.Errorf("Expected 'hello campaign1', got '%s'", results[0].Content)
		}
	})
}

func TestReembedAllVectors_Comprehensive(t *testing.T) {
	ctx := context.Background()

	t.Run("NilEngineError", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		err = s.ReembedAllVectors(ctx)
		if err == nil {
			t.Error("Expected error on nil engine, got nil")
		}
	})

	t.Run("ZeroVectorsToReembed", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		s.SetEmbeddingEngine(&mockSimpleEngine{})

		err = s.ReembedAllVectors(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
	})

	t.Run("BatchingWith33Vectors", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		// Populate 33 vectors without embeddings
		for i := 1; i <= 33; i++ {
			err = s.storeVectorKeywordOnly(fmt.Sprintf("vector %d", i), map[string]any{"num": i})
			if err != nil {
				t.Fatalf("Store keyword-only failed at %d: %v", i, err)
			}
		}

		batchEmbedCount := 0
		engine := &mockSimpleEngine{
			embedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				batchEmbedCount++
				res := make([][]float32, len(texts))
				for j := range texts {
					res[j] = []float32{0.1, 0.2, 0.3, 0.4}
				}
				return res, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		err = s.ReembedAllVectors(ctx)
		if err != nil {
			t.Fatalf("ReembedAllVectors failed: %v", err)
		}

		// Check that batching occurred. 33 vectors in batches of 32 means 2 batches.
		if batchEmbedCount != 2 {
			t.Errorf("Expected 2 batches, got %d", batchEmbedCount)
		}

		stats, err := s.GetVectorStats()
		if err != nil {
			t.Fatalf("GetVectorStats failed: %v", err)
		}
		if stats["total_vectors"] != int64(33) {
			t.Errorf("Expected 33 vectors in stats, got %v", stats["total_vectors"])
		}
		if stats["with_embeddings"] != int64(33) {
			t.Errorf("Expected 33 with embeddings, got %v", stats["with_embeddings"])
		}
	})

	t.Run("EmbedBatchError", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		err = s.storeVectorKeywordOnly("content", nil)
		if err != nil {
			t.Fatalf("Store keyword-only failed: %v", err)
		}

		engineErr := errors.New("batch embed failed")
		engine := &mockSimpleEngine{
			embedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				return nil, engineErr
			},
		}
		s.SetEmbeddingEngine(engine)

		err = s.ReembedAllVectors(ctx)
		if !errors.Is(err, engineErr) {
			t.Errorf("Expected error %v, got %v", engineErr, err)
		}
	})
}

func TestReembedAllVectorsForce_Comprehensive(t *testing.T) {
	ctx := context.Background()

	t.Run("NilEngineError", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		_, err = s.ReembedAllVectorsForce(ctx)
		if err == nil {
			t.Error("Expected error on nil engine, got nil")
		}
	})

	t.Run("ZeroVectorsToReembed", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		s.SetEmbeddingEngine(&mockSimpleEngine{})

		count, err := s.ReembedAllVectorsForce(ctx)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if count != 0 {
			t.Errorf("Expected 0 vectors force reembedded, got %d", count)
		}
	})

	t.Run("BatchingWith33VectorsAndFallback", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		s.SetEmbeddingEngine(&mockSimpleEngine{})

		// Populate 33 vectors with embeddings
		for i := 1; i <= 33; i++ {
			err = s.StoreVectorWithEmbedding(ctx, fmt.Sprintf("vector %d", i), map[string]any{"num": i, "content_type": "code"})
			if err != nil {
				t.Fatalf("Store keyword-only failed at %d: %v", i, err)
			}
		}

		// Configure engine that fails batch embedding but succeeds single embedding
		batchEmbedCount := 0
		singleEmbedCount := 0
		engine := &mockTaskAndBatchAwareEngine{
			mockSimpleEngine: mockSimpleEngine{
				embedFunc: func(ctx context.Context, text string) ([]float32, error) {
					singleEmbedCount++
					return []float32{0.1, 0.2, 0.3, 0.4}, nil
				},
			},
			embedBatchWithTaskFunc: func(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
				batchEmbedCount++
				return nil, errors.New("batch embed failed, trigger fallback")
			},
			embedWithTaskFunc: func(ctx context.Context, text string, taskType string) ([]float32, error) {
				singleEmbedCount++
				return []float32{0.1, 0.2, 0.3, 0.4}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		count, err := s.ReembedAllVectorsForce(ctx)
		if err != nil {
			t.Fatalf("ReembedAllVectorsForce failed: %v", err)
		}

		if count != 33 {
			t.Errorf("Expected 33 force reembedded, got %d", count)
		}

		// 33 vectors in batches of 32 means 2 batches attempted and failed
		if batchEmbedCount != 2 {
			t.Errorf("Expected 2 batch attempts, got %d", batchEmbedCount)
		}

		// Triggered fallback for all 33 items
		if singleEmbedCount != 33 {
			t.Errorf("Expected 33 single fallback attempts, got %d", singleEmbedCount)
		}
	})

	t.Run("FallbackFailures", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		s.SetEmbeddingEngine(&mockSimpleEngine{})

		err = s.StoreVectorWithEmbedding(ctx, "content", nil)
		if err != nil {
			t.Fatalf("Store failed: %v", err)
		}

		fallbackErr := errors.New("fallback failed")
		engine := &mockSimpleEngine{
			embedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				return nil, errors.New("batch failed")
			},
			embedFunc: func(ctx context.Context, text string) ([]float32, error) {
				return nil, fallbackErr
			},
		}
		s.SetEmbeddingEngine(engine)

		_, err = s.ReembedAllVectorsForce(ctx)
		if !errors.Is(err, fallbackErr) {
			t.Errorf("Expected error %v, got %v", fallbackErr, err)
		}
	})
}

func TestVectorRecallForPromptAtoms_KeywordFilter(t *testing.T) {
	ctx := context.Background()

	s, err := NewLocalStore(":memory:")
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer s.Close()

	// Store prompt atom and a non-prompt atom without an engine (keyword fallback)
	err = s.storeVectorKeywordOnly("prompt atom content", map[string]any{"content_type": "prompt_atom"})
	if err != nil {
		t.Fatalf("Store prompt atom failed: %v", err)
	}
	err = s.storeVectorKeywordOnly("regular content", map[string]any{"content_type": "regular"})
	if err != nil {
		t.Fatalf("Store regular content failed: %v", err)
	}

	// Call VectorRecallForPromptAtoms with engine = nil
	results, err := s.VectorRecallForPromptAtoms(ctx, "content", 10)
	if err != nil {
		t.Fatalf("VectorRecallForPromptAtoms failed: %v", err)
	}

	if len(results) != 1 {
		t.Fatalf("Expected 1 result, got %d", len(results))
	}
	if results[0].Content != "prompt atom content" {
		t.Errorf("Expected 'prompt atom content', got '%s'", results[0].Content)
	}
}
