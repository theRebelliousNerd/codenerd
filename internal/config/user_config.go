package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// UserConfig holds ALL codeNERD configuration from .nerd/config.json.
// This is the single source of truth for configuration.
//
// Supported models by provider:
//   - anthropic:   claude-sonnet-4-5-20250514, claude-opus-4-20250514, claude-3-5-sonnet-20241022
//   - openai:      gpt-5.1-codex-max (default), gpt-5.1-codex-mini, gpt-5-codex, gpt-4o
//   - gemini:      gemini-3-flash-preview (default), gemini-3-pro-preview, gemini-2.5-pro, gemini-2.5-flash
//   - xai:         grok-2-latest (default), grok-2, grok-beta
//   - zai:         GLM-4.6 (default)
//   - openrouter:  anthropic/claude-3.5-sonnet, openai/gpt-4o, google/gemini-pro, etc.
type UserConfig struct {
	// =========================================================================
	// LLM PROVIDER CONFIGURATION
	// =========================================================================

	// Provider selection (anthropic, openai, gemini, xai, zai, openrouter)
	Provider string `json:"provider,omitempty"`

	// API keys for each provider
	APIKey           string `json:"api_key,omitempty"`            // Legacy: single key
	AnthropicAPIKey  string `json:"anthropic_api_key,omitempty"`  // Anthropic/Claude
	OpenAIAPIKey     string `json:"openai_api_key,omitempty"`     // OpenAI/Codex
	GeminiAPIKey     string `json:"gemini_api_key,omitempty"`     // Google Gemini
	XAIAPIKey        string `json:"xai_api_key,omitempty"`        // xAI/Grok
	ZAIAPIKey        string `json:"zai_api_key,omitempty"`        // Z.AI
	OpenRouterAPIKey string `json:"openrouter_api_key,omitempty"` // OpenRouter (multi-provider)

	// Optional model override (see supported models above)
	Model string `json:"model,omitempty"`

	// =========================================================================
	// CLI ENGINE CONFIGURATION
	// =========================================================================

	// Engine selection: "api" (default), "claude-cli", "codex-cli"
	// When set to "claude-cli" or "codex-cli", uses CLI subprocess instead of HTTP API
	Engine string `json:"engine,omitempty"`

	// Claude Code CLI configuration (used when Engine="claude-cli")
	ClaudeCLI *ClaudeCLIConfig `json:"claude_cli,omitempty"`

	// Codex CLI configuration (used when Engine="codex-cli")
	CodexCLI *CodexCLIConfig `json:"codex_cli,omitempty"`

	// =========================================================================
	// UI SETTINGS
	// =========================================================================

	// Theme for the TUI ("light" or "dark")
	Theme string `json:"theme,omitempty"`

	// ContinuationMode controls multi-step task execution behavior
	// 0 = Auto (fully automatic), 1 = Confirm (pause after each step), 2 = Breakpoint (pause before mutations)
	ContinuationMode int `json:"continuation_mode,omitempty"`

	// =========================================================================
	// EXTERNAL SERVICE KEYS
	// =========================================================================

	// Context7 API key for research shards
	Context7APIKey string `json:"context7_api_key,omitempty"`

	// =========================================================================
	// CONTEXT & MEMORY CONFIGURATION
	// =========================================================================

	// Context Window Configuration (§8.2 Semantic Compression)
	// This controls the token budget for context compression
	ContextWindow *ContextWindowConfig `json:"context_window,omitempty"`

	// Embedding engine configuration for semantic vector search
	Embedding *EmbeddingConfig `json:"embedding,omitempty"`

	// =========================================================================
	// SHARD CONFIGURATION (Per-Shard Settings)
	// =========================================================================

	// Per-shard profiles: coder, tester, reviewer, researcher
	// Each shard type can have custom model, temperature, context limits
	ShardProfiles map[string]ShardProfile `json:"shard_profiles,omitempty"`

	// Default shard settings (fallback for undefined shard types)
	DefaultShard *ShardProfile `json:"default_shard,omitempty"`

	// =========================================================================
	// RESOURCE LIMITS
	// =========================================================================

	// Core resource limits enforced system-wide
	CoreLimits *CoreLimits `json:"core_limits,omitempty"`

	// World model scanning/AST parsing configuration
	World *WorldConfig `json:"world,omitempty"`

	// =========================================================================
	// INTEGRATIONS
	// =========================================================================

	// Integration service configuration
	Integrations *IntegrationsConfig `json:"integrations,omitempty"`

	// =========================================================================
	// TOOL GENERATION (Ouroboros)
	// =========================================================================

	// Tool Generation settings for Ouroboros self-generating tools
	ToolGeneration *ToolGenerationConfig `json:"tool_generation,omitempty"`

	// =========================================================================
	// BUILD ENVIRONMENT
	// =========================================================================

	// Build environment configuration for go build/test commands
	// Ensures consistent CGO_CFLAGS etc. across all components
	Build *BuildConfig `json:"build,omitempty"`

	// =========================================================================
	// EXECUTION SETTINGS
	// =========================================================================

	// Execution configuration for tactile interface
	Execution *ExecutionConfig `json:"execution,omitempty"`

	// =========================================================================
	// LOGGING
	// =========================================================================

	// Logging configuration
	Logging *LoggingConfig `json:"logging,omitempty"`

	// =========================================================================
	// JIT PROMPT COMPILER
	// =========================================================================

	// JIT Prompt Compiler configuration
	JIT *JITConfig `json:"jit,omitempty"`

	// =========================================================================
	// USER EXPERIENCE
	// =========================================================================

	// Onboarding state tracking for progressive disclosure
	Onboarding *OnboardingState `json:"onboarding,omitempty"`

	// Transparency configuration for operation visibility
	Transparency *TransparencyConfig `json:"transparency,omitempty"`

	// Guidance configuration for contextual help
	Guidance *GuidanceConfig `json:"guidance,omitempty"`
}

