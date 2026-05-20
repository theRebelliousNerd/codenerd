package perception

import (
	"context"
	"testing"
)

func TestNewGeminiThinkingTransducer(t *testing.T) {
	base := NewUnderstandingTransducer(&baseMockLLMClient{}).(*UnderstandingTransducer)
	tc := NewGeminiThinkingTransducer(base)
	if tc == nil {
		t.Errorf("NewGeminiThinkingTransducer returned nil")
	}
}

func TestGeminiThinkingTransducer_ParseIntent(t *testing.T) {
	base := NewUnderstandingTransducer(&baseMockLLMClient{}).(*UnderstandingTransducer)
	tc := NewGeminiThinkingTransducer(base)
	ctx := context.Background()
	_, err := tc.ParseIntent(ctx, "hello")
	if err != nil {
		t.Errorf("Expected no error from ParseIntent, got %v", err)
	}
}

func TestGeminiThinkingTransducer_ParseIntentWithContext(t *testing.T) {
	base := NewUnderstandingTransducer(&baseMockLLMClient{}).(*UnderstandingTransducer)
	tc := NewGeminiThinkingTransducer(base)
	ctx := context.Background()
	_, err := tc.ParseIntentWithContext(ctx, "hello", nil)
	if err != nil {
		t.Errorf("Expected no error from ParseIntentWithContext, got %v", err)
	}
}
