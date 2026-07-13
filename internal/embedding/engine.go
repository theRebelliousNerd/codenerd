// Package embedding provides vector embedding generation for semantic search.
// Supports multiple backends: Ollama (local) and Google GenAI (cloud).
package embedding

import (
	"context"
	"fmt"
	"math"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// EMBEDDING ENGINE INTERFACE
// =============================================================================

// EmbeddingEngine generates vector embeddings for text.
type EmbeddingEngine interface {
	// Embed generates embeddings for a single text
	Embed(ctx context.Context, text string) ([]float32, error)

	// EmbedBatch generates embeddings for multiple texts
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)

	// Dimensions returns the dimensionality of embeddings
	Dimensions() int

	// Name returns the engine name
	Name() string
}

// TaskTypeAwareEngine extends EmbeddingEngine with task-type-specific embedding.
type TaskTypeAwareEngine interface {
	EmbeddingEngine
	// EmbedWithTask generates embeddings with a specific task type.
	EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error)
}

// TaskTypeBatchAwareEngine extends EmbeddingEngine with task-type-specific batch embedding.
type TaskTypeBatchAwareEngine interface {
	EmbeddingEngine
	// EmbedBatchWithTask generates embeddings with a specific task type.
	EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error)
}

// HealthChecker is an optional interface for embedding engines that support
// health checks. If an engine implements this interface, the system can
// verify availability before attempting batch operations.
type HealthChecker interface {
	// HealthCheck verifies the embedding service is reachable.
	// Returns nil if healthy, error otherwise.
	HealthCheck(ctx context.Context) error
}

// =============================================================================
// EMBEDDING CONFIGURATION
// =============================================================================

// Config holds embedding engine configuration.
type Config struct {
	// Provider: "ollama" or "genai"
	Provider string `json:"provider"`

	// Ollama Configuration
	OllamaEndpoint string `json:"ollama_endpoint"` // Default: "http://localhost:11434"
	OllamaModel    string `json:"ollama_model"`    // Default: "embeddinggemma:300m"

	// GenAI Configuration
	GenAIAPIKey string `json:"genai_api_key"`
	GenAIModel  string `json:"genai_model"` // Default: "gemini-embedding-001"

	// TaskType for GenAI: "SEMANTIC_SIMILARITY", "RETRIEVAL_QUERY", "RETRIEVAL_DOCUMENT"
	TaskType string `json:"task_type"`
}

// DefaultConfig returns sensible defaults.
func DefaultConfig() Config {
	return Config{
		Provider:       "ollama", // Default to local Ollama
		OllamaEndpoint: "http://localhost:11434",
		OllamaModel:    defaultOllamaEmbedModel,
		GenAIModel:     "gemini-embedding-001",
		TaskType:       "SEMANTIC_SIMILARITY",
	}
}

// =============================================================================
// FACTORY
// =============================================================================

// NewEngine creates an embedding engine based on configuration.
func NewEngine(cfg Config) (EmbeddingEngine, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "NewEngine")
	defer timer.Stop()

	logging.Embedding("Creating embedding engine with provider=%s", cfg.Provider)
	logging.EmbeddingDebug("Engine config: provider=%s, ollama_endpoint=%s, ollama_model=%s, genai_model=%s, task_type=%s",
		cfg.Provider, cfg.OllamaEndpoint, cfg.OllamaModel, cfg.GenAIModel, cfg.TaskType)

	var engine EmbeddingEngine
	var err error

	switch cfg.Provider {
	case "ollama":
		logging.Embedding("Initializing Ollama embedding engine: endpoint=%s, model=%s", cfg.OllamaEndpoint, cfg.OllamaModel)
		var oe *OllamaEngine
		oe, err = NewOllamaEngine(cfg.OllamaEndpoint, cfg.OllamaModel)
		if err == nil {
			// Best-effort ensure at construction so first embed isn't the first
			// time we discover a missing model. Short timeout so a down Ollama
			// does not block boot; Embed will retry EnsureModel later.
			ensureCtx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
			if ensureErr := oe.EnsureModel(ensureCtx); ensureErr != nil {
				logging.Get(logging.CategoryEmbedding).Warn(
					"Ollama EnsureModel at init: %v (will retry on first Embed)", ensureErr)
			} else {
				logging.Embedding("Ollama model ready at init: %s", oe.Model())
			}
			cancel()
			engine = oe
		}
	case "genai":
		logging.Embedding("Initializing GenAI embedding engine: model=%s, task_type=%s", cfg.GenAIModel, cfg.TaskType)
		engine, err = NewGenAIEngine(cfg.GenAIAPIKey, cfg.GenAIModel, cfg.TaskType)
	default:
		err = fmt.Errorf("unsupported embedding provider: %s (use 'ollama' or 'genai')", cfg.Provider)
		logging.Get(logging.CategoryEmbedding).Error("Unsupported embedding provider: %s", cfg.Provider)
		return nil, err
	}

	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("Failed to create embedding engine: %v", err)
		return nil, err
	}

	logging.Embedding("Embedding engine created successfully: name=%s, dimensions=%d", engine.Name(), engine.Dimensions())
	return engine, nil
}

