package perception

import (
	"strings"
	"testing"

	"codenerd/internal/config"
)

// The worker slot is where a cheap bulk model belongs. It previously accepted
// only ollama/xai/openai/gemini and hard-errored on everything else, which made
// the entire OpenAI-compatible family — plus anthropic, zai and openrouter —
// unusable as the worker. It now delegates to NewClientFromConfig, so it
// supports whatever the main client supports.
func TestNewWorkerClientFromUserConfig_SupportsAllProviders(t *testing.T) {
	cases := []struct {
		provider string
		model    string
		apply    func(*config.UserConfig)
	}{
		{"meta", "muse-spark-1.2-contributor", func(c *config.UserConfig) { c.MetaAPIKey = "k" }},
		{"dashscope", "qwen3.8-max", func(c *config.UserConfig) { c.DashScopeAPIKey = "k" }},
		{"moonshot", "kimi-k3", func(c *config.UserConfig) { c.MoonshotAPIKey = "k" }},
		{"anthropic", "claude-haiku-4-5", func(c *config.UserConfig) { c.AnthropicAPIKey = "k" }},
		{"openrouter", "moonshotai/kimi-k3", func(c *config.UserConfig) { c.OpenRouterAPIKey = "k" }},
		{"zai", "glm-4.7", func(c *config.UserConfig) { c.ZAIAPIKey = "k" }},
		{"openai", "gpt-4o-mini", func(c *config.UserConfig) { c.OpenAIAPIKey = "k" }},
		{"gemini", "gemini-3.1-flash-lite", func(c *config.UserConfig) { c.GeminiAPIKey = "k" }},
		{"xai", "grok-4.5", func(c *config.UserConfig) { c.XAIAPIKey = "k" }},
	}

	for _, tc := range cases {
		t.Run(tc.provider, func(t *testing.T) {
			cfg := &config.UserConfig{
				Worker: &config.WorkerLLMConfig{Provider: tc.provider, Model: tc.model},
			}
			tc.apply(cfg)

			client, err := NewWorkerClientFromUserConfig(cfg)
			if err != nil {
				t.Fatalf("worker provider %s rejected: %v", tc.provider, err)
			}
			if client == nil {
				t.Fatalf("worker provider %s returned a nil client", tc.provider)
			}
		})
	}
}

func TestNewWorkerClientFromUserConfig_MissingKeyFailsLoudly(t *testing.T) {
	cfg := &config.UserConfig{
		Worker: &config.WorkerLLMConfig{Provider: "meta", Model: "muse-spark-1.2-contributor"},
	}
	_, err := NewWorkerClientFromUserConfig(cfg)
	if err == nil {
		t.Fatal("expected an error when the worker provider's key is missing")
	}
	// The message must name the field to set, not just say "failed".
	if !strings.Contains(err.Error(), "meta_api_key") {
		t.Errorf("error should name the missing config field, got: %v", err)
	}
}

func TestNewWorkerClientFromUserConfig_NoWorkerReturnsNil(t *testing.T) {
	client, err := NewWorkerClientFromUserConfig(&config.UserConfig{})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Error("no worker block must yield a nil client so callers fall back to main")
	}
}

// The worker's endpoint doubles as a per-slot base-URL override, so main and
// worker can sit behind different gateways.
func TestNewWorkerClientFromUserConfig_EndpointOverridesBaseURL(t *testing.T) {
	cfg := &config.UserConfig{
		MetaAPIKey: "k",
		Worker: &config.WorkerLLMConfig{
			Provider: "meta",
			Model:    "muse-spark-1.2-contributor",
			Endpoint: "https://proxy.internal/v1",
		},
	}
	client, err := NewWorkerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatalf("NewWorkerClientFromUserConfig: %v", err)
	}
	compat, ok := client.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("expected *OpenAICompatClient, got %T", client)
	}
	if compat.baseURL != "https://proxy.internal/v1" {
		t.Errorf("baseURL = %q, want the worker endpoint override", compat.baseURL)
	}
}

// A provider is only usable end-to-end if config key resolution knows it too.
func TestAPIKeyResolution_CoversNewProviders(t *testing.T) {
	cfg := &config.UserConfig{
		DashScopeAPIKey: "ds",
		MetaAPIKey:      "mt",
		MoonshotAPIKey:  "ms",
	}
	for provider, want := range map[string]string{
		"dashscope": "ds",
		"meta":      "mt",
		"moonshot":  "ms",
	} {
		if got := cfg.APIKeyForProvider(provider); got != want {
			t.Errorf("APIKeyForProvider(%s) = %q, want %q", provider, got, want)
		}
	}

	// A config carrying only a new-provider key must still count as an explicit
	// LLM selection; otherwise boot silently falls through to an ambient key for
	// a different provider.
	if !(&config.UserConfig{DashScopeAPIKey: "ds"}).HasExplicitLLMSelection() {
		t.Error("a dashscope-only config must report an explicit LLM selection")
	}
	if !(&config.UserConfig{MetaAPIKey: "mt"}).HasExplicitLLMSelection() {
		t.Error("a meta-only config must report an explicit LLM selection")
	}

	// Config-is-boss: an explicit provider resolves only its own key.
	explicit := &config.UserConfig{Provider: "dashscope", DashScopeAPIKey: "ds", MetaAPIKey: "mt"}
	gotProvider, gotKey := explicit.GetActiveProvider()
	if gotProvider != "dashscope" || gotKey != "ds" {
		t.Errorf("GetActiveProvider() = (%q, %q), want (dashscope, ds)", gotProvider, gotKey)
	}
}