// GetContextWindowConfig returns the context window config with defaults.
func (c *UserConfig) GetContextWindowConfig() ContextWindowConfig {
	if c.ContextWindow != nil {
		cfg := *c.ContextWindow
		// Apply defaults for zero values
		if cfg.MaxTokens == 0 {
			cfg.MaxTokens = 128000
		}
		if cfg.CoreReservePercent == 0 {
			cfg.CoreReservePercent = 5
		}
		if cfg.AtomReservePercent == 0 {
			cfg.AtomReservePercent = 30
		}
		if cfg.HistoryReservePercent == 0 {
			cfg.HistoryReservePercent = 15
		}
		if cfg.WorkingReservePercent == 0 {
			cfg.WorkingReservePercent = 50
		}
		if cfg.RecentTurnWindow == 0 {
			cfg.RecentTurnWindow = 5
		}
		if cfg.CompressionThreshold == 0 {
			cfg.CompressionThreshold = 0.60
		}
		if cfg.TargetCompressionRatio == 0 {
			cfg.TargetCompressionRatio = 100.0
		}
		if cfg.ActivationThreshold == 0 {
			cfg.ActivationThreshold = 30.0
		}
		return cfg
	}
	// Return defaults
	return ContextWindowConfig{
		MaxTokens:              128000,
		CoreReservePercent:     5,
		AtomReservePercent:     30,
		HistoryReservePercent:  15,
		WorkingReservePercent:  50,
		RecentTurnWindow:       5,
		CompressionThreshold:   0.60,
		TargetCompressionRatio: 100.0,
		ActivationThreshold:    30.0,
	}
}

