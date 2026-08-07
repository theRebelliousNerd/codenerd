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
	case "dashscope":
		return "dashscope_api_key (or DASHSCOPE_API_KEY)"
	case "meta":
		return "meta_api_key (or META_API_KEY / MODEL_API_KEY)"
	case "moonshot":
		return "moonshot_api_key (or MOONSHOT_API_KEY)"
	case "ollama":
		return "ollama (local — no API key; set ollama.model / ollama.endpoint)"
	default:
		return provider + " api key"
	}
}

// ProviderConfig holds the resolved provider and API key.
// classificationMaxOutputTokens bounds the perception transducer's reply.
// Classification returns a short intent label, so this is deliberately small;
// it is raised to the vendor's reasoning floor where one exists.
const classificationMaxOutputTokens = 2048

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

	// BaseURL overrides the endpoint for OpenAI-compatible providers
	// (dashscope, meta, moonshot). Each has a sensible built-in default, so this
	// is only needed for a proxy, a regional endpoint, or a vendor codeNERD does
	// not yet know by name.
	BaseURL string

	// Provider-specific configurations
	Gemini *config.GeminiProviderConfig // Gemini thinking mode and built-in tools
	Ollama *config.OllamaLLMConfig      // Local Ollama chat endpoint + model

	// Worker is optional secondary LLM (e.g. ollama gemma for shards).
	Worker *config.WorkerLLMConfig

	// MaxOutputTokens caps completion length for OpenAI-compatible vendors.
	// Zero leaves the client default. Carried here so a per-slot budget from
	// SecondaryLLMConfig survives the trip through the shared factory.
	MaxOutputTokens int
}

// LoadConfigJSON loads provider configuration from a JSON config file.
// This now delegates to the unified config.LoadUserConfig().
func LoadConfigJSON(path string) (*ProviderConfig, error) {
	userCfg, err := config.LoadUserConfig(path)
	if err != nil {
		return nil, err
	}
	return ProviderConfigFromUserConfig(userCfg)
}

// ProviderConfigFromUserConfig resolves the provider contract from an already
// validated UserConfig. Boot uses this path so it cannot parse one config for
// scheduling and a different config for the LLM client.
func ProviderConfigFromUserConfig(userCfg *config.UserConfig) (*ProviderConfig, error) {
	if userCfg == nil {
		return nil, fmt.Errorf("user config is nil")
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
		BaseURL:             userCfg.BaseURL,
		ClassificationModel: userCfg.ClassificationModel,
		Context7APIKey:      context7Key,
		Gemini:              userCfg.GetGeminiConfig(),
		Ollama:              &ollamaCfg,
		Worker:              userCfg.GetWorkerLLMConfig(),
		MaxOutputTokens:     userCfg.MaxOutputTokens,
	}, nil
}