// =============================================================================
// COSINE SIMILARITY UTILITY
// =============================================================================

// FindTopK returns the indices of the top K most similar vectors to the query.
// Uses cosine similarity.
func FindTopK(query []float32, corpus [][]float32, k int) ([]SimilarityResult, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "FindTopK")
	defer timer.Stop()

	if k <= 0 {
		k = 10
	}

	logging.EmbeddingDebug("FindTopK: searching for top %d results in corpus of %d vectors (query dim=%d)",
		k, len(corpus), len(query))

	results := make([]SimilarityResult, 0, len(corpus))
	skippedCount := 0

	for i, vec := range corpus {
		similarity, err := CosineSimilarity(query, vec)
		if err != nil {
			skippedCount++
			continue
		}

		results = append(results, SimilarityResult{
			Index:      i,
			Similarity: similarity,
		})
	}

	if skippedCount > 0 {
		logging.Get(logging.CategoryEmbedding).Warn("FindTopK: skipped %d vectors due to dimension mismatch", skippedCount)
	}

	// Sort by similarity descending
	// Use simple bubble sort for small K
	sortStart := time.Now()
	for i := 0; i < len(results) && i < k; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
	logging.EmbeddingDebug("FindTopK: sorting completed in %v", time.Since(sortStart))

	// Return top K
	if len(results) > k {
		results = results[:k]
	}

	logging.EmbeddingDebug("FindTopK: returning %d results (top similarity=%.4f, bottom similarity=%.4f)",
		len(results),
		func() float64 {
			if len(results) > 0 {
				return results[0].Similarity
			}
			return 0
		}(),
		func() float64 {
			if len(results) > 0 {
				return results[len(results)-1].Similarity
			}
			return 0
		}())

	return results, nil
}

// SimilarityResult represents a similarity search result.
type SimilarityResult struct {
	Index      int
	Similarity float64
}

func validateEmbeddingVector(values []float32) error {
	if len(values) == 0 {
		return fmt.Errorf("embedding vector is empty")
	}
	for i, value := range values {
		if math.IsNaN(float64(value)) || math.IsInf(float64(value), 0) {
			return fmt.Errorf("embedding vector contains non-finite value at index %d", i)
		}
	}
	return nil
}

func validateEmbeddingBatchResponse(want int, embeddings [][]float32) error {
	if len(embeddings) != want {
		return fmt.Errorf("embedding batch returned %d vectors for %d inputs", len(embeddings), want)
	}
	if want == 0 {
		return nil
	}

	dimensions := len(embeddings[0])
	for i, values := range embeddings {
		if err := validateEmbeddingVector(values); err != nil {
			return fmt.Errorf("embedding %d: %w", i, err)
		}
		if len(values) != dimensions {
			return fmt.Errorf("embedding %d has %d dimensions; want %d", i, len(values), dimensions)
		}
	}
	return nil
}
