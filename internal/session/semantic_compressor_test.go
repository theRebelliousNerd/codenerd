package session

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"unicode/utf8"

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
	t.Parallel()
	const limit = 60000
	var calls, finals int
	var mu sync.Mutex
	mockClient := &MockLLMClient{
		CompleteWithSystemFunc: func(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
			mu.Lock()
			defer mu.Unlock()
			calls++
			if len(userPrompt) > limit+2000 {
				return "", fmt.Errorf("prompt too large! size: %d", len(userPrompt))
			}
			if strings.Contains(userPrompt, "TRUNCATED") {
				return "", fmt.Errorf("history was dropped instead of summarized")
			}
			if strings.Contains(userPrompt, "<summary part=") {
				finals++
			}
			return "Summary", nil
		},
	}
	compressor := NewSemanticCompressor(mockClient)
	compressor.allowance = func() int { return limit }

	// Gap 6 & 7: 100,000 items. Every one must reach a summary: the history
	// is ~8 MB, so it is summarized in parts and the part summaries again.
	var turns []perception.ConversationTurn
	for range 100000 {
		turns = append(turns, perception.ConversationTurn{Role: "user", Content: "A very long turn content that goes on and on."})
	}

	got, err := compressor.Compress(context.Background(), turns)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}
	if got != "Summary" || calls < 3 || finals != 1 {
		t.Fatalf("summary %q after %d calls (%d over part summaries); want parts then one final", got, calls, finals)
	}
}

// One turn larger than the allowance is split inside, rune-safe, not cut.
func TestPackParts_SplitsAnOversizedTurnWithoutLoss(t *testing.T) {
	line := strings.Repeat("é", 50) // 100 bytes
	parts := packParts([]string{"ab", line, "cd"}, 30)
	joined := strings.Join(parts, "")
	if joined != "ab"+line+"cd" {
		t.Fatalf("parts lost or reordered text: %q", joined)
	}
	for _, p := range parts {
		if len(p) > 30 || !utf8.ValidString(p) {
			t.Fatalf("part %q exceeds the bound or splits a rune", p)
		}
	}
}

// Summaries that never shrink stop with an error rather than looping.
func TestSemanticCompressor_StopsWhenSummariesDoNotShrink(t *testing.T) {
	compressor := NewSemanticCompressor(&MockLLMClient{
		CompleteWithSystemFunc: func(context.Context, string, string) (string, error) {
			return strings.Repeat("x", 500), nil
		},
	})
	compressor.allowance = func() int { return 400 }
	turns := make([]perception.ConversationTurn, 20)
	for i := range turns {
		turns[i] = perception.ConversationTurn{Role: "user", Content: strings.Repeat("y", 300)}
	}
	if _, err := compressor.Compress(context.Background(), turns); err == nil {
		t.Fatal("Compress returned no error for summaries that never shrink")
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
