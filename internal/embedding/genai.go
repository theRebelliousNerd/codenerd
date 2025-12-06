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
	taskType genai.TaskType
}

// NewGenAIEngine creates a new GenAI embedding engine.
func NewGenAIEngine(apiKey, model, taskType string) (*GenAIEngine, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("GenAI API key is required")
	}

	if model == "" {
		model = "gemini-embedding-001"
	}

	ctx := context.Background()
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create GenAI client: %w", err)
	}

	// Parse task type (see https://ai.google.dev/gemma/docs/embeddinggemma)
	var task genai.TaskType
	switch taskType {
	case "SEMANTIC_SIMILARITY", "":
		task = genai.TaskTypeSemanticSimilarity
	case "CLASSIFICATION":
		task = genai.TaskTypeClassification
	case "CLUSTERING":
		task = genai.TaskTypeClustering
	case "RETRIEVAL_DOCUMENT":
		task = genai.TaskTypeRetrievalDocument
	case "RETRIEVAL_QUERY":
		task = genai.TaskTypeRetrievalQuery
	case "CODE_RETRIEVAL_QUERY":
		task = genai.TaskTypeCodeRetrievalQuery
	case "QUESTION_ANSWERING":
		task = genai.TaskTypeQuestionAnswering
	case "FACT_VERIFICATION":
		task = genai.TaskTypeFactVerification
	default:
		task = genai.TaskTypeSemanticSimilarity
	}

	return &GenAIEngine{
		client:   client,
		model:    model,
		taskType: task,
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
		&genai.EmbedContentRequest{
			TaskType: e.taskType,
		},
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
		&genai.EmbedContentRequest{
			TaskType: e.taskType,
		},
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

// Close closes the GenAI client.
func (e *GenAIEngine) Close() error {
	if e.client != nil {
		return e.client.Close()
	}
	return nil
}
