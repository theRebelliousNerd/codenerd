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
