package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gopkg.in/yaml.v3"
)

// Config holds all codeNERD configuration.
type Config struct {
	// Core settings
	Name    string `yaml:"name"`
	Version string `yaml:"version"`

	// LLM configuration
	LLM LLMConfig `yaml:"llm"`

	// Mangle kernel configuration
	Mangle MangleConfig `yaml:"mangle"`

	// Memory shards configuration
	Memory MemoryConfig `yaml:"memory"`

	// Integration services
	Integrations IntegrationsConfig `yaml:"integrations"`

	// Execution settings
	Execution ExecutionConfig `yaml:"execution"`

	// Logging
	Logging LoggingConfig `yaml:"logging"`
}

// LLMConfig configures the LLM transducer.
type LLMConfig struct {
	Provider string `yaml:"provider"` // zai, anthropic, openai
	APIKey   string `yaml:"api_key"`
	Model    string `yaml:"model"`
	BaseURL  string `yaml:"base_url"`
	Timeout  string `yaml:"timeout"`
}

// MangleConfig configures the Mangle kernel.
type MangleConfig struct {
	SchemaPath   string `yaml:"schema_path"`
	PolicyPath   string `yaml:"policy_path"`
	FactLimit    int    `yaml:"fact_limit"`
	QueryTimeout string `yaml:"query_timeout"`
}

// MemoryConfig configures the memory shards.
type MemoryConfig struct {
	// Shard A: Working Memory (RAM)
	WorkingMemorySize int `yaml:"working_memory_size"`

	// Shard B/C/D: SQLite storage
	DatabasePath string `yaml:"database_path"`

	// Session management
	SessionTTL string `yaml:"session_ttl"`

	// Context Window Management (§8.2 Semantic Compression)
	ContextWindow ContextWindowConfig `yaml:"context_window"`
}

// ContextWindowConfig configures the semantic compression context window.
type ContextWindowConfig struct {
	// Maximum tokens for the context window (default: 128000)
	MaxTokens int `yaml:"max_tokens" json:"max_tokens"`

	// Token budget allocation percentages
	CoreReservePercent     int `yaml:"core_reserve_percent" json:"core_reserve_percent"`      // % for constitutional facts (default: 5)
	AtomReservePercent     int `yaml:"atom_reserve_percent" json:"atom_reserve_percent"`      // % for high-activation atoms (default: 30)
	HistoryReservePercent  int `yaml:"history_reserve_percent" json:"history_reserve_percent"` // % for compressed history (default: 15)
	WorkingReservePercent  int `yaml:"working_reserve_percent" json:"working_reserve_percent"` // % for working memory (default: 50)

	// Recent turn window (how many turns to keep with full metadata)
	RecentTurnWindow int `yaml:"recent_turn_window" json:"recent_turn_window"`

	// Compression settings
	CompressionThreshold   float64 `yaml:"compression_threshold" json:"compression_threshold"`       // Trigger at this % usage (default: 0.80)
	TargetCompressionRatio float64 `yaml:"target_compression_ratio" json:"target_compression_ratio"` // Target ratio (default: 100.0)
	ActivationThreshold    float64 `yaml:"activation_threshold" json:"activation_threshold"`         // Min score to include (default: 30.0)
}

// IntegrationsConfig configures external service integrations.
type IntegrationsConfig struct {
	// code-graph-mcp-server
	CodeGraph CodeGraphIntegration `yaml:"code_graph"`

	// BrowserNERD
	Browser BrowserIntegration `yaml:"browser"`

	// scraper_service
	Scraper ScraperIntegration `yaml:"scraper"`
}

// CodeGraphIntegration configures the code graph MCP server.
type CodeGraphIntegration struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
	Timeout string `yaml:"timeout"`
}

// BrowserIntegration configures BrowserNERD.
type BrowserIntegration struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
	Timeout string `yaml:"timeout"`
}

// ScraperIntegration configures the scraper service.
type ScraperIntegration struct {
	Enabled bool   `yaml:"enabled"`
	BaseURL string `yaml:"base_url"`
	Timeout string `yaml:"timeout"`
}

// ExecutionConfig configures the tactile interface.
type ExecutionConfig struct {
	// Allowed binaries (Constitutional Logic)
	AllowedBinaries []string `yaml:"allowed_binaries"`

	// Default timeout for commands
	DefaultTimeout string `yaml:"default_timeout"`

	// Working directory
	WorkingDirectory string `yaml:"working_directory"`

	// Environment variables to pass
	AllowedEnvVars []string `yaml:"allowed_env_vars"`
}

