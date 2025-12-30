package embedding

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
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

	logging.Embedding("Creating Ollama engine: endpoint=%s, model=%s, timeout=60s", endpoint, model)

	engine := &OllamaEngine{
		endpoint: endpoint,
		model:    model,
		client: &http.Client{
			// 60 seconds allows for:
			// - Ollama cold starts (model loading)
			// - High load scenarios (multiple concurrent requests)
			// - Larger text embeddings
			// Individual operations like JIT compilation use their own sub-deadlines.
			Timeout: 60 * time.Second,
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

	// Retry transient Ollama runner/network failures.
	const maxRetries = 3
	backoff := 300 * time.Millisecond

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		req := ollamaEmbedRequest{
			Model:  e.model,
			Prompt: text,
		}

		body, err := json.Marshal(req)
		if err != nil {
			logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: failed to marshal request: %v", err)
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		logging.EmbeddingDebug("Ollama.Embed: sending POST to %s/api/embeddings (attempt %d/%d)", e.endpoint, attempt, maxRetries)
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
			if attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: request failed after %v (attempt %d/%d): %v; retrying in %v",
					apiLatency, attempt, maxRetries, err, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: request failed after %v: %v", apiLatency, err)
			return nil, fmt.Errorf("ollama request failed: %w", err)
		}

		respBytes, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			if attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: failed to read response (attempt %d/%d): %v; retrying in %v",
					attempt, maxRetries, readErr, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, fmt.Errorf("failed to read response: %w", readErr)
		}

		if resp.StatusCode != http.StatusOK {
			bodyStr := string(respBytes)
			retryable := resp.StatusCode >= 500 && resp.StatusCode <= 599
			if !retryable && strings.Contains(bodyStr, "connection was forcibly closed") {
				retryable = true
			}

			if retryable && attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: non-OK status %d (attempt %d/%d): %s; retrying in %v",
					resp.StatusCode, attempt, maxRetries, bodyStr, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}

			logging.Get(logging.CategoryEmbedding).Error("Ollama.Embed: non-OK status %d: %s", resp.StatusCode, bodyStr)
			return nil, fmt.Errorf("ollama returned status %d: %s", resp.StatusCode, bodyStr)
		}

		var result ollamaEmbedResponse
		if err := json.Unmarshal(respBytes, &result); err != nil {
			if attempt < maxRetries && ctx.Err() == nil {
				logging.Get(logging.CategoryEmbedding).Warn("Ollama.Embed: decode failed (attempt %d/%d): %v; retrying in %v",
					attempt, maxRetries, err, backoff)
				time.Sleep(backoff)
				backoff *= 2
				continue
			}
			return nil, fmt.Errorf("failed to decode response: %w", err)
		}

		timer.Stop()
		logging.Embedding("Ollama.Embed: completed successfully, dimensions=%d, api_latency=%v", len(result.Embedding), apiLatency)

		return result.Embedding, nil
	}

	return nil, fmt.Errorf("ollama embed failed after %d attempts", maxRetries)
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

// HealthCheck verifies that the Ollama service is reachable.
// This should be called before batch embedding operations to fail fast
// instead of blocking for minutes with retries.
func (e *OllamaEngine) HealthCheck(ctx context.Context) error {
	timer := logging.StartTimer(logging.CategoryEmbedding, "Ollama.HealthCheck")
	defer timer.Stop()

	// Create a short timeout context for the health check
	checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	logging.EmbeddingDebug("Ollama.HealthCheck: checking endpoint %s/api/tags", e.endpoint)

	req, err := http.NewRequestWithContext(checkCtx, "GET", e.endpoint+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("failed to create health check request: %w", err)
	}

	resp, err := e.client.Do(req)
	if err != nil {
		logging.Get(logging.CategoryEmbedding).Warn("Ollama.HealthCheck: endpoint unreachable: %v", err)
		return fmt.Errorf("ollama unavailable at %s: %w", e.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		logging.Get(logging.CategoryEmbedding).Warn("Ollama.HealthCheck: endpoint returned status %d", resp.StatusCode)
		return fmt.Errorf("ollama returned status %d", resp.StatusCode)
	}

	logging.Embedding("Ollama.HealthCheck: endpoint healthy")
	return nil
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