// GetEmbeddingConfig returns the embedding config with defaults.
func (c *UserConfig) GetEmbeddingConfig() EmbeddingConfig {
	if c.Embedding != nil {
		cfg := *c.Embedding
		// Apply defaults for zero values
		if cfg.Provider == "" {
			cfg.Provider = "ollama"
		}
		if cfg.OllamaEndpoint == "" {
			cfg.OllamaEndpoint = "http://localhost:11434"
		}
		if cfg.OllamaModel == "" {
			cfg.OllamaModel = "embeddinggemma"
		}
		if cfg.GenAIModel == "" {
			cfg.GenAIModel = "gemini-embedding-001"
		}
		if cfg.TaskType == "" {
			cfg.TaskType = "SEMANTIC_SIMILARITY"
		}
		return cfg
	}
	// Return defaults (Ollama for local processing)
	return EmbeddingConfig{
		Provider:       "ollama",
		OllamaEndpoint: "http://localhost:11434",
		OllamaModel:    "embeddinggemma",
		GenAIModel:     "gemini-embedding-001",
		TaskType:       "SEMANTIC_SIMILARITY",
	}
}

// GetToolGenerationConfig returns tool generation settings with defaults applied.
func (c *UserConfig) GetToolGenerationConfig() ToolGenerationConfig {
	cfg := DefaultToolGenerationConfig()
	if c != nil && c.ToolGeneration != nil {
		if c.ToolGeneration.TargetOS != "" {
			cfg.TargetOS = c.ToolGeneration.TargetOS
		}
		if c.ToolGeneration.TargetArch != "" {
			cfg.TargetArch = c.ToolGeneration.TargetArch
		}
	}
	return cfg
}

// GetBuildConfig returns the build configuration with defaults.
func (c *UserConfig) GetBuildConfig() BuildConfig {
	cfg := DefaultBuildConfig()
	if c.Build != nil {
		if len(c.Build.EnvVars) > 0 {
			cfg.EnvVars = c.Build.EnvVars
		}
		if len(c.Build.GoFlags) > 0 {
			cfg.GoFlags = c.Build.GoFlags
		}
		if len(c.Build.CGOPackages) > 0 {
			cfg.CGOPackages = c.Build.CGOPackages
		}
	}
	return cfg
}

// DefaultUserConfigPath returns the default path to .nerd/config.json.
func DefaultUserConfigPath() string {
	root, err := FindWorkspaceRoot()
	if err != nil {
		return ".nerd/config.json"
	}
	return filepath.Join(root, ".nerd", "config.json")
}

// FindWorkspaceRoot attempts to find the project root by looking for .nerd or go.mod.
// If not found, returns the current working directory.
func FindWorkspaceRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	originalDir := dir
	for {
		if _, err := os.Stat(filepath.Join(dir, ".nerd")); err == nil {
			return dir, nil
		}
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return originalDir, nil
}

// LoadUserConfig loads configuration from .nerd/config.json.
func LoadUserConfig(path string) (*UserConfig, error) {
	cfg := &UserConfig{}

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // Return empty config if file doesn't exist
		}
		return nil, fmt.Errorf("failed to read user config: %w", err)
	}

	if err := json.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse user config: %w", err)
	}

	return cfg, nil
}

// SaveUserConfig saves configuration to .nerd/config.json.
func (c *UserConfig) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal user config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write user config: %w", err)
	}

	return nil
}

