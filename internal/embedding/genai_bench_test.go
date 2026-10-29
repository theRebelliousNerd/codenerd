package embedding

import (
	"context"
	"os"
	"strconv"
	"testing"
)

// BenchmarkEmbedBatchParallel exercises the parallel chunk path in
// embedBatchWithTask. The genai SDK v1.58 does not expose a public interface
// over Models.EmbedContent, so this benchmark requires a real GenAI API key
// in the GENAI_API_KEY environment variable and is skipped otherwise. Run with:
//
//	go test -bench=BenchmarkEmbedBatchParallel -benchtime=1x ./internal/embedding/
//
//nolint:unused // wired for manual perf runs; kept behind env gate
func BenchmarkEmbedBatchParallel(b *testing.B) {
	apiKey := os.Getenv("GENAI_API_KEY")
	if apiKey == "" {
		b.Skip("GENAI_API_KEY not set; skipping live parallel benchmark")
		return
	}

	engine, err := NewGenAIEngine(apiKey, "gemini-embedding-001", "SEMANTIC_SIMILARITY")
	if err != nil {
		b.Fatalf("NewGenAIEngine: %v", err)
	}
	defer func() { _ = engine.Close() }()

	// 250 texts -> 3 chunks of 100/100/50, well-suited to parallelism=6.
	texts := make([]string, 250)
	for i := range texts {
		texts[i] = "benchmark text " + strconv.Itoa(i)
	}

	ctx := context.Background()
	b.ResetTimer()
	for b.Loop() {
		if _, err := engine.EmbedBatch(ctx, texts); err != nil {
			b.Fatalf("EmbedBatch: %v", err)
		}
	}
}
