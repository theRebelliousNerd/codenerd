package store

import (
	"context"
	"path/filepath"
	"testing"
)

func TestReflectionReembed_Traces(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")
	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create LocalStore: %v", err)
	}
	defer store.Close()

	ctx := context.Background()
	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	store.SetEmbeddingEngine(mockEngine)

	store.GetTraceStore().StoreReasoningTrace(&ReasoningTrace{
		ID:        "trace1",
		ShardType: "coder",
	})

	traces, err := store.ReembedAllTracesForce(ctx)
	if err != nil {
		t.Fatalf("ReembedAllTracesForce failed: %v", err)
	}
	if traces == 0 {
		t.Errorf("Expected >0 traces embedded")
	}
}

func TestReflectionReembed_Learnings(t *testing.T) {
	tempDir := t.TempDir()
	shardsDir := filepath.Join(tempDir, "shards")
	ls, err := NewLearningStore(shardsDir)
	if err != nil {
		t.Fatalf("Failed to create LearningStore: %v", err)
	}
	defer ls.Close()

	ctx := context.Background()
	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	ls.SetEmbeddingEngine(mockEngine)

	ls.Save("coder", "test_pred", []any{"arg1"}, "campaign1")

	learnings, err := ls.ReembedAllLearningsForce(ctx)
	if err != nil {
		t.Fatalf("ReembedAllLearningsForce failed: %v", err)
	}
	if learnings == 0 {
		t.Errorf("Expected >0 learnings embedded")
	}
}