// GetActiveProvider returns the provider and API key to use.
// Priority: explicit provider setting > first available key
func (c *UserConfig) GetActiveProvider() (provider string, apiKey string) {
	// If provider is explicitly set, use that provider's key
	if c.Provider != "" {
		switch c.Provider {
		case "anthropic":
			if c.AnthropicAPIKey != "" {
				return "anthropic", c.AnthropicAPIKey
			}
		case "openai":
			if c.OpenAIAPIKey != "" {
				return "openai", c.OpenAIAPIKey
			}
		case "gemini":
			if c.GeminiAPIKey != "" {
				return "gemini", c.GeminiAPIKey
			}
		case "xai":
			if c.XAIAPIKey != "" {
				return "xai", c.XAIAPIKey
			}
		case "zai":
			if c.ZAIAPIKey != "" {
				return "zai", c.ZAIAPIKey
			}
		case "openrouter":
			if c.OpenRouterAPIKey != "" {
				return "openrouter", c.OpenRouterAPIKey
			}
		}
	}

	// Check for provider-specific keys in priority order
	if c.AnthropicAPIKey != "" {
		return "anthropic", c.AnthropicAPIKey
	}
	if c.OpenAIAPIKey != "" {
		return "openai", c.OpenAIAPIKey
	}
	if c.GeminiAPIKey != "" {
		return "gemini", c.GeminiAPIKey
	}
	if c.XAIAPIKey != "" {
		return "xai", c.XAIAPIKey
	}
	if c.ZAIAPIKey != "" {
		return "zai", c.ZAIAPIKey
	}
	if c.OpenRouterAPIKey != "" {
		return "openrouter", c.OpenRouterAPIKey
	}

	// Legacy: single api_key field (assume zai for backward compatibility)
	if c.APIKey != "" {
		return "zai", c.APIKey
	}

	return "", ""
}

// GetEngine returns the configured engine, defaulting to "api".
func (c *UserConfig) GetEngine() string {
	if c.Engine == "" {
		return "api"
	}
	return c.Engine
}

// SetEngine updates the engine setting.
func (c *UserConfig) SetEngine(engine string) error {
	validEngines := map[string]bool{
		"api":        true,
		"claude-cli": true,
		"codex-cli":  true,
	}
	if !validEngines[engine] {
		return fmt.Errorf("invalid engine: %s (valid: api, claude-cli, codex-cli)", engine)
	}
	c.Engine = engine
	return nil
}

// GetClaudeCLIConfig returns Claude CLI config with defaults applied.
func (c *UserConfig) GetClaudeCLIConfig() *ClaudeCLIConfig {
	if c.ClaudeCLI == nil {
		return &ClaudeCLIConfig{
			Model:   "sonnet",
			Timeout: 300,
		}
	}
	cfg := *c.ClaudeCLI
	if cfg.Model == "" {
		cfg.Model = "sonnet"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300
	}
	return &cfg
}

// GetCodexCLIConfig returns Codex CLI config with defaults applied.
func (c *UserConfig) GetCodexCLIConfig() *CodexCLIConfig {
	if c.CodexCLI == nil {
		return &CodexCLIConfig{
			Model:   "gpt-5",
			Sandbox: "read-only",
			Timeout: 300,
		}
	}
	cfg := *c.CodexCLI
	if cfg.Model == "" {
		cfg.Model = "gpt-5"
	}
	if cfg.Sandbox == "" {
		cfg.Sandbox = "read-only"
	}
	if cfg.Timeout == 0 {
		cfg.Timeout = 300
	}
	return &cfg
}

// GetShardProfile returns the profile for a specific shard type, falling back to defaults.
func (c *UserConfig) GetShardProfile(shardType string) ShardProfile {
	// Check for explicit profile
	if c.ShardProfiles != nil {
		if profile, ok := c.ShardProfiles[shardType]; ok {
			return applyShardDefaults(profile)
		}
	}

	// Use default shard settings if available
	if c.DefaultShard != nil {
		return applyShardDefaults(*c.DefaultShard)
	}

	// Ultimate fallback - sensible defaults
	return ShardProfile{
		Model:                 "glm-4.7",
		Temperature:           0.7,
		TopP:                  0.9,
		MaxContextTokens:      20000,
		MaxOutputTokens:       4000,
		MaxExecutionTimeSec:   300,
		MaxRetries:            3,
		MaxFactsInShardKernel: 20000,
		EnableLearning:        true,
	}
}

