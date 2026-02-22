package store

import (
	"context"
	"fmt"
	"math/rand"
	"testing"
)

func BenchmarkVectorRecallBruteForce(b *testing.B) {
	store, err := NewLocalStore(":memory:")
	if err != nil {
		b.Fatalf("Failed to create store: %v", err)
	}
	defer store.Close()

	// 1. Populate Store with N vectors
	numVectors := 1000 // Adjust as needed to see impact
	dim := 1536        // Typical OpenAI embedding size

	// Create a mock engine that just returns random vectors
	mockEngine := &MockEmbeddingEngine{
		EmbedFunc: func(ctx context.Context, text string) ([]float32, error) {
			vec := make([]float32, dim)
			for i := 0; i < dim; i++ {
				vec[i] = rand.Float32()
			}
			return vec, nil
		},
		DimensionsFunc: func() int { return dim },
	}
	store.SetEmbeddingEngine(mockEngine)

	ctx := context.Background()

	// Batch insert for speed
	batchSize := 100
	for i := 0; i < numVectors; i += batchSize {
		contents := make([]string, batchSize)
		metas := make([]map[string]interface{}, batchSize)
		for j := 0; j < batchSize; j++ {
			contents[j] = fmt.Sprintf("content_%d", i+j)
			metas[j] = map[string]interface{}{"id": i + j}
		}
		if _, err := store.StoreVectorBatchWithEmbedding(ctx, contents, metas); err != nil {
			b.Fatalf("Failed to store batch: %v", err)
		}
	}

	b.ResetTimer()

	// 2. Run Benchmark
	query := "test query"
	for i := 0; i < b.N; i++ {
		_, err := store.VectorRecallSemantic(ctx, query, 10)
		if err != nil {
			b.Fatalf("Recall failed: %v", err)
		}
	}
}
