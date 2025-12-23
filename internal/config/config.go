package config

import (
	"codenerd/internal/logging"
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

	// Embedding engine configuration
	Embedding EmbeddingConfig `yaml:"embedding"`

	// Integration services
	Integrations IntegrationsConfig `yaml:"integrations"`

	// Execution settings
	Execution ExecutionConfig `yaml:"execution"`

	// Tool Generation settings
	ToolGeneration ToolGenerationConfig `yaml:"tool_generation" json:"tool_generation"`

	// Transparency settings
	Transparency TransparencyConfig `yaml:"transparency" json:"transparency"`

	// Logging
	Logging LoggingConfig `yaml:"logging"`

	// Per-Shard Configuration (CRITICAL - addresses feedback)
	ShardProfiles map[string]ShardProfile `yaml:"shard_profiles" json:"shard_profiles"`

	// Default Shard Settings (fallback)
	DefaultShard ShardProfile `yaml:"default_shard" json:"default_shard"`

	// Core Resource Limits (enforced system-wide)
	CoreLimits CoreLimits `yaml:"core_limits" json:"core_limits"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		Name:    "codeNERD",
		Version: "1.5.0",

		LLM: LLMConfig{
			Provider: "zai",
			Model:    "glm-4.7", // Z.AI GLM-4.7 - Default for codeNERD
			BaseURL:  "https://api.z.ai/api/coding/paas/v4",
			Timeout:  "120s",
		},

		Mangle: MangleConfig{
			SchemaPath:   "", // Empty triggers embedded defaults + .nerd/mangle extensions
			PolicyPath:   "", // Empty triggers embedded defaults + .nerd/mangle extensions
			FactLimit:    1000000,
			QueryTimeout: "30s",
		},

		Memory: MemoryConfig{
			WorkingMemorySize: 20000,
			DatabasePath:      "data/codenerd.db",
			SessionTTL:        "24h",
			ContextWindow: ContextWindowConfig{
				MaxTokens:              128000,
				CoreReservePercent:     5,
				AtomReservePercent:     30,
				HistoryReservePercent:  15,
				WorkingReservePercent:  50,
				RecentTurnWindow:       5,
				CompressionThreshold:   0.60,
				TargetCompressionRatio: 100.0,
				ActivationThreshold:    30.0,
			},
		},

		// Embedding engine defaults (Ollama for local, fast embeddings)
		Embedding: EmbeddingConfig{
			Provider:       "ollama",                 // Default to local Ollama
			OllamaEndpoint: "http://localhost:11434", // Ollama default port
			OllamaModel:    "embeddinggemma",         // embeddinggemma for local embeddings
			GenAIModel:     "gemini-embedding-001",   // GenAI default model
			TaskType:       "SEMANTIC_SIMILARITY",    // Default task type
		},

		Integrations: IntegrationsConfig{
			Servers: map[string]MCPServerIntegration{
				"code_graph": {
					Enabled: true,
					BaseURL: "http://localhost:8080",
					Timeout: "30s",
				},
				"browser": {
					Enabled: true,
					BaseURL: "http://localhost:8081",
					Timeout: "60s",
				},
				"scraper": {
					Enabled: true,
					BaseURL: "http://localhost:8082",
					Timeout: "120s",
				},
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

		ToolGeneration: DefaultToolGenerationConfig(),

		Transparency: *DefaultTransparencyConfig(),

		Logging: LoggingConfig{
			Level:  "info",
			Format: "text",
			File:   "codenerd.log",
		},

		// Core resource limits (enforced system-wide)
		CoreLimits: CoreLimits{
			MaxTotalMemoryMB:      12288,  // 12GB RAM limit
			MaxConcurrentShards:   4,      // Max 4 parallel shards
			MaxSessionDurationMin: 120,    // 2 hour sessions
			MaxFactsInKernel:      250000, // Increase working-set ceiling with larger RAM
			MaxDerivedFactsLimit:  100000, // Mangle gas limit scales with fact budget
		},

		// Default shard settings (fallback for undefined shard types)
		DefaultShard: ShardProfile{
			Model:                 "glm-4.7", // Inherit from main LLM config
			Temperature:           0.7,
			TopP:                  0.9,
			MaxContextTokens:      20000,
			MaxOutputTokens:       4000,
			MaxExecutionTimeSec:   300, // 5 min
			MaxRetries:            3,
			MaxFactsInShardKernel: 20000,
			EnableLearning:        true,
		},

		// Per-shard profiles (custom settings per shard type)
		ShardProfiles: map[string]ShardProfile{
			"coder": {
				Model:                 "glm-4.7", // Z.AI GLM-4.7 for code generation
				Temperature:           0.7,
				TopP:                  0.9,
				MaxContextTokens:      30000, // More context for code
				MaxOutputTokens:       6000,
				MaxExecutionTimeSec:   600, // 10 min
				MaxRetries:            3,
				MaxFactsInShardKernel: 30000,
				EnableLearning:        true,
			},
			"tester": {
				Model:                 "glm-4.7", // Z.AI GLM-4.7 for test generation
				Temperature:           0.5,       // Lower temp for precise tests
				TopP:                  0.9,
				MaxContextTokens:      20000,
				MaxOutputTokens:       4000,
				MaxExecutionTimeSec:   300,
				MaxRetries:            3,
				MaxFactsInShardKernel: 20000,
				EnableLearning:        true,
			},
			"reviewer": {
				Model:                 "glm-4.7", // Z.AI GLM-4.7 for code review
				Temperature:           0.3,       // Very low temp for rigorous analysis
				TopP:                  0.9,
				MaxContextTokens:      40000, // Max context for full codebase
				MaxOutputTokens:       8000,
				MaxExecutionTimeSec:   900, // 15 min
				MaxRetries:            2,
				MaxFactsInShardKernel: 30000,
				EnableLearning:        false, // No learning for safety-critical
			},
			"researcher": {
				Model:                 "glm-4.7", // Z.AI GLM-4.7 for research
				Temperature:           0.6,
				TopP:                  0.95,
				MaxContextTokens:      25000,
				MaxOutputTokens:       5000,
				MaxExecutionTimeSec:   600, // 10 min for deep research
				MaxRetries:            3,
				MaxFactsInShardKernel: 25000,
				EnableLearning:        true,
			},
		},
	}
}

// Load loads configuration from a YAML file.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()
	logging.BootDebug("Loading config from: %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			// Return defaults if config file doesn't exist
			logging.Boot("Config file not found, using defaults: %s", path)
			cfg.applyEnvOverrides()
			logging.BootDebug("Config loaded: provider=%s", cfg.LLM.Provider)
			return cfg, nil
		}
		logging.BootError("Failed to read config file %s: %v", path, err)
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		logging.BootError("Failed to parse config file %s: %v", path, err)
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Override with environment variables
	cfg.applyEnvOverrides()
	logging.Boot("Config loaded: provider=%s model=%s", cfg.LLM.Provider, cfg.LLM.Model)

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
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		c.LLM.APIKey = key
		c.LLM.Provider = "openrouter"
	}

	// Integration URLs from environment
	if url := os.Getenv("CODEGRAPH_URL"); url != "" {
		c.setMCPServerURL("code_graph", url)
	}
	if url := os.Getenv("BROWSERNERD_URL"); url != "" {
		c.setMCPServerURL("browser", url)
	}
	if url := os.Getenv("SCRAPER_URL"); url != "" {
		c.setMCPServerURL("scraper", url)
	}

	// Database path from environment
	if path := os.Getenv("CODENERD_DB"); path != "" {
		c.Memory.DatabasePath = path
	}

	// Embedding configuration from environment
	if key := os.Getenv("GENAI_API_KEY"); key != "" {
		c.Embedding.GenAIAPIKey = key
		if c.Embedding.Provider == "" || c.Embedding.Provider == "ollama" {
			// Only switch to genai if no provider explicitly set or using default
			c.Embedding.Provider = "genai"
		}
	}
	if endpoint := os.Getenv("OLLAMA_ENDPOINT"); endpoint != "" {
		c.Embedding.OllamaEndpoint = endpoint
	}
	if model := os.Getenv("OLLAMA_EMBEDDING_MODEL"); model != "" {
		c.Embedding.OllamaModel = model
	}
}

// GetLLMTimeout returns the LLM timeout as a duration.
func (c *Config) GetLLMTimeout() time.Duration {
	d, err := time.ParseDuration(c.LLM.Timeout)
	if err != nil {
		return 300 * time.Second
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

// =============================================================================
// SHARD PROFILE HELPERS
// =============================================================================

// GetShardProfile returns the profile for a given shard type, falling back to default.
func (c *Config) GetShardProfile(shardType string) ShardProfile {
	if profile, ok := c.ShardProfiles[shardType]; ok {
		return profile
	}
	return c.DefaultShard
}

// SetShardProfile updates or adds a shard profile.
func (c *Config) SetShardProfile(shardType string, profile ShardProfile) {
	if c.ShardProfiles == nil {
		c.ShardProfiles = make(map[string]ShardProfile)
	}
	c.ShardProfiles[shardType] = profile
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
var ValidProviders = []string{"zai", "anthropic", "openai", "gemini", "xai", "openrouter"}

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

// setMCPServerURL is a helper to update a server's URL in the integrations map.
func (c *Config) setMCPServerURL(serverID, url string) {
	if c.Integrations.Servers == nil {
		c.Integrations.Servers = make(map[string]MCPServerIntegration)
	}
	server := c.Integrations.Servers[serverID]
	server.BaseURL = url
	c.Integrations.Servers[serverID] = server
}

// IsMCPServerEnabled returns whether a specific MCP server is enabled.
func (c *Config) IsMCPServerEnabled(serverID string) bool {
	return c.Integrations.IsServerEnabled(serverID)
}

// IsCodeGraphEnabled returns whether code graph integration is enabled.
func (c *Config) IsCodeGraphEnabled() bool {
	return c.IsMCPServerEnabled("code_graph")
}

// IsBrowserEnabled returns whether browser integration is enabled.
func (c *Config) IsBrowserEnabled() bool {
	return c.IsMCPServerEnabled("browser")
}

// IsScraperEnabled returns whether scraper integration is enabled.
func (c *Config) IsScraperEnabled() bool {
	return c.IsMCPServerEnabled("scraper")
}
