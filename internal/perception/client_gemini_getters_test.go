package perception

import "testing"

func TestGeminiClientGettersSetters(t *testing.T) {
	c := NewGeminiClient("test-key")
	if c == nil {
		t.Fatal("NewGeminiClient returned nil")
	}
	if !c.SchemaCapable() {
		t.Error("SchemaCapable should be true for the Gemini client")
	}

	c.SetEnableGoogleSearch(true)
	if !c.IsGoogleSearchEnabled() {
		t.Error("Google search should be enabled after SetEnableGoogleSearch(true)")
	}
	c.SetEnableGoogleSearch(false)
	if c.IsGoogleSearchEnabled() {
		t.Error("Google search should be disabled after SetEnableGoogleSearch(false)")
	}

	c.SetEnableURLContext(true)
	if !c.IsURLContextEnabled() {
		t.Error("URL context should be enabled after SetEnableURLContext(true)")
	}
	c.SetURLContextURLs([]string{"https://docs.example.com"}) // must not panic

	c.SetModel("gemini-2.0-flash")
	if c.GetModel() != "gemini-2.0-flash" {
		t.Errorf("GetModel=%q, want gemini-2.0-flash", c.GetModel())
	}

	// Reasoning/grounding accessors are safe to read on a fresh client.
	_ = c.GetLastThoughtSignature()
	_ = c.GetLastThoughtSummary()
	_ = c.GetLastThinkingTokens()
	_ = c.GetThinkingLevel()
	_ = c.GetLastGroundingSources()
	_ = c.ShouldUsePiggybackTools()
}

func TestDefaultMaxOutputTokensForModel(t *testing.T) {
	if got := defaultMaxOutputTokensForModel("gemini-3-pro"); got != 65536 {
		t.Errorf("defaultMaxOutputTokensForModel(gemini-3)=%d, want 65536", got)
	}
	if got := defaultMaxOutputTokensForModel("gemini-2.0-flash"); got != 65536 {
		t.Errorf("defaultMaxOutputTokensForModel(gemini-2)=%d, want 65536", got)
	}
}
