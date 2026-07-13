package embedding

import (
	"context"
	"fmt"
	"time"

	"codenerd/internal/logging"

	"golang.org/x/sync/errgroup"
	"google.golang.org/genai"
)

// =============================================================================
// GOOGLE GENAI EMBEDDING ENGINE
// =============================================================================

// maxBatchSize is the maximum number of texts allowed in a single GenAI batch request.
// The API returns error 400 if more than 100 requests are in one batch.
const maxBatchSize = 100

// batchParallelism bounds the number of concurrent EmbedContent calls when a
// caller-provided slice exceeds maxBatchSize. Tuned for shared HTTP transport
// and Gemini's typical per-key concurrency budget; raising it without the
// pooled transport in internal/perception/transport.go will starve other
// callers and provoke 429s.
const batchParallelism = 6

//go:fix inline
func int32Ptr(i int32) *int32 {
	return new(i)
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

	taskType = normalizeTaskType(taskType)
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
	return e.embedWithTask(ctx, text, e.taskType)
}

// EmbedWithTask generates an embedding for a single text using an explicit task type.
func (e *GenAIEngine) EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error) {
	return e.embedWithTask(ctx, text, taskType)
}

func (e *GenAIEngine) embedWithTask(ctx context.Context, text string, taskType string) ([]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "GenAI.Embed")
	defer timer.Stop()

	if taskType == "" {
		taskType = e.taskType
	}
	taskType = normalizeTaskType(taskType)

	textLen := len(text)
	logging.EmbeddingDebug("GenAI.Embed: starting embed request, text_length=%d chars, model=%s, task_type=%s", textLen, e.model, taskType)

	contents := []*genai.Content{
		genai.NewContentFromText(text, genai.RoleUser),
	}

	cfg := &genai.EmbedContentConfig{
		OutputDimensionality: new(int32(3072)),
	}
	if taskType != "" {
		cfg.TaskType = taskType
	}

	logging.EmbeddingDebug("GenAI.Embed: calling EmbedContent API")
	apiStart := time.Now()

	result, err := e.client.Models.EmbedContent(ctx, e.model, contents, cfg)
	apiLatency := time.Since(apiStart)

	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("GenAI.Embed: API call failed after %v: %v", apiLatency, err)
		return nil, fmt.Errorf("GenAI embed failed: %w", err)
	}

	logging.EmbeddingDebug("GenAI.Embed: API response received in %v", apiLatency)

	if len(result.Embeddings) != 1 {
		logging.Get(logging.CategoryEmbedding).Error("GenAI.Embed: API returned %d embeddings for one input", len(result.Embeddings))
		return nil, fmt.Errorf("GenAI embed returned %d vectors for one input", len(result.Embeddings))
	}
	if result.Embeddings[0] == nil {
		logging.Get(logging.CategoryEmbedding).Error("GenAI.Embed: nil embedding returned from API")
		return nil, fmt.Errorf("GenAI embed returned nil embedding")
	}
	if err := validateEmbeddingVector(result.Embeddings[0].Values); err != nil {
		return nil, fmt.Errorf("GenAI embed returned invalid vector: %w", err)
	}

	dimensions := len(result.Embeddings[0].Values)
	logging.Embedding("GenAI.Embed: completed successfully, dimensions=%d, api_latency=%v", dimensions, apiLatency)

	return result.Embeddings[0].Values, nil
}

// EmbedBatch generates embeddings for multiple texts.
// GenAI has native batch support but limits batches to 100 items.
// This function automatically chunks larger batches and concatenates results.
func (e *GenAIEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return e.embedBatchWithTask(ctx, texts, e.taskType)
}

// EmbedBatchWithTask generates embeddings for multiple texts using an explicit task type.
func (e *GenAIEngine) EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	return e.embedBatchWithTask(ctx, texts, taskType)
}

func (e *GenAIEngine) embedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "GenAI.EmbedBatch")
	defer timer.Stop()

	if taskType == "" {
		taskType = e.taskType
	}
	taskType = normalizeTaskType(taskType)

	logging.Embedding("GenAI.EmbedBatch: starting native batch embed for %d texts (task_type=%s)", len(texts), taskType)

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
		return e.embedBatchChunk(ctx, texts, taskType)
	}

	// Chunk into batches of maxBatchSize and process in parallel.
	// Order is preserved by writing each chunk's result into a fixed slot of
	// chunkResults indexed by the batch position, then flattening after Wait.
	numBatches := (len(texts) + maxBatchSize - 1) / maxBatchSize
	logging.Embedding("GenAI.EmbedBatch: chunking %d texts into %d batches of up to %d items (parallelism=%d)", len(texts), numBatches, maxBatchSize, batchParallelism)

	chunkResults := make([][][]float32, numBatches)

	eg, gctx := errgroup.WithContext(ctx)
	eg.SetLimit(batchParallelism)

	parallelStart := time.Now()
	for batchIdx := range numBatches {
		batchIdx := batchIdx // capture before goroutine launch
		start := batchIdx * maxBatchSize
		end := min(start+maxBatchSize, len(texts))
		chunk := texts[start:end]

		eg.Go(func() error {
			// errgroup.WithContext cancels gctx on first error; check before
			// issuing the HTTP call so queued goroutines bail out fast.
			select {
			case <-gctx.Done():
				return gctx.Err()
			default:
			}

			logging.EmbeddingDebug("GenAI.EmbedBatch: processing batch %d/%d with %d texts (indices %d-%d)",
				batchIdx+1, numBatches, len(chunk), start, end-1)

			chunkEmbeddings, err := e.embedBatchChunk(gctx, chunk, taskType)
			if err != nil {
				return fmt.Errorf("batch %d/%d failed: %w", batchIdx+1, numBatches, err)
			}
			chunkResults[batchIdx] = chunkEmbeddings
			return nil
		})
	}

	if err := eg.Wait(); err != nil {
		return nil, err
	}
	parallelLatency := time.Since(parallelStart)

	allEmbeddings := make([][]float32, 0, len(texts))
	for _, chunk := range chunkResults {
		allEmbeddings = append(allEmbeddings, chunk...)
	}

	dimensions := 0
	if len(allEmbeddings) > 0 && len(allEmbeddings[0]) > 0 {
		dimensions = len(allEmbeddings[0])
	}

	logging.Embedding("GenAI.EmbedBatch: completed successfully, processed %d texts in %d parallel batches (parallelism=%d), wall_time=%v, dimensions=%d",
		len(texts), numBatches, batchParallelism, parallelLatency, dimensions)

	return allEmbeddings, nil
}

