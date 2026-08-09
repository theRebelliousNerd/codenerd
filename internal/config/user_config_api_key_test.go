package config

import "testing"

func TestSetAPIKeyForProvider(t *testing.T) {
	tests := []struct {
		provider string
		read     func(*UserConfig) string
	}{
		{"", func(c *UserConfig) string { return c.APIKey }},
		{"anthropic", func(c *UserConfig) string { return c.AnthropicAPIKey }},
		{" OPENAI ", func(c *UserConfig) string { return c.OpenAIAPIKey }},
		{"gemini", func(c *UserConfig) string { return c.GeminiAPIKey }},
		{"xai", func(c *UserConfig) string { return c.XAIAPIKey }},
		{"zai", func(c *UserConfig) string { return c.ZAIAPIKey }},
		{"openrouter", func(c *UserConfig) string { return c.OpenRouterAPIKey }},
		{"dashscope", func(c *UserConfig) string { return c.DashScopeAPIKey }},
		{"meta", func(c *UserConfig) string { return c.MetaAPIKey }},
		{"moonshot", func(c *UserConfig) string { return c.MoonshotAPIKey }},
	}
	for _, tt := range tests {
		t.Run(tt.provider, func(t *testing.T) {
			cfg := &UserConfig{}
			if err := cfg.SetAPIKeyForProvider(tt.provider, "new-key"); err != nil {
				t.Fatal(err)
			}
			if got := tt.read(cfg); got != "new-key" {
				t.Fatalf("configured key = %q, want new-key", got)
			}
		})
	}
}

func TestSetAPIKeyForProviderRejectsInvalidTargets(t *testing.T) {
	for _, provider := range []string{"ollama", "unknown"} {
		cfg := &UserConfig{APIKey: "unchanged"}
		if err := cfg.SetAPIKeyForProvider(provider, "new-key"); err == nil {
			t.Fatalf("SetAPIKeyForProvider(%q) succeeded", provider)
		}
		if cfg.APIKey != "unchanged" {
			t.Fatalf("SetAPIKeyForProvider(%q) mutated config", provider)
		}
	}

	var cfg *UserConfig
	if err := cfg.SetAPIKeyForProvider("openai", "new-key"); err == nil {
		t.Fatal("nil config accepted API key")
	}
}
