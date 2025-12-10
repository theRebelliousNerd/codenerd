package embedding

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/logging"

	"google.golang.org/genai"
)

// =============================================================================
// GOOGLE GENAI EMBEDDING ENGINE
// =============================================================================

// maxBatchSize is the maximum number of texts allowed in a single GenAI batch request.
// The API returns error 400 if more than 100 requests are in one batch.
const maxBatchSize = 100

func int32Ptr(i int32) *int32 {
	return &i
}

// GenAIEngine generates embeddings using Google's Gemini API.
type GenAIEngine struct {
	client   *genai.Client
	model    string
	taskType string // Task type as string for API flexibility
}

// NewGenAIEngine creates a new GenAI embedding engine.
func NewGenAIEngine(apiKey, model, taskType string) (*GenAIEngine, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "NewGenAIEngine")
	defer timer.Stop()

	logging.Embedding("Creating GenAI embedding engine")

	if apiKey == "" {
		logging.Get(logging.CategoryEmbedding).Error("GenAI API key is required but not provided")
		return nil, fmt.Errorf("GenAI API key is required")
	}
	logging.EmbeddingDebug("GenAI API key provided (length=%d)", len(apiKey))

	if model == "" {
		model = "gemini-embedding-001"
		logging.EmbeddingDebug("GenAI model defaulted to: %s", model)
	}

	if taskType == "" {
		taskType = "SEMANTIC_SIMILARITY"
		logging.EmbeddingDebug("GenAI taskType defaulted to: %s", taskType)
	}

	logging.Embedding("Initializing GenAI client: model=%s, task_type=%s", model, taskType)

	ctx := context.Background()
	clientStart := time.Now()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	clientLatency := time.Since(clientStart)

	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("Failed to create GenAI client after %v: %v", clientLatency, err)
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	logging.Embedding("GenAI client created successfully in %v", clientLatency)

	return &GenAIEngine{
		client:   client,
		model:    model,
		taskType: taskType,
	}, nil
}

// Embed generates an embedding for a single text.
func (e *GenAIEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "GenAI.Embed")

	textLen := len(text)
	logging.EmbeddingDebug("GenAI.Embed: starting embed request, text_length=%d chars, model=%s", textLen, e.model)

	contents := []*genai.Content{
		genai.NewContentFromText(text, genai.RoleUser),
	}

	logging.EmbeddingDebug("GenAI.Embed: calling EmbedContent API")
	apiStart := time.Now()

	result, err := e.client.Models.EmbedContent(ctx,
		e.model,
		contents,
		&genai.EmbedContentConfig{
			OutputDimensionality: int32Ptr(3072),
		},
	)
	apiLatency := time.Since(apiStart)

	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("GenAI.Embed: API call failed after %v: %v", apiLatency, err)
		return nil, fmt.Errorf("GenAI embed failed: %w", err)
	}

	logging.EmbeddingDebug("GenAI.Embed: API response received in %v", apiLatency)

	if len(result.Embeddings) == 0 {
		logging.Get(logging.CategoryEmbedding).Error("GenAI.Embed: no embeddings returned from API")
		return nil, fmt.Errorf("no embeddings returned")
	}

	dimensions := len(result.Embeddings[0].Values)
	timer.Stop()
	logging.Embedding("GenAI.Embed: completed successfully, dimensions=%d, api_latency=%v", dimensions, apiLatency)

	return result.Embeddings[0].Values, nil
}

// EmbedBatch generates embeddings for multiple texts.
// GenAI has native batch support but limits batches to 100 items.
// This function automatically chunks larger batches and concatenates results.
func (e *GenAIEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "GenAI.EmbedBatch")
	defer timer.Stop()

	logging.Embedding("GenAI.EmbedBatch: starting native batch embed for %d texts", len(texts))

	if len(texts) == 0 {
		logging.EmbeddingDebug("GenAI.EmbedBatch: empty input, returning nil")
		return nil, nil
	}

	// Calculate total text size for logging
	totalChars := 0
	for _, text := range texts {
		totalChars += len(text)
	}
	logging.EmbeddingDebug("GenAI.EmbedBatch: total input size=%d chars across %d texts", totalChars, len(texts))

	// If within batch limit, process in single request
	if len(texts) <= maxBatchSize {
		return e.embedBatchChunk(ctx, texts)
	}

	// Chunk into batches of maxBatchSize and process sequentially
	numBatches := (len(texts) + maxBatchSize - 1) / maxBatchSize
	logging.Embedding("GenAI.EmbedBatch: chunking %d texts into %d batches of up to %d items", len(texts), numBatches, maxBatchSize)

	allEmbeddings := make([][]float32, 0, len(texts))

	for batchIdx := 0; batchIdx < numBatches; batchIdx++ {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		start := batchIdx * maxBatchSize
		end := start + maxBatchSize
		if end > len(texts) {
			end = len(texts)
		}

		chunk := texts[start:end]
		logging.EmbeddingDebug("GenAI.EmbedBatch: processing batch %d/%d with %d texts (indices %d-%d)",
			batchIdx+1, numBatches, len(chunk), start, end-1)

		chunkEmbeddings, err := e.embedBatchChunk(ctx, chunk)
		if err != nil {
			return nil, fmt.Errorf("batch %d/%d failed: %w", batchIdx+1, numBatches, err)
		}

		allEmbeddings = append(allEmbeddings, chunkEmbeddings...)
	}

	dimensions := 0
	if len(allEmbeddings) > 0 && len(allEmbeddings[0]) > 0 {
		dimensions = len(allEmbeddings[0])
	}

	logging.Embedding("GenAI.EmbedBatch: completed successfully, processed %d texts in %d batches, dimensions=%d",
		len(texts), numBatches, dimensions)

	return allEmbeddings, nil
}

// embedBatchChunk processes a single batch chunk (must be <= maxBatchSize).
func (e *GenAIEngine) embedBatchChunk(ctx context.Context, texts []string) ([][]float32, error) {
	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = genai.NewContentFromText(text, genai.RoleUser)
	}

	logging.EmbeddingDebug("GenAI.embedBatchChunk: calling EmbedContent API with %d contents", len(contents))
	apiStart := time.Now()

	result, err := e.client.Models.EmbedContent(ctx,
		e.model,
		contents,
		&genai.EmbedContentConfig{
			OutputDimensionality: int32Ptr(3072),
		},
	)
	apiLatency := time.Since(apiStart)

	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("GenAI.embedBatchChunk: API call failed after %v: %v", apiLatency, err)
		return nil, fmt.Errorf("GenAI batch embed failed: %w", err)
	}

	logging.EmbeddingDebug("GenAI.embedBatchChunk: API response received in %v, got %d embeddings", apiLatency, len(result.Embeddings))

	embeddings := make([][]float32, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		embeddings[i] = emb.Values
	}

	return embeddings, nil
}

// Dimensions returns the dimensionality of embeddings.
// gemini-embedding-001 / text-embedding-004 produce 3072-dimensional vectors.
// Note: Google updated these models from 768 to 3072 dimensions.
func (e *GenAIEngine) Dimensions() int {
	return 3072
}

// Name returns the engine name.
func (e *GenAIEngine) Name() string {
	return fmt.Sprintf("genai:%s", e.model)
}

// Close is a no-op for GenAI client (no cleanup needed).
func (e *GenAIEngine) Close() error {
	// GenAI client doesn't require explicit cleanup
	return nil
}