// embedBatchChunk processes a single batch chunk (must be <= maxBatchSize).
func (e *GenAIEngine) embedBatchChunk(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = genai.NewContentFromText(text, genai.RoleUser)
	}

	cfg := &genai.EmbedContentConfig{
		OutputDimensionality: new(int32(3072)),
	}
	if taskType != "" {
		cfg.TaskType = taskType
	}

	logging.EmbeddingDebug("GenAI.embedBatchChunk: calling EmbedContent API with %d contents", len(contents))
	apiStart := time.Now()

	result, err := e.client.Models.EmbedContent(ctx,
		e.model,
		contents,
		cfg,
	)
	apiLatency := time.Since(apiStart)

	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("GenAI.embedBatchChunk: API call failed after %v: %v", apiLatency, err)
		return nil, fmt.Errorf("GenAI batch embed failed: %w", err)
	}

	logging.EmbeddingDebug("GenAI.embedBatchChunk: API response received in %v, got %d embeddings", apiLatency, len(result.Embeddings))

	embeddings := make([][]float32, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		if emb == nil {
			return nil, fmt.Errorf("GenAI batch embed returned nil embedding at index %d", i)
		}
		embeddings[i] = emb.Values
	}
	if err := validateEmbeddingBatchResponse(len(texts), embeddings); err != nil {
		return nil, fmt.Errorf("GenAI batch embed returned invalid response: %w", err)
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

// EmbedBatchJob submits an asynchronous embedding batch job via
// client.Batches.CreateEmbeddings. Returns the job handle so callers can poll
// with client.Batches.Get(...) and read results from BatchJob.Dest once the
// job reaches JobStateSucceeded.
//
// Use this for large-corpus reembed paths (e.g. corpus_builder ingest of
// thousands of atoms). For smaller batches the synchronous EmbedBatch with
// parallelism=6 is faster and avoids the polling round-trip.
//
// NOTE: CreateEmbeddings is only supported on the Gemini Developer API
// backend; calling it against BackendVertexAI returns an error from the SDK.
// The implementation is marked experimental by the SDK (v1.58) and may
// change. TaskType and OutputDimensionality are carried inside the
// per-batch Config field on EmbedContentBatch, not on the job-level
// CreateEmbeddingsBatchJobConfig.
func (e *GenAIEngine) EmbedBatchJob(ctx context.Context, texts []string, taskType string) (*genai.BatchJob, error) {
	timer := logging.StartTimer(logging.CategoryEmbedding, "GenAI.EmbedBatchJob")
	defer timer.Stop()

	if len(texts) == 0 {
		return nil, fmt.Errorf("EmbedBatchJob: texts must be non-empty")
	}

	if taskType == "" {
		taskType = e.taskType
	}
	taskType = normalizeTaskType(taskType)

	logging.Embedding("GenAI.EmbedBatchJob: submitting async batch for %d texts (task_type=%s, model=%s)",
		len(texts), taskType, e.model)

	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = genai.NewContentFromText(text, genai.RoleUser)
	}

	cfg := &genai.EmbedContentConfig{
		OutputDimensionality: new(int32(3072)),
	}
	if taskType != "" {
		cfg.TaskType = taskType
	}

	model := e.model
	src := &genai.EmbeddingsBatchJobSource{
		InlinedRequests: &genai.EmbedContentBatch{
			Contents: contents,
			Config:   cfg,
		},
	}

	jobCfg := &genai.CreateEmbeddingsBatchJobConfig{
		DisplayName: fmt.Sprintf("codenerd-embed-%d", time.Now().UnixNano()),
	}

	apiStart := time.Now()
	job, err := e.client.Batches.CreateEmbeddings(ctx, &model, src, jobCfg)
	apiLatency := time.Since(apiStart)

	if err != nil {
		logging.Get(logging.CategoryEmbedding).Error("GenAI.EmbedBatchJob: submission failed after %v: %v", apiLatency, err)
		return nil, fmt.Errorf("GenAI embed batch job submit failed: %w", err)
	}

	jobName := ""
	if job != nil {
		jobName = job.Name
	}
	logging.Embedding("GenAI.EmbedBatchJob: submitted job=%s in %v", jobName, apiLatency)

	return job, nil
}
