package store

import (
	"codenerd/internal/config"
	"context"
	"testing"
	"time"
)

func TestReflectionWorker_LocalStore_Lifecycle(t *testing.T) {
	dbPath := t.TempDir() + "/reflection.db"
	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	store.SetEmbeddingEngine(mockEngine)

	cfg := config.ReflectionConfig{
		Enabled:             true,
		TopK:                5,
		MinScore:            0.7,
		RecencyHalfLifeDays: 14,
		BacklogWatermark:    300,
	}
	store.SetReflectionConfig(cfg)

	store.startReflectionWorker()
	time.Sleep(10 * time.Millisecond)
	store.stopReflectionWorker()
}

func TestReflectionWorker_LearningStore_Lifecycle(t *testing.T) {
	dbPath := t.TempDir() + "/learning_reflection.db"
	ls, err := NewLearningStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create learning store: %v", err)
	}
	defer ls.Close()

	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	ls.SetEmbeddingEngine(mockEngine)

	cfg := config.ReflectionConfig{
		Enabled:             true,
		TopK:                5,
		MinScore:            0.7,
		RecencyHalfLifeDays: 14,
		BacklogWatermark:    300,
	}
	ls.SetReflectionConfig(cfg)

	ls.startLearningReflectionWorker()
	time.Sleep(10 * time.Millisecond)
	ls.stopLearningReflectionWorker()
}

func TestReflectionWorker_LocalStore_ManualProcess(t *testing.T) {
	dbPath := t.TempDir() + "/reflection2.db"
	store, err := NewLocalStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	store.SetEmbeddingEngine(mockEngine)

	store.GetTraceStore().StoreReasoningTrace(&ReasoningTrace{
		ID:           "trace1",
		ShardID:      "shard1",
		ShardType:    "coder",
		TaskContext:  "Context",
		Success:      true,
		QualityScore: 0.9,
		CreatedAt:    time.Now(),
	})

	store.processReflectionCycle()

	err = store.syncTraceVectorIndex([]TraceEmbeddingUpdate{}, 4)
	if err != nil {
		t.Errorf("syncTraceVectorIndex failed: %v", err)
	}
}

func TestReflectionWorker_LearningStore_ManualProcess(t *testing.T) {
	dbPath := t.TempDir() + "/learning_reflection2.db"
	ls, err := NewLearningStore(dbPath)
	if err != nil {
		t.Fatalf("Failed to create learning store: %v", err)
	}
	defer ls.Close()

	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		DimensionsFunc: func() int { return 4 },
	}
	ls.SetEmbeddingEngine(mockEngine)

	ls.Save("coder", "test_fact", []any{"arg1"}, "campaign1")

	ls.processLearningReflectionCycle()

	db, err := ls.getDB("coder")
	if err != nil {
		t.Fatalf("Failed to get DB: %v", err)
	}

	err = syncLearningVectorIndex(db, []LearningEmbeddingUpdate{}, 4)
	if err != nil {
		t.Errorf("syncLearningVectorIndex failed: %v", err)
	}
}

func TestApplyRecencyWeight(t *testing.T) {
	now := time.Now()

	// age is 0 days -> same score (within tolerance)
	val1 := applyRecencyWeight(0.8, now, 14)
	if val1 < 0.79 || val1 > 0.8 {
		t.Errorf("Expected close to 0.8, got %f", val1)
	}

	// age is 14 days -> score * 0.5 (within tolerance)
	val2 := applyRecencyWeight(0.8, now.Add(-14*24*time.Hour), 14)
	if val2 < 0.39 || val2 > 0.41 {
		t.Errorf("Expected close to 0.4, got %f", val2)
	}

	// zero time -> same score
	if applyRecencyWeight(0.8, time.Time{}, 14) != 0.8 {
		t.Errorf("Expected 0.8")
	}
}