// GetCoreLimits returns core resource limits with defaults applied.
func (c *UserConfig) GetCoreLimits() CoreLimits {
	if c.CoreLimits != nil {
		limits := *c.CoreLimits
		// Apply defaults for zero values
		if limits.MaxTotalMemoryMB == 0 {
			limits.MaxTotalMemoryMB = 12288
		}
		if limits.MaxConcurrentShards == 0 {
			limits.MaxConcurrentShards = 12
		}
		if limits.MaxConcurrentAPICalls == 0 {
			limits.MaxConcurrentAPICalls = 5
		}
		if limits.MaxSessionDurationMin == 0 {
			limits.MaxSessionDurationMin = 120
		}
		if limits.MaxFactsInKernel == 0 {
			limits.MaxFactsInKernel = 250000
		}
		if limits.MaxDerivedFactsLimit == 0 {
			limits.MaxDerivedFactsLimit = 100000
		}
		return limits
	}
	// Return defaults
	return CoreLimits{
		MaxTotalMemoryMB:      12288,
		MaxConcurrentShards:   12,
		MaxConcurrentAPICalls: 5,
		MaxSessionDurationMin: 120,
		MaxFactsInKernel:      250000,
		MaxDerivedFactsLimit:  100000,
	}
}

// GetWorldConfig returns world-model scanning settings with defaults.
func (c *UserConfig) GetWorldConfig() WorldConfig {
	def := DefaultWorldConfig()
	if c != nil && c.World != nil {
		cfg := *c.World
		if cfg.FastWorkers <= 0 {
			cfg.FastWorkers = def.FastWorkers
		}
		if cfg.DeepWorkers <= 0 {
			cfg.DeepWorkers = def.DeepWorkers
		}
		if len(cfg.IgnorePatterns) == 0 {
			cfg.IgnorePatterns = def.IgnorePatterns
		}
		if cfg.MaxFastASTBytes <= 0 {
			cfg.MaxFastASTBytes = def.MaxFastASTBytes
		}
		return cfg
	}
	return def
}

// GetIntegrations returns integration settings with defaults.
// By default, no external MCP servers are configured.
// Internal capabilities (code analysis, browser automation) use internal packages directly.
func (c *UserConfig) GetIntegrations() IntegrationsConfig {
	if c.Integrations != nil {
		return *c.Integrations
	}
	// Return empty - no default MCP servers. User configures external servers as needed.
	return IntegrationsConfig{
		Servers: make(map[string]MCPServerIntegration),
	}
}

// GetExecution returns execution settings with defaults.
func (c *UserConfig) GetExecution() ExecutionConfig {
	if c.Execution != nil {
		cfg := *c.Execution
		if cfg.DefaultTimeout == "" {
			cfg.DefaultTimeout = "30s"
		}
		if cfg.WorkingDirectory == "" {
			cfg.WorkingDirectory = "."
		}
		if len(cfg.AllowedBinaries) == 0 {
			cfg.AllowedBinaries = []string{
				"go", "git", "grep", "ls", "mkdir", "cp", "mv",
				"npm", "npx", "node", "python", "python3", "pip",
				"cargo", "rustc", "make", "cmake",
			}
		}
		if len(cfg.AllowedEnvVars) == 0 {
			cfg.AllowedEnvVars = []string{"PATH", "HOME", "GOPATH", "GOROOT"}
		}
		return cfg
	}
	return ExecutionConfig{
		AllowedBinaries: []string{
			"go", "git", "grep", "ls", "mkdir", "cp", "mv",
			"npm", "npx", "node", "python", "python3", "pip",
			"cargo", "rustc", "make", "cmake",
		},
		DefaultTimeout:   "30s",
		WorkingDirectory: ".",
		AllowedEnvVars:   []string{"PATH", "HOME", "GOPATH", "GOROOT"},
	}
}

// GetLogging returns logging settings with defaults.
func (c *UserConfig) GetLogging() LoggingConfig {
	if c.Logging != nil {
		cfg := *c.Logging
		if cfg.Level == "" {
			cfg.Level = "info"
		}
		if cfg.Format == "" {
			cfg.Format = "text"
		}
		// Note: DebugMode defaults to false (production mode) unless explicitly set
		return cfg
	}
	return LoggingConfig{
		Level:     "info",
		Format:    "text",
		File:      "codenerd.log",
		DebugMode: false, // Production mode by default
	}
}

