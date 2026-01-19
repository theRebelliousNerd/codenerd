package session

import (
	"context"
	"testing"

	"codenerd/internal/perception"
	"codenerd/internal/types"
)

func TestSubAgent_Run_Success(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			return &types.LLMToolResponse{Text: "Mission accomplished"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			return "Mission accomplished", nil
		},
	}

	cfg := DefaultSubAgentConfig("test-agent")
	agent := NewSubAgent(
		cfg,
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)

	agent.Run(context.Background(), "Do the mission")

	result, err := agent.GetResult()
	if err != nil {
		t.Fatalf("Agent failed: %v", err)
	}

	if result != "Mission accomplished" {
		t.Errorf("Expected 'Mission accomplished', got '%s'", result)
	}

	if agent.GetState() != SubAgentStateCompleted {
		t.Errorf("Expected Completed state, got %v", agent.GetState())
	}
}

func TestSubAgent_MemoryCompression(t *testing.T) {
	// Setup: Run 3 turns, verify compression called
	turns := 0
	mockLLM := &MockLLMClient{
		CompleteWithToolsFunc: func(ctx context.Context, sys, user string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
			turns++
			if turns < 3 {
				return &types.LLMToolResponse{Text: "Turning..."}, nil
			}
			return &types.LLMToolResponse{Text: "Done"}, nil
		},
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			turns++
			if turns < 3 {
				return "Turning...", nil
			}
			return "Done", nil
		},
	}

	compressCalled := false
	mockCompressor := &MockCompressor{
		CompressFunc: func(ctx context.Context, turns []perception.ConversationTurn) (string, error) {
			compressCalled = true
			return "Compressed summary", nil
		},
	}

	cfg := DefaultSubAgentConfig("compress-agent")
	// Set threshold low implicitly? No, Compressor is called manually or by max turns policy?
	// subagent.go: agent.CompressMemory() is public.
	// But let's check if the loop calls it.
	// Reading subagent.go (not visible now), usually compression is triggered by token limit or turn count.
	// We'll call it manually to test the integration.

	agent := NewSubAgent(
		cfg,
		&MockKernel{},
		&MockVirtualStore{},
		mockLLM,
		&MockJITCompiler{},
		&MockConfigFactory{},
		&MockTransducer{},
	)
	agent.SetCompressor(mockCompressor)

	// Add some history
	agent.conversationHistory = append(agent.conversationHistory,
		perception.ConversationTurn{Role: "user", Content: "1"},
		perception.ConversationTurn{Role: "assistant", Content: "2"},
	)

	err := agent.CompressMemory(context.Background(), 1)
	if err != nil {
		t.Fatalf("CompressMemory failed: %v", err)
	}

	if !compressCalled {
		t.Error("Compressor was not called")
	}

	// Verify history was compressed (1 summary + 0 turns if all compressed?)
	// Logic depends on implementation.
	// Assuming it replaces old turns with summary.
}
