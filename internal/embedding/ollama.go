package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// OLLAMA EMBEDDING ENGINE
// =============================================================================

// OllamaEngine generates embeddings using local Ollama server.
// Supports embeddinggemma and other embedding models.
type OllamaEngine struct {
	endpoint string
	model    string
	client   *http.Client
}

// NewOllamaEngine creates a new Ollama embedding engine.
func NewOllamaEngine(endpoint, model string) (*OllamaEngine, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "NewOllamaEngine")
	defer timer.Stop()

	if endpoint == "" {
		endpoint = "http://localhost:11434"
		logging.EmbeddingDebug("Ollama endpoint defaulted to: %s", endpoint)
	}
	if model == "" {
		model = "embeddinggemma"
		logging.EmbeddingDebug("Ollama model defaulted to: %s", model)
	}

	logging.Embedding("Creating Ollama engine: endpoint=%s, model=%s, timeout=30s", endpoint, model)

	engine := &OllamaEngine{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}

	logging.Embedding("Ollama engine created successfully")
	return engine, nil
}

// Embed generates an embedding for a single text.
func (e *OllamaEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "Ollama.Embed")

	textLen := len(text)
	logging.EmbeddingDebug("Ollama.Embed: starting embed request, text_length=%d chars", textLen)

	req := ollamaEmbedRequest{
		Model:  e.model,
		Prompt: text,
	}

	body, err := json.Marshal(req)
	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: failed to marshal request: %v", err)
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	logging.EmbeddingDebug("Ollama.Embed: sending POST to %s/api/embeddings", e.endpoint)
	apiStart := time.Now()

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.endpoint+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: failed to create HTTP request: %v", err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	apiLatency := time.Since(apiStart)

	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: request failed after %v: %v", apiLatency, err)
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	logging.EmbeddingDebug("Ollama.Embed: API response received in %v, status=%d", apiLatency, resp.StatusCode)

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: non-OK status %d: %s", resp.StatusCode, string(bodyBytes))
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: failed to decode response: %v", err)
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	timer.Stop()
	logging.Embedding("Ollama.Embed: completed successfully, dimensions=%d, api_latency=%v", len(result.Embedding), apiLatency)

	return result.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
// Ollama doesn't have native batch API, so we call Embed sequentially.
func (e *OllamaEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "Ollama.EmbedBatch")
	defer timer.Stop()

	logging.Embedding("Ollama.EmbedBatch: starting batch embed for %d texts", len(texts))

	if len(texts) == 0 {
		logging.EmbeddingDebug("Ollama.EmbedBatch: empty input, returning nil")
		return nil, nil
	}

	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		logging.EmbeddingDebug("Ollama.EmbedBatch: processing text %d/%d (length=%d chars)", i+1, len(texts), len(text))

		embedding, err := e.Embed(ctx, text)
		if err != nil {
			logging.Get(logging.CategoryEmbedding).Error("Ollama.EmbedBatch: failed at text %d: %v", i, err)
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

	logging.Embedding("Ollama.EmbedBatch: completed successfully, processed %d texts", len(texts))
	return embeddings, nil
}

// Dimensions returns the dimensionality of embeddings.
// embeddinggemma produces 768-dimensional vectors.
func (e *OllamaEngine) Dimensions() int {
	// embeddinggemma: 768 dimensions
	// Other models may vary
	return 768
}

// Name returns the engine name.
func (e *OllamaEngine) Name() string {
	return fmt.Sprintf("ollama:%s", e.model)
}

// =============================================================================
// OLLAMA API TYPES
// =============================================================================

type ollamaEmbedRequest struct {
	Model  string `json:"model"`
	Prompt string `json:"prompt"`
}

type ollamaEmbedResponse struct {
	Embedding []float32 `json:"embedding"`
}
