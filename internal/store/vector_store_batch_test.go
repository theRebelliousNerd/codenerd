package store

import (
	"context"
	"errors"
	"testing"
)

// 1. mockSimpleEngine only implements embedding.EmbeddingEngine
type mockSimpleEngine struct {
	embedFunc      func(ctx context.Context, text string) ([]float32, error)
	embedBatchFunc func(ctx context.Context, texts []string) ([][]float32, error)
}

func (e *mockSimpleEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	if e.embedFunc != nil {
		return e.embedFunc(ctx, text)
	}
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

func (e *mockSimpleEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if e.embedBatchFunc != nil {
		return e.embedBatchFunc(ctx, texts)
	}
	res := make([][]float32, len(texts))
	for i := range texts {
		res[i] = []float32{0.1, 0.2, 0.3, 0.4}
	}
	return res, nil
}

func (e *mockSimpleEngine) Dimensions() int { return 4 }
func (e *mockSimpleEngine) Name() string    { return "simple" }

// 2. mockTaskAwareEngine implements embedding.EmbeddingEngine and embedding.TaskTypeAwareEngine
type mockTaskAwareEngine struct {
	mockSimpleEngine
	embedWithTaskFunc func(ctx context.Context, text string, taskType string) ([]float32, error)
}

func (e *mockTaskAwareEngine) EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error) {
	if e.embedWithTaskFunc != nil {
		return e.embedWithTaskFunc(ctx, text, taskType)
	}
	return e.Embed(ctx, text)
}

// 3. mockBatchAwareEngine implements embedding.EmbeddingEngine and embedding.TaskTypeBatchAwareEngine
type mockBatchAwareEngine struct {
	mockSimpleEngine
	embedBatchWithTaskFunc func(ctx context.Context, texts []string, taskType string) ([][]float32, error)
}

func (e *mockBatchAwareEngine) EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	if e.embedBatchWithTaskFunc != nil {
		return e.embedBatchWithTaskFunc(ctx, texts, taskType)
	}
	return e.EmbedBatch(ctx, texts)
}

