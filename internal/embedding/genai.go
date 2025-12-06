package embedding

import (
	"context"
	"fmt"

	"google.golang.org/genai"
)

// =============================================================================
// GOOGLE GENAI EMBEDDING ENGINE
// =============================================================================

// GenAIEngine generates embeddings using Google's Gemini API.
type GenAIEngine struct {
	client   *genai.Client
	model    string
	taskType string // Task type as string for API flexibility
}

// NewGenAIEngine creates a new GenAI embedding engine.
func NewGenAIEngine(apiKey, model, taskType string) (*GenAIEngine, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GenAI API key is required")
	}

	if model == "" {
		model = "gemini-embedding-001"
	}

	if taskType == "" {
		taskType = "SEMANTIC_SIMILARITY"
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	return &GenAIEngine{
		client:   client,
		model:    model,
		taskType: taskType,
	}, nil
}

// Embed generates an embedding for a single text.
func (e *GenAIEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	contents := []*genai.Content{
		genai.NewContentFromText(text, genai.RoleUser),
	}

	result, err := e.client.Models.EmbedContent(ctx,
		e.model,
		contents,
		nil, // TaskType set via model, not request
	)
	if err != nil {
		return nil, fmt.Errorf("GenAI embed failed: %w", err)
	}

	if len(result.Embeddings) == 0 {
		return nil, fmt.Errorf("no embeddings returned")
	}

	return result.Embeddings[0].Values, nil
}

// EmbedBatch generates embeddings for multiple texts.
// GenAI has native batch support.
func (e *GenAIEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if len(texts) == 0 {
		return nil, nil
	}

	contents := make([]*genai.Content, len(texts))
	for i, text := range texts {
		contents[i] = genai.NewContentFromText(text, genai.RoleUser)
	}

	result, err := e.client.Models.EmbedContent(ctx,
		e.model,
		contents,
		nil, // TaskType set via model, not request
	)
	if err != nil {
		return nil, fmt.Errorf("GenAI batch embed failed: %w", err)
	}

	embeddings := make([][]float32, len(result.Embeddings))
	for i, emb := range result.Embeddings {
		embeddings[i] = emb.Values
	}

	return embeddings, nil
}

// Dimensions returns the dimensionality of embeddings.
// gemini-embedding-001 produces 768-dimensional vectors.
func (e *GenAIEngine) Dimensions() int {
	// gemini-embedding-001: 768 dimensions
	return 768
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
