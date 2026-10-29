package perception

import "testing"

// The factory promises classification clients carry classification-specific
// settings (thinking off, small ceiling). Gemini's constructor defaults are
// thinking-on with a 64K budget, so the factory must override both;
// otherwise every turn burns a thinking trace plus a main-sized ceiling
// for a one-label reply.
func TestNewRawClassificationClient_GeminiThinkingOffAndBounded(t *testing.T) {
	cfg := &ProviderConfig{
		Provider:            ProviderGemini,
		APIKey:              "test-key",
		Model:               "gemini-2.5-flash",
		ClassificationModel: "gemini-2.5-flash",
	}
	client, err := newRawClassificationClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("classification client: %v", err)
	}
	gc, ok := client.(*GeminiClient)
	if !ok {
		t.Fatalf("expected *GeminiClient, got %T", client)
	}
	if gc.enableThinking {
		t.Error("gemini classification client thinks: every turn would burn a thinking trace for a label")
	}
	if gc.maxOutputTokens != classificationMaxOutputTokens {
		t.Errorf("maxOutputTokens = %d, want classification ceiling %d", gc.maxOutputTokens, classificationMaxOutputTokens)
	}
}

// Gemini 3 models force thinking on in the client constructor; the factory
// cannot switch that off, but the tight ceiling plus the minimal level must
// still bound the spend.
func TestNewRawClassificationClient_Gemini3ForcedThinkingStillBounded(t *testing.T) {
	cfg := &ProviderConfig{
		Provider:            ProviderGemini,
		APIKey:              "test-key",
		Model:               "gemini-3.5-flash",
		ClassificationModel: "gemini-3.5-flash",
	}
	client, err := newRawClassificationClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("classification client: %v", err)
	}
	gc, ok := client.(*GeminiClient)
	if !ok {
		t.Fatalf("expected *GeminiClient, got %T", client)
	}
	if !gc.enableThinking {
		t.Error("expected forced thinking for gemini-3 (constructor forces it); test premise changed")
	}
	if gc.maxOutputTokens != classificationMaxOutputTokens {
		t.Errorf("maxOutputTokens = %d, want classification ceiling %d", gc.maxOutputTokens, classificationMaxOutputTokens)
	}
	if gc.thinkingLevel != "minimal" {
		t.Errorf("thinkingLevel = %q, want minimal to bound the forced trace", gc.thinkingLevel)
	}
}
