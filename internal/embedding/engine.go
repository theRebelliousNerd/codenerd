// Package embedding provides vector embedding generation for semantic search.
// Supports multiple backends: Ollama (local) and Google GenAI (cloud).
package embedding

import (
	"context"
	"fmt"
	"math"
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

// =============================================================================
// EMBEDDING CONFIGURATION
// =============================================================================

// Config holds embedding engine configuration.
type Config struct {
	// Provider: "ollama" or "genai"
	Provider string `json:"provider"`

	// Ollama Configuration
	OllamaEndpoint string `json:"ollama_endpoint"` // Default: "http://localhost:11434"
	OllamaModel    string `json:"ollama_model"`    // Default: "embeddinggemma"

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
		OllamaModel:    "embeddinggemma",
		GenAIModel:     "gemini-embedding-001",
		TaskType:       "SEMANTIC_SIMILARITY",
	}
}

// =============================================================================
// FACTORY
// =============================================================================

// NewEngine creates an embedding engine based on configuration.
func NewEngine(cfg Config) (EmbeddingEngine, error) {
	switch cfg.Provider {
	case "ollama":
		return NewOllamaEngine(cfg.OllamaEndpoint, cfg.OllamaModel)
	case "genai":
		return NewGenAIEngine(cfg.GenAIAPIKey, cfg.GenAIModel, cfg.TaskType)
	default:
		return nil, fmt.Errorf("unsupported embedding provider: %s (use 'ollama' or 'genai')", cfg.Provider)
	}
}

// =============================================================================
// COSINE SIMILARITY UTILITY
// =============================================================================

// CosineSimilarity calculates the cosine similarity between two vectors.
// Returns a value between -1 and 1, where 1 means identical, 0 means orthogonal.
func CosineSimilarity(a, b []float32) (float64, error) {
	if len(a) != len(b) {
		return 0, fmt.Errorf("vectors must have the same length: %d != %d", len(a), len(b))
	}

	var dotProduct, aMagnitude, bMagnitude float64
	for i := 0; i < len(a); i++ {
		dotProduct += float64(a[i] * b[i])
		aMagnitude += float64(a[i] * a[i])
		bMagnitude += float64(b[i] * b[i])
	}

	if aMagnitude == 0 || bMagnitude == 0 {
		return 0, nil
	}

	return dotProduct / (math.Sqrt(aMagnitude) * math.Sqrt(bMagnitude)), nil
}

// FindTopK returns the indices of the top K most similar vectors to the query.
// Uses cosine similarity.
func FindTopK(query []float32, corpus [][]float32, k int) ([]SimilarityResult, error) {
	if k <= 0 {
		k = 10
	}

	results := make([]SimilarityResult, 0, len(corpus))

	for i, vec := range corpus {
		similarity, err := CosineSimilarity(query, vec)
		if err != nil {
			continue
		}

		results = append(results, SimilarityResult{
			Index:      i,
			Similarity: similarity,
		})
	}

	// Sort by similarity descending
	// Use simple bubble sort for small K
	for i := 0; i < len(results) && i < k; i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].Similarity > results[i].Similarity {
				results[i], results[j] = results[j], results[i]
			}
		}
	}

	// Return top K
	if len(results) > k {
		results = results[:k]
	}

	return results, nil
}

// SimilarityResult represents a similarity search result.
type SimilarityResult struct {
	Index      int
	Similarity float64
}
