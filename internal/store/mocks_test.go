package store

import (
	"context"
	"fmt"
)

// MockEmbeddingEngine implements embedding.EmbeddingEngine for testing.
type MockEmbeddingEngine struct {
	EmbedFunc              func(ctx context.Context, text string) ([]float32, error)
	EmbedBatchFunc         func(ctx context.Context, texts []string) ([][]float32, error)
	DimensionsFunc         func() int
	NameFunc               func() string
	EmbedWithTaskFunc      func(ctx context.Context, text string, taskType string) ([]float32, error)
	EmbedBatchWithTaskFunc func(ctx context.Context, texts []string, taskType string) ([][]float32, error)
}

func (m *MockEmbeddingEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	if m.EmbedFunc != nil {
		return m.EmbedFunc(ctx, text)
	}
	// Return a dummy vector of length 4 by default
	return []float32{0.1, 0.2, 0.3, 0.4}, nil
}

func (m *MockEmbeddingEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	if m.EmbedBatchFunc != nil {
		return m.EmbedBatchFunc(ctx, texts)
	}
	// Return dummy vectors
	result := make([][]float32, len(texts))
	for i := range texts {
		result[i] = []float32{0.1, 0.2, 0.3, 0.4}
	}
	return result, nil
}

func (m *MockEmbeddingEngine) Dimensions() int {
	if m.DimensionsFunc != nil {
		return m.DimensionsFunc()
	}
	return 4
}

func (m *MockEmbeddingEngine) Name() string {
	if m.NameFunc != nil {
		return m.NameFunc()
	}
	return "mock-embedding-engine"
}

func (m *MockEmbeddingEngine) EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error) {
	if m.EmbedWithTaskFunc != nil {
		return m.EmbedWithTaskFunc(ctx, text, taskType)
	}
	return m.Embed(ctx, text)
}

func (m *MockEmbeddingEngine) EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	if m.EmbedBatchWithTaskFunc != nil {
		return m.EmbedBatchWithTaskFunc(ctx, texts, taskType)
	}
	return m.EmbedBatch(ctx, texts)
}

// Ensure MockEmbeddingEngine implements all interfaces
var _ interface {
	Embed(ctx context.Context, text string) ([]float32, error)
	EmbedBatch(ctx context.Context, texts []string) ([][]float32, error)
	Dimensions() int
	Name() string
	EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error)
	EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error)
} = (*MockEmbeddingEngine)(nil)

// MockErrorEmbeddingEngine always returns errors
type MockErrorEmbeddingEngine struct{}

func (m *MockErrorEmbeddingEngine) Embed(ctx context.Context, text string) ([]float32, error) {
	return nil, fmt.Errorf("mock error")
}

func (m *MockErrorEmbeddingEngine) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	return nil, fmt.Errorf("mock error")
}

func (m *MockErrorEmbeddingEngine) Dimensions() int {
	return 4
}

func (m *MockErrorEmbeddingEngine) Name() string {
	return "mock-error-engine"
}

func (m *MockErrorEmbeddingEngine) EmbedWithTask(ctx context.Context, text string, taskType string) ([]float32, error) {
	return nil, fmt.Errorf("mock error")
}

func (m *MockErrorEmbeddingEngine) EmbedBatchWithTask(ctx context.Context, texts []string, taskType string) ([][]float32, error) {
	return nil, fmt.Errorf("mock error")
}
