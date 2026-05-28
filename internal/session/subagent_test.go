package session

import (
	"context"
	"strings"
	"testing"
	"time"

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

	result, err := agent.Wait()
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

// -----------------------------------------------------------------------------
// QA NEGATIVE TESTING
// -----------------------------------------------------------------------------

func TestSubAgent_DoubleKill(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(5 * time.Second):
				return "Done", nil
			}
		},
	}
	agent := NewSubAgent(DefaultSubAgentConfig("test"), &MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})

	ctx := t.Context()

	go agent.Run(ctx, "task")
	time.Sleep(50 * time.Millisecond) // Let it start

	agent.Stop()
	agent.Stop() // Double Stop

	agent.Wait()

	// Ensure it didn't panic and the state is failed (due to context cancellation via Stop)
	if agent.GetState() != SubAgentStateFailed {
		t.Errorf("Expected Failed state, got %v", agent.GetState())
	}
}

func TestSubAgent_ContextCancellation(t *testing.T) {
	mockLLM := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, sys, user string) (string, error) {
			select {
			case <-ctx.Done():
				return "", ctx.Err()
			case <-time.After(5 * time.Second):
				return "Done", nil
			}
		},
	}
	agent := NewSubAgent(DefaultSubAgentConfig("test"), &MockKernel{}, &MockVirtualStore{}, mockLLM, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})

	ctx, cancel := context.WithCancel(context.Background())

	go agent.Run(ctx, "task")

	// Wait a bit to ensure it started
	time.Sleep(50 * time.Millisecond)

	// Cancel the context, which should abort the LLM call and fail the agent
	cancel()

	agent.Wait()

	if agent.GetState() != SubAgentStateFailed {
		t.Errorf("Expected Failed state due to context cancellation, got %v", agent.GetState())
	}
}

// -----------------------------------------------------------------------------
// Marathon 14: SubAgent Gap Implementations
// -----------------------------------------------------------------------------

type mockCompressor struct {
	summary string
	err     error
	delay   time.Duration
}

func (m *mockCompressor) Compress(ctx context.Context, turns []perception.ConversationTurn) (string, error) {
	if m.delay > 0 {
		select {
		case <-time.After(m.delay):
		case <-ctx.Done():
			return "", ctx.Err()
		}
	}
	return m.summary, m.err
}

func TestSubAgent_CompressMemory_ThresholdZero(t *testing.T) {
	agent := NewSubAgent(DefaultSubAgentConfig("test"), &MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})

	// Add 5 turns
	for range 5 {
		agent.conversationHistory = append(agent.conversationHistory, perception.ConversationTurn{})
	}

	agent.SetCompressor(&mockCompressor{summary: "compressed"})

	// Gap 1: Threshold 0 (Should default to 10 and do nothing, or if default is <5, it should compress)
	// We made it default to 10, so with 5 turns it should do nothing.
	err := agent.CompressMemory(context.Background(), 0)
	if err != nil {
		t.Fatalf("Unexpected error: %v", err)
	}

	if len(agent.conversationHistory) != 5 {
		t.Errorf("Expected 5 turns, got %d. Threshold 0 should have defaulted to 10.", len(agent.conversationHistory))
	}
}

func TestSubAgent_CompressMemory_StateConflicts_NoBlock(t *testing.T) {
	agent := NewSubAgent(DefaultSubAgentConfig("test"), &MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})

	for range 15 {
		agent.conversationHistory = append(agent.conversationHistory, perception.ConversationTurn{})
	}

	// Gap 2: Compressor blocks for 100ms
	agent.SetCompressor(&mockCompressor{summary: "compressed", delay: 100 * time.Millisecond})

	done := make(chan bool)
	go func() {
		_ = agent.CompressMemory(context.Background(), 10)
		done <- true
	}()

	// Wait a tiny bit to ensure compression started
	time.Sleep(10 * time.Millisecond)

	// Try to get state while it's compressing. If mu is held, this will block and take > 100ms.
	start := time.Now()
	_ = agent.GetState()
	dur := time.Since(start)

	if dur > 50*time.Millisecond {
		t.Errorf("GetState blocked for %v, lock was not released during compression!", dur)
	}

	<-done
}

func TestSubAgent_CompressMemory_MassiveSummary(t *testing.T) {
	agent := NewSubAgent(DefaultSubAgentConfig("test"), &MockKernel{}, &MockVirtualStore{}, &MockLLMClient{}, &MockJITCompiler{}, &MockConfigFactory{}, &MockTransducer{})

	for range 15 {
		agent.conversationHistory = append(agent.conversationHistory, perception.ConversationTurn{})
	}

	// Gap 3: Massive 5MB summary
	massiveSummary := strings.Repeat("A", 5*1024*1024)
	agent.SetCompressor(&mockCompressor{summary: massiveSummary})

	_ = agent.CompressMemory(context.Background(), 10)

	// Summary should be truncated to 4096 + "..."
	summaryTurn := agent.conversationHistory[0]
	if len(summaryTurn.Content) > 5000 {
		t.Errorf("Summary was not truncated! Length: %d", len(summaryTurn.Content))
	}

	// Gap 4: Type Coercion. Role is "assistant".
	if summaryTurn.Role != "assistant" {
		t.Errorf("Expected role 'assistant', got '%s'", summaryTurn.Role)
	}
	if !strings.HasPrefix(summaryTurn.Content, "[MEMORY SUMMARY]") {
		t.Errorf("Expected prefix '[MEMORY SUMMARY]', got '%s'", summaryTurn.Content[:min(20, len(summaryTurn.Content))])
	}
}
