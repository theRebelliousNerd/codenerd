package perception

import (
	"testing"

	"codenerd/internal/config"
)

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
	if _, ok := client.(*CodexCLIClient); !ok {
		t.Errorf("Expected *CodexCLIClient, got %T", client)
	}

	// 3. Invalid Engine
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
