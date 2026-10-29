package perception

import (
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
	"context"
)

// Test seams: conveniences only tests call. They live here, not in
// production files, so the dead-code budget measures production code.

// NewZAIClient creates a new ZAI client with default config.
func NewZAIClient(apiKey string) *ZAIClient {
	config := DefaultZAIConfig(apiKey)
	return NewZAIClientWithConfig(config)
}

// cacheGet retrieves a cached embedding by text hash and model name.
// Returns nil if not found or on error.
func (s *EmbeddedCorpusStore) cacheGet(textHash, modelName string) []float32 {
	return s.cacheGetWithContext(context.Background(), textHash, modelName)
}

// cachePut stores an embedding in the cache.
func (s *EmbeddedCorpusStore) cachePut(textHash, modelName string, vec []float32) {
	_ = s.cachePutWithContext(context.Background(), textHash, modelName, vec)
}

// LoadFromKernel hydrates the embedded corpus from intent_definition facts in the kernel.
// This preserves the split-brain architecture: Mangle stores canonical patterns as data,
// while semantic matching uses embeddings over those patterns.
//
// When a SQLite cache is configured, embeddings are looked up by content hash + model
// name before falling back to the embedding engine. Only cache misses require API calls.
func (s *EmbeddedCorpusStore) LoadFromKernel(ctx context.Context, kernel core.Kernel, engine embedding.EmbeddingEngine) error {
	return s.loadFromKernel(ctx, kernel, engine, false)
}

// NewEmbeddedCorpusStoreWithCache creates an EmbeddedCorpusStore backed by a SQLite cache.
// The cache persists embeddings between boots so that static intent_definition texts
// are not re-embedded on every startup. If cachePath is empty, behaves identically
// to NewEmbeddedCorpusStore (no cache).
func NewEmbeddedCorpusStoreWithCache(dimensions int, cachePath string) (*EmbeddedCorpusStore, error) {
	return NewEmbeddedCorpusStoreWithCacheWithContext(context.Background(), dimensions, cachePath)
}

// NewLearnedCorpusStore initializes the learned patterns store.
// In production this is backed by `.nerd/learned_patterns.db`; tests/dev fall back to memory.
func NewLearnedCorpusStore(cfg *config.UserConfig, dimensions int, embedEngine embedding.EmbeddingEngine) (*LearnedCorpusStore, error) {
	return NewLearnedCorpusStoreWithContext(context.Background(), cfg, dimensions, embedEngine)
}

// InitPerceptionLayer initializes all perception components.
// This should be called during session startup after the kernel and config are available.
//
// Components initialized:
//   - SemanticClassifier (vector-based intent classification)
//
// The function performs graceful degradation: if semantic classification cannot be
// initialized (e.g., no embedding config), the system continues with regex-only
// classification. This ensures the perception layer is always functional.
func InitPerceptionLayer(kernel core.Kernel, cfg *config.UserConfig) error {
	return InitPerceptionLayerWithContext(context.Background(), kernel, cfg)
}