// LoggingConfig configures logging.
type LoggingConfig struct {
	Level  string `yaml:"level"` // debug, info, warn, error
	Format string `yaml:"format"` // json, text
	File   string `yaml:"file"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Name:    "codeNERD",
		Version: "1.5.0",

		LLM: LLMConfig{
			Provider: "zai",
			Model:    "claude-sonnet-4-20250514",
			BaseURL:  "https://api.zukijourney.com/v1",
			Timeout:  "120s",
		},

		Mangle: MangleConfig{
			SchemaPath:   "internal/mangle/schemas.gl",
			PolicyPath:   "internal/mangle/policy.gl",
			FactLimit:    1000000,
			QueryTimeout: "30s",
		},

		Memory: MemoryConfig{
			WorkingMemorySize: 10000,
			DatabasePath:      "data/codenerd.db",
			SessionTTL:        "24h",
			ContextWindow: ContextWindowConfig{
				MaxTokens:              128000,
				CoreReservePercent:     5,
				AtomReservePercent:     30,
				HistoryReservePercent:  15,
				WorkingReservePercent:  50,
				RecentTurnWindow:       5,
				CompressionThreshold:   0.80,
				TargetCompressionRatio: 100.0,
				ActivationThreshold:    30.0,
			},
		},

		Integrations: IntegrationsConfig{
			CodeGraph: CodeGraphIntegration{
				Enabled: true,
				BaseURL: "http://localhost:8080",
				Timeout: "30s",
			},
			Browser: BrowserIntegration{
				Enabled: true,
				BaseURL: "http://localhost:8081",
				Timeout: "60s",
			},
			Scraper: ScraperIntegration{
				Enabled: true,
				BaseURL: "http://localhost:8082",
				Timeout: "120s",
			},
		},

		Execution: ExecutionConfig{
			AllowedBinaries: []string{
				"go", "git", "grep", "ls", "mkdir", "cp", "mv",
				"npm", "npx", "node", "python", "python3", "pip",
				"cargo", "rustc", "make", "cmake",
			},
			DefaultTimeout:   "30s",
			WorkingDirectory: ".",
			AllowedEnvVars:   []string{"PATH", "HOME", "GOPATH", "GOROOT"},
		},

		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   "codenerd.log",
		},
	}
}

// Load loads configuration from a YAML file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults if config file doesn't exist
			return cfg, nil
		}
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Override with environment variables
	cfg.applyEnvOverrides()

	return cfg, nil
}

// Save saves configuration to a YAML file.
func (c *Config) Save(path string) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config: %w", err)
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides.
func (c *Config) applyEnvOverrides() {
	// LLM API key from environment (check in priority order)
	if key := os.Getenv("ZAI_API_KEY"); key != "" {
		c.LLM.APIKey = key
		if c.LLM.Provider == "" {
			c.LLM.Provider = "zai"
		}
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		c.LLM.APIKey = key
		c.LLM.Provider = "anthropic"
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		c.LLM.APIKey = key
		c.LLM.Provider = "openai"
	}
	if key := os.Getenv("GEMINI_API_KEY"); key != "" {
		c.LLM.APIKey = key
		c.LLM.Provider = "gemini"
	}
	if key := os.Getenv("XAI_API_KEY"); key != "" {
		c.LLM.APIKey = key
		c.LLM.Provider = "xai"
	}

	// Integration URLs from environment
	if url := os.Getenv("CODEGRAPH_URL"); url != "" {
		c.Integrations.CodeGraph.BaseURL = url
	}
	if url := os.Getenv("BROWSERNERD_URL"); url != "" {
		c.Integrations.Browser.BaseURL = url
	}
	if url := os.Getenv("SCRAPER_URL"); url != "" {
		c.Integrations.Scraper.BaseURL = url
	}

	// Database path from environment
	if path := os.Getenv("CODENERD_DB"); path != "" {
		c.Memory.DatabasePath = path
	}
}

// GetLLMTimeout returns the LLM timeout as a duration.
func (c *Config) GetLLMTimeout() time.Duration {
	d, err := time.ParseDuration(c.LLM.Timeout)
	if err != nil {
		return 120 * time.Second
	}
	return d
}

// GetQueryTimeout returns the Mangle query timeout as a duration.
func (c *Config) GetQueryTimeout() time.Duration {
	d, err := time.ParseDuration(c.Mangle.QueryTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// GetExecutionTimeout returns the default execution timeout as a duration.
func (c *Config) GetExecutionTimeout() time.Duration {
	d, err := time.ParseDuration(c.Execution.DefaultTimeout)
	if err != nil {
		return 30 * time.Second
	}
	return d
}

// GetSessionTTL returns the session TTL as a duration.
func (c *Config) GetSessionTTL() time.Duration {
	d, err := time.ParseDuration(c.Memory.SessionTTL)
	if err != nil {
		return 24 * time.Hour
	}
	return d
}

// ValidProviders lists all supported LLM providers.
var ValidProviders = []string{"zai", "anthropic", "openai", "gemini", "xai"}

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.LLM.APIKey == "" {
		return fmt.Errorf("LLM API key not configured (set ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, XAI_API_KEY, or ZAI_API_KEY)")
	}

	validProvider := false
	for _, p := range ValidProviders {
		if c.LLM.Provider == p {
			validProvider = true
			break
		}
	}
	if !validProvider {
		return fmt.Errorf("invalid LLM provider: %s (valid: %v)", c.LLM.Provider, ValidProviders)
	}

	return nil
}

// IsCodeGraphEnabled returns whether code graph integration is enabled.
func (c *Config) IsCodeGraphEnabled() bool {
	return c.Integrations.CodeGraph.Enabled
}

// IsBrowserEnabled returns whether browser integration is enabled.
func (c *Config) IsBrowserEnabled() bool {
	return c.Integrations.Browser.Enabled
}

// IsScraperEnabled returns whether scraper integration is enabled.
func (c *Config) IsScraperEnabled() bool {
	return c.Integrations.Scraper.Enabled
}

// ============================================================================
// User Config (.nerd/config.json)
// ============================================================================

// UserConfig holds user-specific settings from .nerd/config.json.
//
// Supported models by provider:
//   - anthropic: claude-sonnet-4-5-20250514, claude-opus-4-20250514, claude-3-5-sonnet-20241022
//   - openai:    gpt-5.1-codex-max (default), gpt-5.1-codex-mini, gpt-5-codex, gpt-4o
//   - gemini:    gemini-3-pro-preview (default), gemini-2.5-pro, gemini-2.5-flash
//   - xai:       grok-2-latest (default), grok-2, grok-beta
//   - zai:       GLM-4.6 (default)
type UserConfig struct {
	// Provider selection (anthropic, openai, gemini, xai, zai)
	Provider string `json:"provider,omitempty"`

	// API keys for each provider
	APIKey          string `json:"api_key,omitempty"`           // Legacy: single key
	AnthropicAPIKey string `json:"anthropic_api_key,omitempty"` // Anthropic/Claude
	OpenAIAPIKey    string `json:"openai_api_key,omitempty"`    // OpenAI/Codex
	GeminiAPIKey    string `json:"gemini_api_key,omitempty"`    // Google Gemini
	XAIAPIKey       string `json:"xai_api_key,omitempty"`       // xAI/Grok
	ZAIAPIKey       string `json:"zai_api_key,omitempty"`       // Z.AI

	// Optional model override (see supported models above)
	Model string `json:"model,omitempty"`

	// UI settings
	Theme string `json:"theme,omitempty"`

	// Context7 API key
	Context7APIKey string `json:"context7_api_key,omitempty"`

	// Context Window Configuration (§8.2 Semantic Compression)
	// This controls the token budget for context compression
	ContextWindow *ContextWindowConfig `json:"context_window,omitempty"`
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
			cfg.CompressionThreshold = 0.80
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
		CompressionThreshold:   0.80,
		TargetCompressionRatio: 100.0,
		ActivationThreshold:    30.0,
	}
}

// DefaultUserConfigPath returns the default path to .nerd/config.json.
func DefaultUserConfigPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ".nerd/config.json"
	}
	return filepath.Join(cwd, ".nerd", "config.json")
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

	// Legacy: single api_key field (assume zai for backward compatibility)
	if c.APIKey != "" {
		return "zai", c.APIKey
	}

	return "", ""
}