// DefaultUserConfig returns a UserConfig with sensible defaults.
func DefaultUserConfig() *UserConfig {
	return &UserConfig{
		Provider: "zai",
		Model:    "glm-4.7",
		Theme:    "light",
	}
}

// GlobalConfig is a convenience function to load config from the default path.
// Returns an empty config (with defaults available via Get* methods) if file doesn't exist.
func GlobalConfig() (*UserConfig, error) {
	return LoadUserConfig(DefaultUserConfigPath())
}

// GetContext7APIKey returns the Context7 API key with auto-detection.
// Priority order:
//  1. CONTEXT7_API_KEY environment variable
//  2. UserConfig.Context7APIKey from .nerd/config.json
//
// Returns empty string if not configured.
func (c *UserConfig) GetContext7APIKey() string {
	// Priority 1: Environment variable
	if key := os.Getenv("CONTEXT7_API_KEY"); key != "" {
		return key
	}

	// Priority 2: Config file value
	if c != nil && c.Context7APIKey != "" {
		return c.Context7APIKey
	}

	return ""
}

// AutoDetectContext7APIKey is a convenience function to get the Context7 API key
// from environment variables or the default config file.
// This is useful for initializing with auto-detection when UserConfig may not be loaded.
func AutoDetectContext7APIKey() string {
	// Priority 1: Environment variable
	if key := os.Getenv("CONTEXT7_API_KEY"); key != "" {
		return key
	}

	// Priority 2: Load from config file
	cfg, err := GlobalConfig()
	if err == nil && cfg != nil && cfg.Context7APIKey != "" {
		return cfg.Context7APIKey
	}

	return ""
}

// GetJITConfig returns JIT Prompt Compiler config with defaults applied.
func (c *UserConfig) GetJITConfig() JITConfig {
	if c.JIT != nil {
		cfg := *c.JIT
		// Apply defaults for zero values (except booleans which default to false)
		if cfg.TokenBudget == 0 {
			cfg.TokenBudget = 100000
		}
		if cfg.ReservedTokens == 0 {
			cfg.ReservedTokens = 8000
		}
		if cfg.SemanticTopK == 0 {
			cfg.SemanticTopK = 20
		}
		return cfg
	}
	return DefaultJITConfig()
}

// GetOnboardingState returns the onboarding state with defaults applied.
func (c *UserConfig) GetOnboardingState() *OnboardingState {
	if c.Onboarding != nil {
		return c.Onboarding
	}
	return DefaultOnboardingState()
}

// GetTransparencyConfig returns the transparency config with defaults applied.
func (c *UserConfig) GetTransparencyConfig() *TransparencyConfig {
	if c.Transparency != nil {
		return c.Transparency
	}
	return DefaultTransparencyConfig()
}

// GetGuidanceConfig returns the guidance config with defaults applied.
func (c *UserConfig) GetGuidanceConfig() *GuidanceConfig {
	if c.Guidance != nil {
		return c.Guidance
	}
	return DefaultGuidanceConfig()
}

// IsOnboardingComplete returns true if the user has completed onboarding.
func (c *UserConfig) IsOnboardingComplete() bool {
	if c.Onboarding == nil {
		return false
	}
	return c.Onboarding.SetupComplete
}

// GetExperienceLevel returns the user's experience level.
func (c *UserConfig) GetExperienceLevel() ExperienceLevel {
	if c.Onboarding != nil && c.Onboarding.ExperienceLevel != "" {
		return c.Onboarding.ExperienceLevel
	}
	return ExperienceBeginner
}

// ShouldShowTransparency returns true if any transparency feature is enabled.
func (c *UserConfig) ShouldShowTransparency() bool {
	if c.Transparency == nil {
		return false
	}
	return c.Transparency.Enabled
}
