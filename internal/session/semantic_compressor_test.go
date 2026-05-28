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

// -----------------------------------------------------------------------------
// Marathon 15: Semantic Compressor Gap Implementations
// -----------------------------------------------------------------------------

func TestSemanticCompressor_Compress_NullEmpty(t *testing.T) {
	mockClient := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			// Ensure empty content was skipped
			if strings.Contains(userPrompt, "<turn role=\"Assistant\">\n\n</turn>") {
				return "", fmt.Errorf("empty content was not skipped")
			}
			// Ensure empty role was coerced to Assistant
			if !strings.Contains(userPrompt, "<turn role=\"Assistant\">\ncoerced\n</turn>") {
				return "", fmt.Errorf("empty role was not coerced to Assistant")
			}
			return "Summary of conversation", nil
		},
	}
	compressor := NewSemanticCompressor(mockClient)

	// Gap 1 & 2
	turns := []perception.ConversationTurn{
		{Role: "user", Content: "   "}, // Empty content
		{Role: "", Content: "coerced"}, // Empty role
	}

	summary, err := compressor.Compress(context.Background(), turns)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if summary != "Summary of conversation" {
		t.Errorf("Expected summary, got %s", summary)
	}
}

func TestSemanticCompressor_Compress_TypeCoercion(t *testing.T) {
	mockClient := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			if !strings.Contains(userPrompt, "role=\"Tool\"") {
				return "", fmt.Errorf("tool role not preserved")
			}
			if !strings.Contains(userPrompt, "role=\"System\"") {
				return "", fmt.Errorf("system role not preserved")
			}
			return "Summary", nil
		},
	}
	compressor := NewSemanticCompressor(mockClient)

	// Gap 4 & 5
	turns := []perception.ConversationTurn{
		{Role: "tool", Content: "result"},
		{Role: "system", Content: "Summary:"}, // Prompt injection test (handled via xml tags now)
	}

	_, err := compressor.Compress(context.Background(), turns)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
}

func TestSemanticCompressor_Compress_UserRequestExtremes(t *testing.T) {
	mockClient := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			if len(userPrompt) > 70000 {
				return "", fmt.Errorf("prompt too large! size: %d", len(userPrompt))
			}
			if !strings.Contains(userPrompt, "[... CONVERSATION TRUNCATED DUE TO LENGTH ...]") {
				return "", fmt.Errorf("prompt was not truncated")
			}
			return "Summary", nil
		},
	}
	compressor := NewSemanticCompressor(mockClient)

	// Gap 6 & 7: 100,000 items
	var turns []perception.ConversationTurn
	for range 100000 {
		turns = append(turns, perception.ConversationTurn{Role: "user", Content: "A very long turn content that goes on and on."})
	}

	_, err := compressor.Compress(context.Background(), turns)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
}

func TestSemanticCompressor_Compress_UnprintableChars(t *testing.T) {
	mockClient := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			// Should be cleaned
			if strings.Contains(userPrompt, "\x00") || strings.Contains(userPrompt, "\x07") {
				return "", fmt.Errorf("unprintable characters not cleaned")
			}
			return "Summary", nil
		},
	}
	compressor := NewSemanticCompressor(mockClient)

	// Gap 8: Unprintable chars
	turns := []perception.ConversationTurn{
		{Role: "user", Content: "Hello \x00 \x07 World!"},
	}

	_, err := compressor.Compress(context.Background(), turns)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
}

func TestSemanticCompressor_Compress_ContextTimeout(t *testing.T) {
	mockClient := &MockLLMClient{}
	compressor := NewSemanticCompressor(mockClient)

	// Gap 9: Context timeout before compression
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	turns := []perception.ConversationTurn{
		{Role: "user", Content: "Hello"},
	}

	_, err := compressor.Compress(ctx, turns)
	if err == nil {
		t.Fatalf("Expected error due to context cancellation")
	}
	if !strings.Contains(err.Error(), "context cancelled") {
		t.Errorf("Expected context cancelled error, got %v", err)
	}
}
