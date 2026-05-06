package session

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/perception"
)

func TestSemanticCompressor_Compress(t *testing.T) {
	mockClient := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			if !strings.Contains(systemPrompt, "context compressor") {
				return "", fmt.Errorf("unexpected system prompt")
			}
			return "Summary of conversation", nil
		},
	}

	compressor := NewSemanticCompressor(mockClient)
	turns := []perception.ConversationTurn{
		{Role: "user", Content: "Hello"},
		{Role: "assistant", Content: "Hi there"},
	}

	summary, err := compressor.Compress(context.Background(), turns)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if summary != "Summary of conversation" {
		t.Errorf("Expected summary 'Summary of conversation', got '%s'", summary)
	}
}

func TestSemanticCompressor_Compress_Empty(t *testing.T) {
	mockClient := &MockLLMClient{}
	compressor := NewSemanticCompressor(mockClient)

	summary, err := compressor.Compress(context.Background(), nil)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if summary != "" {
		t.Errorf("Expected empty summary, got '%s'", summary)
	}
}


// TODO: TEST_GAP: Null/Undefined/Empty: Gap 1 - What happens if turns contains items where turn.Content is empty?
// TODO: TEST_GAP: Null/Undefined/Empty: Gap 2 - What happens if turn.Role is empty? (Coerced to "Assistant")
// TODO: TEST_GAP: Null/Undefined/Empty: Gap 3 - What if the LLM client drops the system prompt?
// TODO: TEST_GAP: Type Coercion: Gap 4 - What if turn.Role is "tool" or "system"? It coerces to "Assistant", losing semantic meaning.
// TODO: TEST_GAP: Type Coercion: Gap 5 - What if turn.Content contains prompt injection strings like "Summary:"?
// TODO: TEST_GAP: User Request Extremes: Gap 6 - What happens if turns contains 100,000 items? strings.Builder lacks Grow(), causing excessive allocations/OOM.
// TODO: TEST_GAP: User Request Extremes: Gap 7 - What happens if the combined text exceeds the LLM token limit?
// TODO: TEST_GAP: User Request Extremes: Gap 8 - What happens if 50MB of RTL or unprintable Unicode characters are injected?
// TODO: TEST_GAP: State Conflicts: Gap 9 - Context Timeout: Does the compressor handle CompleteWithSystem context cancellation/timeouts cleanly?
// TODO: TEST_GAP: State Conflicts: Gap 10 - Concurrent Access: What happens if the turns slice is mutated by another goroutine during compression?
