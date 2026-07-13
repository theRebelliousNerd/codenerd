package perception

import (
	"strings"
	"testing"

	"codenerd/internal/config"
)

// TODO: TEST_GAP: [Null/Undefined/Empty] Missing tests for `providerKeyFieldName` when passing an empty string or undefined string.
// TODO: TEST_GAP: [Type Coercion] Testing behavior when string inputs simulate invalid or partially formed integer configurations implicitly mapped by upstream config loaders.
// TODO: TEST_GAP: [User Request Extremes] Testing massive strings for models or keys in `NewClientFromConfig` and massive strings in `providerKeyFieldName`.
// TODO: TEST_GAP: [State Conflicts] Testing concurrency behavior of reading env variables while they are modified by other tests/goroutines (though Go limits os.Setenv concurrency safety natively, the package logic still lacks assertions for TOC/TOU or env reset behavior).
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify fallback behavior when APIKey is an empty string for providers that require authentication.
// TODO: TEST_GAP: [Type Coercion] Verify handling of unexpected casing (e.g., "AnThroPic") or padding in the Provider string to ensure correct provider resolution.
// TODO: TEST_GAP: [State Conflicts] Verify thread safety of NewClientFromConfig if invoked concurrently with shared ProviderConfig pointers that might be mutated elsewhere.

func TestNewClientFromConfig_NilConfig(t *testing.T) {
	client, err := NewClientFromConfig(nil)
	if err == nil {
		t.Fatal("NewClientFromConfig(nil) error = nil; want explicit configuration error")
	}
	if client != nil {
		t.Fatalf("NewClientFromConfig(nil) client = %T; want nil", client)
	}
	if !strings.Contains(err.Error(), "provider config is nil") {
		t.Fatalf("NewClientFromConfig(nil) error = %q; want actionable nil-config message", err)
	}
}

func TestNewClientFromConfig_Engines(t *testing.T) {
	// 1. Claude CLI
	cfg := &ProviderConfig{
		Engine: "claude-cli",
		ClaudeCLI: &config.ClaudeCLIConfig{
			Model: "sonnet",
		},
	}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create claude-cli client: %v", err)
	}
	if _, ok := client.(*ClaudeCodeCLIClient); !ok {
		t.Errorf("Expected *ClaudeCodeCLIClient, got %T", client)
	}

	// 2. Codex CLI
	cfg = &ProviderConfig{
		Engine: "codex-cli",
		CodexCLI: &config.CodexCLIConfig{
			Model: "gpt-5",
		},
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create codex-cli client: %v", err)
	}
	if client == nil {
		t.Fatal("Expected non-nil codex-cli client")
	}

	// 3. SuperGrok OAuth
	importGrok := false
	cfg = &ProviderConfig{
		Engine: "xai-oauth",
		XAIOAuth: &config.XAIOAuthConfig{
			Model:          "grok-4.5",
			ImportGrokAuth: &importGrok,
		},
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create xai-oauth client: %v", err)
	}
	if client == nil {
		t.Fatal("Expected non-nil xai-oauth client")
	}

	// 4. Invalid Engine
	cfg = &ProviderConfig{
		Engine: "invalid-cli",
	}
	_, err = NewClientFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for invalid engine")
	}
}

func TestNewClientFromConfig_Providers(t *testing.T) {
	// 1. Anthropic
	cfg := &ProviderConfig{
		Provider: ProviderAnthropic,
		APIKey:   "sk-ant-test",
	}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create Anthropic client: %v", err)
	}
	if _, ok := client.(*AnthropicClient); !ok {
		t.Errorf("Expected *AnthropicClient, got %T", client)
	}

	// 2. OpenAI
	cfg = &ProviderConfig{
		Provider: ProviderOpenAI,
		APIKey:   "sk-openai-test",
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create OpenAI client: %v", err)
	}
	if _, ok := client.(*OpenAIClient); !ok {
		t.Errorf("Expected *OpenAIClient, got %T", client)
	}

	// 3. Gemini (with config)
	cfg = &ProviderConfig{
		Provider: ProviderGemini,
		APIKey:   "gemini-key",
		Gemini: &config.GeminiProviderConfig{
			EnableThinking: true,
			ThinkingLevel:  "high",
		},
	}
	client, err = NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("Failed to create Gemini client: %v", err)
	}
	if geminiClient, ok := client.(*GeminiClient); !ok {
		t.Errorf("Expected *GeminiClient, got %T", client)
	} else {
		// Verify config propagated using interface method
		if !geminiClient.IsThinkingEnabled() {
			t.Error("Gemini config EnableThinking not propagated")
		}
	}

	// 4. Unknown Provider
	cfg = &ProviderConfig{
		Provider: Provider("unknown"),
		APIKey:   "key",
	}
	_, err = NewClientFromConfig(cfg)
	if err == nil {
		t.Error("Expected error for unknown provider")
	}
}
