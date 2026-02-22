package store

import (
	"context"
	"fmt"
	"testing"
	"time"
)

func BenchmarkReembedAllPromptAtomsForce(b *testing.B) {
	// Setup in-memory store
	s, err := NewLocalStore(":memory:")
	if err != nil {
		b.Fatalf("NewLocalStore failed: %v", err)
	}
	defer s.Close()

	// Insert dummy prompt atoms
	stmt, err := s.db.Prepare("INSERT INTO prompt_atoms (atom_id, content, description, category, token_count, content_hash) VALUES (?, ?, ?, ?, ?, ?)")
	if err != nil {
		b.Fatalf("Prepare failed: %v", err)
	}
	defer stmt.Close()

	numAtoms := 100
	for i := 0; i < numAtoms; i++ {
		_, err = stmt.Exec(
			fmt.Sprintf("atom-%d", i),
			fmt.Sprintf("content for atom %d", i),
			fmt.Sprintf("description for atom %d", i),
			"test-category",
			10,
			fmt.Sprintf("hash-%d", i),
		)
		if err != nil {
			b.Fatalf("Insert failed: %v", err)
		}
	}

	// Mock engine with latency
	latency := 5 * time.Millisecond // Simulate moderate network latency

	mock := &MockEmbeddingEngine{
		EmbedWithTaskFunc: func(ctx context.Context, text string, taskType string) ([]float32, error) {
			time.Sleep(latency)
			return []float32{0.1, 0.2, 0.3, 0.4}, nil
		},
		EmbedBatchWithTaskFunc: func(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
			// Simulate batch efficiency: base latency + small per-item cost
			// For 100 items:
			// Serial: 100 * 5ms = 500ms
			// Batch (approx): 5ms + (100 * 0.1ms) = 15ms
			time.Sleep(latency + (time.Duration(len(texts)) * 100 * time.Microsecond))

			res := make([][]float32, len(texts))
			for i := range texts {
				res[i] = []float32{0.1, 0.2, 0.3, 0.4}
			}
			return res, nil
		},
		DimensionsFunc: func() int { return 4 },
		NameFunc:       func() string { return "mock-benchmark" },
	}

	// Inject the mock engine
	s.embeddingEngine = mock

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := s.ReembedAllPromptAtomsForce(context.Background())
		if err != nil {
			b.Fatalf("ReembedAllPromptAtomsForce failed: %v", err)
		}
	}
}
