package perception

import (
	"context"
	"testing"
)

func TestThinkingClient_ParseIntent(t *testing.T) {
	tc := newCompiledUnderstandingTransducer(t, &textGeminiClient{response: geminiSchemaEnvelope}).(*UnderstandingTransducer)
	ctx := context.Background()
	_, err := tc.ParseIntent(ctx, "hello")
	if err != nil {
		t.Errorf("Expected no error from ParseIntent, got %v", err)
	}
}

func TestThinkingClient_ParseIntentWithContext(t *testing.T) {
	tc := newCompiledUnderstandingTransducer(t, &textGeminiClient{response: geminiSchemaEnvelope}).(*UnderstandingTransducer)
	ctx := context.Background()
	_, err := tc.ParseIntentWithContext(ctx, "hello", nil)
	if err != nil {
		t.Errorf("Expected no error from ParseIntentWithContext, got %v", err)
	}
}
