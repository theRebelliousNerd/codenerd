package perception

import (
	"codenerd/internal/config"
	"fmt"
	"os"
)

// ProviderConfig holds the resolved provider and API key.
type ProviderConfig struct {
	Provider       Provider
	APIKey         string
	Model          string // Optional model override
	Context7APIKey string // Context7 API key for research

	// CLI Engine Configuration (takes precedence over Provider when set)
	Engine    string                  // "api", "claude-cli", "codex-cli"
	ClaudeCLI *config.ClaudeCLIConfig // Claude CLI settings
	CodexCLI  *config.CodexCLIConfig  // Codex CLI settings
}

// DefaultConfigPath returns the default path to .nerd/config.json.
// Deprecated: Use config.DefaultUserConfigPath() instead.
func DefaultConfigPath() string {
	return config.DefaultUserConfigPath()
}

// LoadConfigJSON loads provider configuration from a JSON config file.
// This now delegates to the unified config.LoadUserConfig().
func LoadConfigJSON(path string) (*ProviderConfig, error) {
	userCfg, err := config.LoadUserConfig(path)
	if err != nil {
		return nil, err
	}

	// Check for CLI engine configuration first
	engine := userCfg.GetEngine()
	if engine == "claude-cli" || engine == "codex-cli" {
		// Context7 API key: check config first, then env var
		context7Key := userCfg.Context7APIKey
		if context7Key == "" {
			context7Key = os.Getenv("CONTEXT7_API_KEY")
		}

		return &ProviderConfig{
			Engine:         engine,
			ClaudeCLI:      userCfg.GetClaudeCLIConfig(),
			CodexCLI:       userCfg.GetCodexCLIConfig(),
			Context7APIKey: context7Key,
		}, nil
	}

	// Use the unified config's provider detection for API mode
	providerStr, apiKey := userCfg.GetActiveProvider()
	if apiKey == "" {
		return nil, fmt.Errorf("no API key found in config")
	}

	// Context7 API key: check config first, then env var
	context7Key := userCfg.Context7APIKey
	if context7Key == "" {
		context7Key = os.Getenv("CONTEXT7_API_KEY")
	}

	return &ProviderConfig{
		Engine:         "api",
		Provider:       Provider(providerStr),
		APIKey:         apiKey,
		Model:          userCfg.Model,
		Context7APIKey: context7Key,
	}, nil
}

// DetectProvider checks .nerd/config.json first, then environment variables.
// Priority: config.json > env vars (ANTHROPIC > OPENAI > GEMINI > XAI > ZAI)
// CLI engines (claude-cli, codex-cli) are detected from config.json and don't require API keys.
func DetectProvider() (*ProviderConfig, error) {
	// First, try to load from .nerd/config.json
	configPath := DefaultConfigPath()
	if cfg, err := LoadConfigJSON(configPath); err == nil {
		// CLI engines don't need API keys (subscription-based)
		if cfg.Engine == "claude-cli" || cfg.Engine == "codex-cli" {
			return cfg, nil
		}
		// API mode requires an API key
		if cfg.APIKey != "" {
			return cfg, nil
		}
	}

	// Fall back to environment variables
	providers := []struct {
		envVar   string
		provider Provider
	}{
		{"ANTHROPIC_API_KEY", ProviderAnthropic},
		{"OPENAI_API_KEY", ProviderOpenAI},
		{"GEMINI_API_KEY", ProviderGemini},
		{"XAI_API_KEY", ProviderXAI},
		{"ZAI_API_KEY", ProviderZAI},
		{"OPENROUTER_API_KEY", ProviderOpenRouter},
	}

	for _, p := range providers {
		if key := os.Getenv(p.envVar); key != "" {
			return &ProviderConfig{
				Provider: p.provider,
				APIKey:   key,
			}, nil
		}
	}

	return nil, fmt.Errorf("no API key found; configure .nerd/config.json or set one of: ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, XAI_API_KEY, ZAI_API_KEY")
}

// NewClientFromEnv creates an LLM client based on config file or environment variables.
func NewClientFromEnv() (LLMClient, error) {
	config, err := DetectProvider()
	if err != nil {
		return nil, err
	}
	return NewClientFromConfig(config)
}

// NewClientFromConfig creates an LLM client from a provider config.
// CLI engines (claude-cli, codex-cli) take precedence over API providers when configured.
func NewClientFromConfig(config *ProviderConfig) (LLMClient, error) {
	// Check for CLI engine configuration first (takes precedence over API)
	switch config.Engine {
	case "claude-cli":
		return NewClaudeCodeCLIClient(config.ClaudeCLI), nil
	case "codex-cli":
		return NewCodexCLIClient(config.CodexCLI), nil
	case "api", "":
		// Continue to API-based provider selection below
	default:
		return nil, fmt.Errorf("unknown engine: %s (valid: api, claude-cli, codex-cli)", config.Engine)
	}

	// API-based provider selection
	switch config.Provider {
	case ProviderAnthropic:
		client := NewAnthropicClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderOpenAI:
		client := NewOpenAIClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderGemini:
		client := NewGeminiClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderXAI:
		client := NewXAIClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderZAI:
		client := NewZAIClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderOpenRouter:
		client := NewOpenRouterClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", config.Provider)
	}
}
