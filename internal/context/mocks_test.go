package context

import (
	"codenerd/internal/types"
	"context"
)

// MockLLMClient implements perception.LLMClient for testing.
type MockLLMClient struct {
	CompleteFunc           func(ctx context.Context, prompt string) (string, error)
	CompleteWithSystemFunc func(ctx context.Context, sys, user string) (string, error)
	CompleteWithToolsFunc  func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error)
}

func (m *MockLLMClient) Complete(ctx context.Context, prompt string) (string, error) {
	if m.CompleteFunc != nil {
		return m.CompleteFunc(ctx, prompt)
	}
	return "Mock completion", nil
}

func (m *MockLLMClient) CompleteWithSystem(ctx context.Context, sys, user string) (string, error) {
	if m.CompleteWithSystemFunc != nil {
		return m.CompleteWithSystemFunc(ctx, sys, user)
	}
	return "Mock completion with system", nil
}

func (m *MockLLMClient) CompleteWithTools(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if m.CompleteWithToolsFunc != nil {
		return m.CompleteWithToolsFunc(ctx, sys, user, tools)
	}
	return &types.LLMToolResponse{Text: "Mock tool response"}, nil
}

// Helper to create a test budget config
func DefaultTestContextConfig() CompressorConfig {
	return CompressorConfig{
		TotalBudget:            1000,
		WorkingReserve:         500,
		CoreReserve:            50,
		AtomReserve:            300,
		HistoryReserve:         150,
		CompressionThreshold:   0.5, // Compress at 500 tokens
		RecentTurnWindow:       2,   // Keep 2 turns uncompressed
		TargetCompressionRatio: 0.1,
		ActivationThreshold:    50.0, // Lower for testing
	}
}
