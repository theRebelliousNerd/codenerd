package perception

import (
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/perception/xaioauth"
	"fmt"
	"os"
	"strings"
)

// providerKeyFieldName returns the name of the API key config field/env var
// associated with a provider, used for clearer error messages when an
// explicit provider is set but its key is missing.
func providerKeyFieldName(provider string) string {
	switch provider {
	case "anthropic":
		return "anthropic_api_key (or ANTHROPIC_API_KEY)"
	case "openai":
		return "openai_api_key (or OPENAI_API_KEY)"
	case "gemini":
		return "gemini_api_key (or GEMINI_API_KEY)"
	case "xai":
		return "xai_api_key (or XAI_API_KEY)"
	case "zai":
		return "zai_api_key (or ZAI_API_KEY)"
	case "openrouter":
		return "openrouter_api_key (or OPENROUTER_API_KEY)"
	case "ollama":
		return "ollama (local — no API key; set ollama.model / ollama.endpoint)"
	default:
		return provider + " api key"
	}
}

// ProviderConfig holds the resolved provider and API key.
type ProviderConfig struct {
	Provider       Provider
	APIKey         string
	Model          string // Optional model override
	Context7APIKey string // Context7 API key for research

	// ClassificationModel is the dedicated fast model for perception intent
	// classification. Unlike Model, this never defaults to the main model:
	// classification is on the critical path of every turn and must stay cheap.
	ClassificationModel string

	// CLI Engine Configuration (takes precedence over Provider when set)
	Engine    string                  // "api", "claude-cli", "codex-cli", "xai-oauth"
	ClaudeCLI *config.ClaudeCLIConfig // Claude CLI settings
	CodexCLI  *config.CodexCLIConfig  // Codex CLI settings
	XAIOAuth  *config.XAIOAuthConfig  // SuperGrok OAuth settings

	// Provider-specific configurations
	Gemini *config.GeminiProviderConfig // Gemini thinking mode and built-in tools
	Ollama *config.OllamaLLMConfig      // Local Ollama chat endpoint + model

	// Worker is optional secondary LLM (e.g. ollama gemma for shards).
	Worker *config.WorkerLLMConfig
}

// LoadConfigJSON loads provider configuration from a JSON config file.
// This now delegates to the unified config.LoadUserConfig().
func LoadConfigJSON(path string) (*ProviderConfig, error) {
	userCfg, err := config.LoadUserConfig(path)
	if err != nil {
		return nil, err
	}

	// Check for CLI / OAuth engine configuration first
	engine := userCfg.GetEngine()
	if engine == "claude-cli" || engine == "codex-cli" || engine == "xai-oauth" {
		// Context7 API key: check config first, then env var
		context7Key := userCfg.Context7APIKey
		if context7Key == "" {
			context7Key = os.Getenv("CONTEXT7_API_KEY")
		}

		pc := &ProviderConfig{
			Engine:         engine,
			ClaudeCLI:      userCfg.GetClaudeCLIConfig(),
			CodexCLI:       userCfg.GetCodexCLIConfig(),
			XAIOAuth:       userCfg.GetXAIOAuthConfig(),
			Context7APIKey: context7Key,
			Model:          userCfg.Model,
		}
		// SuperGrok hard-auth fallback needs the metered xAI key available.
		if engine == "xai-oauth" {
			xaiKey := userCfg.XAIAPIKey
			if xaiKey == "" {
				xaiKey = os.Getenv("XAI_API_KEY")
			}
			pc.APIKey = xaiKey
			pc.Provider = ProviderXAI
		}
		return pc, nil
	}

	// Use the unified config's provider detection for API mode
	providerStr, apiKey := userCfg.GetActiveProvider()
	if apiKey == "" {
		if userCfg.Provider != "" {
			return nil, fmt.Errorf("provider %q is configured but its API key is missing; set the %s key or change the provider (config is boss: no silent fallback)", userCfg.Provider, providerKeyFieldName(userCfg.Provider))
		}
		return nil, fmt.Errorf("no API key found in config")
	}

	// Context7 API key: check config first, then env var
	context7Key := userCfg.Context7APIKey
	if context7Key == "" {
		context7Key = os.Getenv("CONTEXT7_API_KEY")
	}

	ollamaCfg := userCfg.GetOllamaLLMConfig()
	return &ProviderConfig{
		Engine:              "api",
		Provider:            Provider(providerStr),
		APIKey:              apiKey,
		Model:               userCfg.Model,
		ClassificationModel: userCfg.ClassificationModel,
		Context7APIKey:      context7Key,
		Gemini:              userCfg.GetGeminiConfig(),
		Ollama:              &ollamaCfg,
		Worker:              userCfg.GetWorkerLLMConfig(),
	}, nil
}