func TestStoreVectorBatchWithEmbedding_Comprehensive(t *testing.T) {
	ctx := context.Background()

	t.Run("Empty", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, nil, nil)
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if stored != 0 {
			t.Errorf("Expected 0 stored, got %d", stored)
		}
	})

	t.Run("Mismatch", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a"}, nil)
		if err == nil {
			t.Error("Expected error on mismatch, got nil")
		}
		if stored != 0 {
			t.Errorf("Expected 0 stored, got %d", stored)
		}
	})

	t.Run("NoEngineFallbackToKeyword", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"k": "v1"}, {"k": "v2"}})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if stored != 2 {
			t.Errorf("Expected 2 stored, got %d", stored)
		}

		count, err := s.CountVectorsByMetadata("k", "v1")
		if err != nil {
			t.Errorf("Count failed: %v", err)
		}
		if count != 1 {
			t.Errorf("Expected 1 vector, got %d", count)
		}
	})

	t.Run("SimpleEngineUniformTask", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engineCalled := false
		engine := &mockSimpleEngine{
			embedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				engineCalled = true
				return [][]float32{{0.1, 0.2, 0.3, 0.4}, {0.5, 0.6, 0.7, 0.8}}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "code"}})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if stored != 2 {
			t.Errorf("Expected 2 stored, got %d", stored)
		}
		if !engineCalled {
			t.Error("Expected simple engine EmbedBatch to be called")
		}
	})

	t.Run("TaskAwareEngineUniformTask", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		embedWithTaskCount := 0
		engine := &mockTaskAwareEngine{
			embedWithTaskFunc: func(ctx context.Context, text string, taskType string) ([]float32, error) {
				embedWithTaskCount++
				if taskType != "RETRIEVAL_DOCUMENT" {
					t.Errorf("Expected RETRIEVAL_DOCUMENT, got %s", taskType)
				}
				return []float32{0.1, 0.2, 0.3, 0.4}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "code"}})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if stored != 2 {
			t.Errorf("Expected 2 stored, got %d", stored)
		}
		if embedWithTaskCount != 2 {
			t.Errorf("Expected EmbedWithTask to be called 2 times, got %d", embedWithTaskCount)
		}
	})

	t.Run("BatchAwareEngineUniformTask", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		batchCalled := false
		engine := &mockBatchAwareEngine{
			embedBatchWithTaskFunc: func(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
				batchCalled = true
				if taskType != "RETRIEVAL_DOCUMENT" {
					t.Errorf("Expected RETRIEVAL_DOCUMENT, got %s", taskType)
				}
				return [][]float32{{0.1, 0.2, 0.3, 0.4}, {0.5, 0.6, 0.7, 0.8}}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "code"}})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if stored != 2 {
			t.Errorf("Expected 2 stored, got %d", stored)
		}
		if !batchCalled {
			t.Error("Expected EmbedBatchWithTask to be called")
		}
	})

	t.Run("TaskAwareEngineNonUniformTask", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		taskTypesCalled := make(map[string]int)
		engine := &mockTaskAwareEngine{
			embedWithTaskFunc: func(ctx context.Context, text string, taskType string) ([]float32, error) {
				taskTypesCalled[taskType]++
				return []float32{0.1, 0.2, 0.3, 0.4}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "fact"}})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if stored != 2 {
			t.Errorf("Expected 2 stored, got %d", stored)
		}
		if taskTypesCalled["RETRIEVAL_DOCUMENT"] != 1 {
			t.Errorf("Expected 1 RETRIEVAL_DOCUMENT call, got %d", taskTypesCalled["RETRIEVAL_DOCUMENT"])
		}
		if taskTypesCalled["FACT_VERIFICATION"] != 1 {
			t.Errorf("Expected 1 FACT_VERIFICATION call, got %d", taskTypesCalled["FACT_VERIFICATION"])
		}
	})

	t.Run("SimpleEngineNonUniformTask", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engineCalled := false
		engine := &mockSimpleEngine{
			embedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				engineCalled = true
				return [][]float32{{0.1, 0.2, 0.3, 0.4}, {0.5, 0.6, 0.7, 0.8}}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "fact"}})
		if err != nil {
			t.Errorf("Unexpected error: %v", err)
		}
		if stored != 2 {
			t.Errorf("Expected 2 stored, got %d", stored)
		}
		if !engineCalled {
			t.Error("Expected simple engine EmbedBatch to be called")
		}
	})

	t.Run("EmbeddingErrors", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engineErr := errors.New("embed failed")
		engine := &mockSimpleEngine{
			embedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				return nil, engineErr
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "code"}})
		if !errors.Is(err, engineErr) {
			t.Errorf("Expected error %v, got %v", engineErr, err)
		}
		if stored != 0 {
			t.Errorf("Expected 0 stored, got %d", stored)
		}
	})

	t.Run("EmbeddingSizeMismatch", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engine := &mockSimpleEngine{
			embedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				// Return 1 embedding for 2 inputs
				return [][]float32{{0.1, 0.2, 0.3, 0.4}}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "code"}})
		if err == nil {
			t.Error("Expected error on size mismatch, got nil")
		}
		if stored != 0 {
			t.Errorf("Expected 0 stored, got %d", stored)
		}
	})

	t.Run("PartialFailures", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engine := &mockSimpleEngine{
			embedBatchFunc: func(ctx context.Context, texts []string) ([][]float32, error) {
				// Return empty embedding for second item
				return [][]float32{{0.1, 0.2, 0.3, 0.4}, nil}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "code"}})
		if err == nil {
			t.Error("Expected error on partial failure, got nil")
		}
		if stored != 1 {
			t.Errorf("Expected 1 stored (first one), got %d", stored)
		}
	})

	t.Run("EmbedWithTaskPartialFailure", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engine := &mockTaskAwareEngine{
			embedWithTaskFunc: func(ctx context.Context, text string, taskType string) ([]float32, error) {
				if text == "b" {
					return nil, errors.New("partial task embed failure")
				}
				return []float32{0.1, 0.2, 0.3, 0.4}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "code"}})
		if err == nil {
			t.Error("Expected error on partial failure, got nil")
		}
		if stored != 1 {
			t.Errorf("Expected 1 stored, got %d", stored)
		}
	})

	t.Run("EmbedWithTaskNonUniformPartialFailure", func(t *testing.T) {
		s, err := NewLocalStore(":memory:")
		if err != nil {
			t.Fatalf("Failed to create store: %v", err)
		}
		defer s.Close()

		engine := &mockTaskAwareEngine{
			embedWithTaskFunc: func(ctx context.Context, text string, taskType string) ([]float32, error) {
				if text == "b" {
					return nil, errors.New("partial task non-uniform embed failure")
				}
				return []float32{0.1, 0.2, 0.3, 0.4}, nil
			},
		}
		s.SetEmbeddingEngine(engine)

		stored, err := s.StoreVectorBatchWithEmbedding(ctx, []string{"a", "b"}, []map[string]any{{"content_type": "code"}, {"content_type": "fact"}})
		if err == nil {
			t.Error("Expected error on partial failure, got nil")
		}
		if stored != 1 {
			t.Errorf("Expected 1 stored, got %d", stored)
		}
	})
}
