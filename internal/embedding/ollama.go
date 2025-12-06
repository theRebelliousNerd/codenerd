package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
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
	if endpoint == "" {
		endpoint = "http://localhost:11434"
	}
	if model == "" {
		model = "embeddinggemma"
	}

	return &OllamaEngine{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}, nil
}

// Embed generates an embedding for a single text.
func (e *OllamaEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	req := ollamaEmbedRequest{
		Model:  e.model,
		Prompt: text,
	}

	body, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", e.endpoint+"/api/embeddings", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := e.client.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("ollama request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		bodyBytes, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, string(bodyBytes))
	}

	var result ollamaEmbedResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	return result.Embedding, nil
}

// EmbedBatch generates embeddings for multiple texts.
// Ollama doesn't have native batch API, so we call Embed sequentially.
func (e *OllamaEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	embeddings := make([][]float32, len(texts))

	for i, text := range texts {
		embedding, err := e.Embed(ctx, text)
		if err != nil {
			return nil, fmt.Errorf("failed to embed text %d: %w", i, err)
		}
		embeddings[i] = embedding
	}

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
