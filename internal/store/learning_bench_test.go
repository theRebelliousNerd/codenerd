package store

import (
	"codenerd/internal/types"
	"fmt"
	"path/filepath"
	"testing"
)

func BenchmarkLearningStore_Save(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench_save.db")
	ls, err := NewLearningStore(dbPath)
	if err != nil {
		b.Fatalf("Failed to create LearningStore: %v", err)
	}
	defer ls.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ls.Save("sys", "correction_pattern", []any{fmt.Sprintf("pattern%d", i)}, "")
		if err != nil {
			b.Fatalf("Save failed: %v", err)
		}
	}
}

func BenchmarkLearningStore_SaveBatch(b *testing.B) {
	tempDir := b.TempDir()
	dbPath := filepath.Join(tempDir, "bench_save_batch.db")
	ls, err := NewLearningStore(dbPath)
	if err != nil {
		b.Fatalf("Failed to create LearningStore: %v", err)
	}
	defer ls.Close()

	// Prepare batch
	batch := make([]types.ShardLearning, 100)
	for i := 0; i < 100; i++ {
		batch[i] = types.ShardLearning{
			FactPredicate: "correction_pattern",
			FactArgs:      []any{fmt.Sprintf("pattern%d", i)},
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		err := ls.SaveBatch("sys", batch, "")
		if err != nil {
			b.Fatalf("SaveBatch failed: %v", err)
		}
	}
}
