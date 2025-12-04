package config

import (
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
	// LLM API key from environment
	if key := os.Getenv("ZAI_API_KEY"); key != "" {
		c.LLM.APIKey = key
	}
	if key := os.Getenv("ANTHROPIC_API_KEY"); key != "" {
		c.LLM.APIKey = key
		c.LLM.Provider = "anthropic"
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		c.LLM.APIKey = key
		c.LLM.Provider = "openai"
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

// Validate validates the configuration.
func (c *Config) Validate() error {
	if c.LLM.APIKey == "" {
		return fmt.Errorf("LLM API key not configured (set ZAI_API_KEY, ANTHROPIC_API_KEY, or OPENAI_API_KEY)")
	}

	if c.LLM.Provider != "zai" && c.LLM.Provider != "anthropic" && c.LLM.Provider != "openai" {
		return fmt.Errorf("invalid LLM provider: %s", c.LLM.Provider)
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