// DetectProvider checks .nerd/config.json first, then environment variables.
// Priority: config.json > env vars (ANTHROPIC > OPENAI > GEMINI > XAI > ZAI)
// CLI/OAuth engines (claude-cli, codex-cli, xai-oauth) are detected from config.json
// and don't require API keys.
func DetectProvider() (*ProviderConfig, error) {
	logging.PerceptionDebug("DetectProvider: checking config and environment")

	// First, try to load from .nerd/config.json. A present-invalid config or an
	// explicit but unusable provider choice is terminal: ambient keys must not
	// silently select a different backend.
	configPath := config.DefaultUserConfigPath()
	userCfg, loadErr := config.LoadUserConfig(configPath)
	if loadErr != nil {
		return nil, fmt.Errorf("load explicit provider config: %w", loadErr)
	}
	if cfg, err := ProviderConfigFromUserConfig(userCfg); err == nil {
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
	} else if userCfg.HasExplicitLLMSelection() {
		return nil, err
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
		{"DASHSCOPE_API_KEY", ProviderDashScope},
		{"META_API_KEY", ProviderMeta},
		// Meta's own docs export the key as MODEL_API_KEY; accept it as an alias
		// so a shell already set up for their SDK works without extra steps.
		{"MODEL_API_KEY", ProviderMeta},
		{"MOONSHOT_API_KEY", ProviderMoonshot},
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
	return nil, fmt.Errorf("no API key found; configure .nerd/config.json or set one of: ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, XAI_API_KEY, ZAI_API_KEY, OPENROUTER_API_KEY, DASHSCOPE_API_KEY, META_API_KEY, MOONSHOT_API_KEY")
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

	case ProviderDashScope, ProviderMeta, ProviderMoonshot:
		// Classification runs on every interactive turn, so reasoning is turned
		// off here regardless of the tier's normal setting — a thinking trace in
		// front of every prompt is pure latency for a labelling task.
		compatCfg := DefaultOpenAICompatConfig(cfg.Provider, cfg.APIKey)
		compatCfg.EnableThinking = false
		// A classification reply is a short label, so a small ceiling is right —
		// and a ceiling is a cap, not a spend, so keeping it tight costs nothing
		// either way. It must still clear the vendor's reasoning floor: below
		// that, a reasoning model burns the whole budget thinking and returns an
		// EMPTY body with finish_reason "stop". A flat 2048 sat under Meta's
		// 4096 floor, so every boot logged a clamp warning twice and the value
		// never applied as written.
		compatCfg.MaxOutputTokens = classificationMaxOutputTokens
		if floor := minCompletionTokensFor(cfg.Provider); floor > compatCfg.MaxOutputTokens {
			compatCfg.MaxOutputTokens = floor
		}
		if model != "" {
			compatCfg.Model = model
		}
		if cfg.BaseURL != "" {
			compatCfg.BaseURL = cfg.BaseURL
		}
		client, err := NewOpenAICompatClient(compatCfg)
		if err != nil {
			// Classification is optional: fall back to the main client rather
			// than failing boot.
			logging.Get(logging.CategoryPerception).Warn("Classification client unavailable for %s: %v", cfg.Provider, err)
			return nil, nil
		}
		logging.Get(logging.CategoryPerception).Debug("Classification client: provider=%s model=%s (configured=%v)", cfg.Provider, compatCfg.Model, model != "")
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
	if config == nil {
		return nil, fmt.Errorf("provider config is nil")
	}

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

	case ProviderDashScope, ProviderMeta, ProviderMoonshot:
		compatCfg := DefaultOpenAICompatConfig(config.Provider, config.APIKey)
		if config.Model != "" {
			compatCfg.Model = config.Model
		}
		if config.BaseURL != "" {
			compatCfg.BaseURL = config.BaseURL
		}
		if config.MaxOutputTokens > 0 {
			compatCfg.MaxOutputTokens = config.MaxOutputTokens
		}
		return NewOpenAICompatClient(compatCfg)

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
	return newSecondarySlotClient(userCfg, "worker", userCfg.GetWorkerLLMConfig())
}

// NewPlannerClientFromUserConfig builds the high-reasoning planner client used
// for planning and analysis intents. Returns (nil, nil) when no planner block
// is configured so callers fall back to the worker, then the main client.
func NewPlannerClientFromUserConfig(userCfg *config.UserConfig) (LLMClient, error) {
	if userCfg == nil {
		return nil, nil
	}
	return newSecondarySlotClient(userCfg, "planner", userCfg.GetPlannerLLMConfig())
}

// newSecondarySlotClient constructs the client for one non-main slot. slot is
// the human-readable slot name used in log lines and error messages.
func newSecondarySlotClient(userCfg *config.UserConfig, slot string, w *config.SecondaryLLMConfig) (LLMClient, error) {
	if w == nil {
		return nil, nil
	}

	provider := strings.ToLower(strings.TrimSpace(w.Provider))

	// Ollama is local and keyless, and its config type is package-local, so it
	// keeps its direct construction path.
	if provider == "ollama" {
		cfg := DefaultOllamaLLMConfig()
		if w.Endpoint != "" {
			cfg.Endpoint = w.Endpoint
		} else if main := userCfg.GetOllamaLLMConfig(); main.Endpoint != "" {
			cfg.Endpoint = main.Endpoint
		}
		if w.Model != "" {
			cfg.Model = w.Model
		}
		logging.Perception("%s LLM: ollama model=%s endpoint=%s", slot, cfg.Model, cfg.Endpoint)
		return NewOllamaClientWithConfig(cfg), nil
	}

	// Everything else delegates to the shared factory rather than
	// re-implementing provider construction. Previously this function hand-rolled
	// four providers and rejected the rest, which silently made anthropic, zai,
	// openrouter and every OpenAI-compatible vendor unusable as the worker — the
	// exact slot a cheap bulk model belongs in. Delegating means the worker
	// supports whatever the main client supports, now and for future providers.
	apiKey := userCfg.APIKeyForProvider(provider)
	if apiKey == "" {
		return nil, fmt.Errorf("%s provider=%s but %s is empty", slot, provider, providerKeyFieldName(provider))
	}

	pc := &ProviderConfig{
		Engine:   "api",
		Provider: Provider(provider),
		APIKey:   apiKey,
		Model:    w.Model,
		BaseURL:  userCfg.BaseURL,
		Gemini:   userCfg.GetGeminiConfig(),
	}
	// For OpenAI-compatible vendors the slot's Endpoint doubles as a per-slot
	// base-URL override, so slots can sit behind different gateways.
	if w.Endpoint != "" {
		pc.BaseURL = w.Endpoint
	}
	// A per-slot completion ceiling: the planner usually wants a much larger
	// budget than a bulk worker, and before this the whole tier was pinned to
	// the client's hardcoded default.
	pc.MaxOutputTokens = w.MaxOutputTokens

	client, err := NewClientFromConfig(pc)
	if err != nil {
		return nil, fmt.Errorf("%s provider=%s: %w", slot, provider, err)
	}
	logging.Perception("%s LLM: provider=%s model=%s", slot, provider, w.Model)
	return client, nil
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