// DetectProvider checks .nerd/config.json first, then environment variables.
// Priority: config.json > env vars (ANTHROPIC > OPENAI > GEMINI > XAI > ZAI)
// CLI/OAuth engines (claude-cli, codex-cli, xai-oauth) are detected from config.json
// and don't require API keys.
func DetectProvider() (*ProviderConfig, error) {
	logging.PerceptionDebug("DetectProvider: checking config and environment")

	// First, try to load from .nerd/config.json
	configPath := config.DefaultUserConfigPath()
	if cfg, err := LoadConfigJSON(configPath); err == nil {
		// Subscription engines don't need API keys
		if cfg.Engine == "claude-cli" || cfg.Engine == "codex-cli" || cfg.Engine == "xai-oauth" {
			logging.Perception("DetectProvider: using subscription engine=%s", cfg.Engine)
			return cfg, nil
		}
		// API mode requires an API key
		if cfg.APIKey != "" {
			logging.Perception("DetectProvider: using provider=%s from config", cfg.Provider)
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
			logging.Perception("DetectProvider: using provider=%s from env var %s", p.provider, p.envVar)
			return &ProviderConfig{
				Provider: p.provider,
				APIKey:   key,
			}, nil
		}
	}

	logging.PerceptionError("DetectProvider: no API key found in config or environment")
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

// NERD-EVOLVE-START: P1P2-model-tiering
// NewClassificationClientFromConfig creates an LLM client for intent
// classification (P2 model tiering). The model is determined by:
//  1. cfg.ClassificationModel — explicit user override from config.json
//  2. Per-provider fast-tier defaults:
//     - Anthropic: claude-haiku-4-5 with prompt caching enabled (P1+P2)
//     - Gemini: gemini-3.1-flash-lite
//     - OpenAI: gpt-4o-mini
//  3. Providers without a known fast tier (zai, xai, openrouter): returns nil
//     unless ClassificationModel is set (caller falls back to main LLMClient).
//
// The main cfg.Model setting deliberately does NOT apply here: classification
// runs on every interactive turn before anything else can happen, so routing
// it to the user's (typically large, slow) main model put minutes of latency
// in front of every prompt. That was the old behavior and it was a bug.
//
// When nil is returned, no error is set — the caller should treat nil as
// "use main client" and not fail.
func NewClassificationClientFromConfig(cfg *ProviderConfig) (LLMClient, error) {
	if cfg == nil {
		return nil, nil
	}

	// Subscription engines do not support model tiering — return nil to use main client.
	if cfg.Engine == "claude-cli" || cfg.Engine == "codex-cli" || cfg.Engine == "xai-oauth" {
		return nil, nil
	}

	model := cfg.ClassificationModel

	switch cfg.Provider {
	case ProviderAnthropic:
		haikuCfg := DefaultAnthropicConfig(cfg.APIKey)
		haikuCfg.Model = "claude-haiku-4-5"
		if model != "" {
			haikuCfg.Model = model
		}
		client := NewAnthropicClientWithConfig(haikuCfg)
		client.EnableSystemCaching() // P1: cache the static perception system prompt
		logging.Get(logging.CategoryPerception).Debug("Classification client: provider=anthropic model=%s (configured=%v)", haikuCfg.Model, model != "")
		return client, nil

	case ProviderGemini:
		flashCfg := DefaultGeminiConfig(cfg.APIKey)
		flashCfg.Model = "gemini-3.1-flash-lite"
		if model != "" {
			flashCfg.Model = model
		}
		logging.Get(logging.CategoryPerception).Debug("Classification client: provider=gemini model=%s (configured=%v)", flashCfg.Model, model != "")
		return NewGeminiClientWithConfig(flashCfg), nil

	case ProviderOpenAI:
		client := NewOpenAIClient(cfg.APIKey)
		client.SetModel("gpt-4o-mini")
		if model != "" {
			client.SetModel(model)
		}
		logging.Get(logging.CategoryPerception).Debug("Classification client: provider=openai model=%s (configured=%v)", model, model != "")
		return client, nil

	case ProviderZAI:
		// Z.AI has no universally-available fast tier we can assume; honor an
		// explicit classification_model only.
		if model == "" {
			return nil, nil
		}
		client := NewZAIClient(cfg.APIKey)
		client.SetModel(model)
		logging.Get(logging.CategoryPerception).Debug("Classification client: provider=zai model=%s", model)
		return client, nil

	case ProviderXAI:
		if model == "" {
			return nil, nil
		}
		client := NewXAIClient(cfg.APIKey)
		client.SetModel(model)
		logging.Get(logging.CategoryPerception).Debug("Classification client: provider=xai model=%s", model)
		return client, nil

	case ProviderOpenRouter:
		if model == "" {
			return nil, nil
		}
		client := NewOpenRouterClient(cfg.APIKey)
		client.SetModel(model)
		logging.Get(logging.CategoryPerception).Debug("Classification client: provider=openrouter model=%s", model)
		return client, nil

	default:
		return nil, nil
	}
}

// NERD-EVOLVE-END: P1P2-model-tiering

// newSuperGrokClientOrAPIFallback builds the SuperGrok OAuth client. If tokens are
// quarantined/missing and an xAI API key is available (and fallback_to_api_key is
// enabled, default true), falls back to metered API with a loud WARNING so CLI
// sessions stay usable after refresh-token revoke. Prefer re-import (handled in
// TokenSource.Load) over stuck quarantined tokens before this path runs.
func newSuperGrokClientOrAPIFallback(config *ProviderConfig) (LLMClient, error) {
	oauthClient := xaioauth.NewClientFromUserConfig(config.XAIOAuth)
	if err := oauthClient.TokenSource().Load(); err != nil && xaioauth.IsAuthRequired(err) {
		if xaiOAuthFallbackEnabled(config) {
			if key := resolveXAIAPIKey(config); key != "" {
				logging.PerceptionWarn(
					"WARNING: engine=xai-oauth hard auth failure — falling back to xAI API key (metered). "+
						"OAuth detail: %v. Re-auth SuperGrok: nerd auth grok. "+
						"Disable fallback: set xai_oauth.fallback_to_api_key=false",
					err,
				)
				xai := NewXAIClient(key)
				model := ""
				if config.XAIOAuth != nil && config.XAIOAuth.Model != "" {
					model = config.XAIOAuth.Model
				} else if config.Model != "" {
					model = config.Model
				}
				if model != "" {
					xai.SetModel(model)
				}
				return xai, nil
			}
		}
		logging.PerceptionWarn(
			"WARNING: SuperGrok OAuth not ready: %v — run: nerd auth grok "+
				"(or set xai_api_key + fallback_to_api_key for metered API fallback)",
			err,
		)
	}
	return oauthClient, nil
}

// xaiOAuthFallbackEnabled reports whether hard-auth API key fallback is allowed.
// Default true when unset (CLI reliability).
func xaiOAuthFallbackEnabled(pc *ProviderConfig) bool {
	if pc == nil || pc.XAIOAuth == nil || pc.XAIOAuth.FallbackToAPIKey == nil {
		return true
	}
	return *pc.XAIOAuth.FallbackToAPIKey
}

func resolveXAIAPIKey(pc *ProviderConfig) string {
	if pc != nil {
		if k := strings.TrimSpace(pc.APIKey); k != "" {
			return k
		}
	}
	if k := strings.TrimSpace(os.Getenv("XAI_API_KEY")); k != "" {
		return k
	}
	// Load user config for root xai_api_key when engine is xai-oauth.
	path := config.DefaultUserConfigPath()
	if cfg, err := config.LoadUserConfig(path); err == nil && cfg != nil {
		return strings.TrimSpace(cfg.XAIAPIKey)
	}
	return ""
}

// NewClientFromConfig creates an LLM client from a provider config.
// Subscription engines (claude-cli, codex-cli, xai-oauth) take precedence over API providers.
func NewClientFromConfig(config *ProviderConfig) (LLMClient, error) {
	// Check for CLI/OAuth engine configuration first (takes precedence over API)
	switch config.Engine {
	case "claude-cli":
		return NewClaudeCodeCLIClient(config.ClaudeCLI), nil
	case "codex-cli":
		return NewCodexExecClient(config.CodexCLI), nil
	case "xai-oauth":
		return newSuperGrokClientOrAPIFallback(config)
	case "api", "":
		// Continue to API-based provider selection below
	default:
		return nil, fmt.Errorf("unknown engine: %s (valid: api, claude-cli, codex-cli, xai-oauth)", config.Engine)
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
		// Build GeminiConfig with user settings
		geminiCfg := DefaultGeminiConfig(config.APIKey)
		if config.Model != "" {
			geminiCfg.Model = config.Model
		}
		// Apply user's Gemini provider config (thinking, grounding tools)
		if config.Gemini != nil {
			geminiCfg.EnableThinking = config.Gemini.EnableThinking
			geminiCfg.ThinkingLevel = config.Gemini.ThinkingLevel
			geminiCfg.EnableGoogleSearch = config.Gemini.EnableGoogleSearch
			geminiCfg.EnableURLContext = config.Gemini.EnableURLContext
			// Respect explicit MaxOutputTokens override. With thinking
			// mode, the budget covers BOTH thinking and visible output;
			// truncation in the chat surface usually means thinking ate
			// too much. The default in DefaultGeminiConfig is already the
			// Gemini-3 cap (65536), but users can lower this for cost or
			// keep it pinned at the cap.
			if config.Gemini.MaxOutputTokens > 0 {
				geminiCfg.MaxOutputTokens = config.Gemini.MaxOutputTokens
			}
		}
		return NewGeminiClientWithConfig(geminiCfg), nil

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

	case ProviderOllama:
		ollamaCfg := DefaultOllamaLLMConfig()
		if config.Ollama != nil {
			if config.Ollama.Endpoint != "" {
				ollamaCfg.Endpoint = config.Ollama.Endpoint
			}
			if config.Ollama.Model != "" {
				ollamaCfg.Model = config.Ollama.Model
			}
		}
		if config.Model != "" {
			ollamaCfg.Model = config.Model
		}
		return NewOllamaClientWithConfig(ollamaCfg), nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", config.Provider)
	}
}

// NewWorkerClientFromUserConfig builds the secondary worker LLM client used for
// shards / spawn / create / cheap classification. Returns (nil, nil) when no
// worker block is configured so callers can fall back to the main client.
func NewWorkerClientFromUserConfig(userCfg *config.UserConfig) (LLMClient, error) {
	if userCfg == nil {
		return nil, nil
	}
	w := userCfg.GetWorkerLLMConfig()
	if w == nil {
		return nil, nil
	}
	switch strings.ToLower(w.Provider) {
	case "ollama":
		cfg := DefaultOllamaLLMConfig()
		if w.Endpoint != "" {
			cfg.Endpoint = w.Endpoint
		} else {
			ollama := userCfg.GetOllamaLLMConfig()
			cfg.Endpoint = ollama.Endpoint
		}
		if w.Model != "" {
			cfg.Model = w.Model
		}
		client := NewOllamaClientWithConfig(cfg)
		logging.Perception("Worker LLM: ollama model=%s endpoint=%s", cfg.Model, cfg.Endpoint)
		return client, nil
	case "xai":
		if userCfg.XAIAPIKey == "" {
			return nil, fmt.Errorf("worker provider=xai but xai_api_key is empty")
		}
		client := NewXAIClient(userCfg.XAIAPIKey)
		if w.Model != "" {
			client.SetModel(w.Model)
		}
		return client, nil
	case "openai":
		if userCfg.OpenAIAPIKey == "" {
			return nil, fmt.Errorf("worker provider=openai but openai_api_key is empty")
		}
		client := NewOpenAIClient(userCfg.OpenAIAPIKey)
		if w.Model != "" {
			client.SetModel(w.Model)
		}
		return client, nil
	case "gemini":
		if userCfg.GeminiAPIKey == "" {
			return nil, fmt.Errorf("worker provider=gemini but gemini_api_key is empty")
		}
		gcfg := DefaultGeminiConfig(userCfg.GeminiAPIKey)
		if w.Model != "" {
			gcfg.Model = w.Model
		}
		return NewGeminiClientWithConfig(gcfg), nil
	default:
		return nil, fmt.Errorf("unsupported worker provider %q (use ollama, xai, openai, gemini)", w.Provider)
	}
}

// NewImageClientFromUserConfig builds the dedicated image-generation client
// (Gemini Nano Banana 2 / gemini-3.1-flash-image). Never uses worker=ollama.
// Returns (nil, nil) only when gemini_api_key is missing (caller may warn).
func NewImageClientFromUserConfig(userCfg *config.UserConfig) (LLMClient, error) {
	if userCfg == nil {
		return nil, nil
	}
	img := userCfg.GetImageLLMConfig()
	if strings.ToLower(img.Provider) != "gemini" && img.Provider != "" {
		return nil, fmt.Errorf("image provider %q not supported (use gemini / Nano Banana 2)", img.Provider)
	}
	key := userCfg.GeminiAPIKey
	if key == "" {
		key = os.Getenv("GEMINI_API_KEY")
	}
	if key == "" {
		key = os.Getenv("GOOGLE_API_KEY")
	}
	if key == "" {
		return nil, fmt.Errorf("image generation needs gemini_api_key (Nano Banana 2 / gemini-3.1-flash-image)")
	}
	gcfg := DefaultGeminiConfig(key)
	gcfg.Model = img.Model
	if gcfg.Model == "" {
		gcfg.Model = config.DefaultImageModel
	}
	logging.Perception("Image LLM: gemini model=%s (Nano Banana 2 family)", gcfg.Model)
	return NewGeminiClientWithConfig(gcfg), nil
}
