package config

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// Config holds user preferences
type Config struct {
	APIKey         string `json:"api_key"`
	Theme          string `json:"theme"`           // "light" or "dark"
	Context7APIKey string `json:"context7_api_key"` // Context7 API key for research

	// Context Window Configuration (§8.2 Semantic Compression)
	ContextWindow *ContextWindowConfig `json:"context_window,omitempty"`

	// Embedding engine configuration
	Embedding *EmbeddingConfig `json:"embedding,omitempty"`
}

// EmbeddingConfig configures the vector embedding engine.
type EmbeddingConfig struct {
	Provider       string `json:"provider"`        // "ollama" or "genai"
	OllamaEndpoint string `json:"ollama_endpoint"` // Default: "http://localhost:11434"
	OllamaModel    string `json:"ollama_model"`    // Default: "embeddinggemma"
	GenAIAPIKey    string `json:"genai_api_key"`
	GenAIModel     string `json:"genai_model"`     // Default: "gemini-embedding-001"
	TaskType       string `json:"task_type"`       // Default: "SEMANTIC_SIMILARITY"
}

// ContextWindowConfig configures the semantic compression context window.
type ContextWindowConfig struct {
	// Maximum tokens for the context window (default: 128000)
	MaxTokens int `json:"max_tokens"`

	// Token budget allocation percentages
	CoreReservePercent    int `json:"core_reserve_percent"`    // % for constitutional facts (default: 5)
	AtomReservePercent    int `json:"atom_reserve_percent"`    // % for high-activation atoms (default: 30)
	HistoryReservePercent int `json:"history_reserve_percent"` // % for compressed history (default: 15)
	WorkingReservePercent int `json:"working_reserve_percent"` // % for working memory (default: 50)

	// Recent turn window (how many turns to keep with full metadata)
	RecentTurnWindow int `json:"recent_turn_window"`

	// Compression settings
	CompressionThreshold   float64 `json:"compression_threshold"`    // Trigger at this % usage (default: 0.80)
	TargetCompressionRatio float64 `json:"target_compression_ratio"` // Target ratio (default: 100.0)
	ActivationThreshold    float64 `json:"activation_threshold"`     // Min score to include (default: 30.0)
}

// GetContextWindowConfig returns the context window config with defaults.
func (c *Config) GetContextWindowConfig() ContextWindowConfig {
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

// GetEmbeddingConfig returns the embedding config with defaults.
func (c *Config) GetEmbeddingConfig() EmbeddingConfig {
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

// DefaultConfig returns the default configuration
func DefaultConfig() Config {
	return Config{
		Theme: "light",
	}
}

// ConfigDir returns the directory where config is stored
func ConfigDir() (string, error) {
	// Prefer project-local .nerd directory if present or creatable
	if cwd, err := os.Getwd(); err == nil {
		localDir := filepath.Join(cwd, ".nerd")
		if stat, err := os.Stat(localDir); (err == nil && stat.IsDir()) || os.IsNotExist(err) {
			return localDir, nil
		}
	}

	// Fallback to home-level config
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".codenerd"), nil
}

// ConfigFile returns the full path to the config file
func ConfigFile() (string, error) {
	dir, err := ConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "config.json"), nil
}

// Load reads the configuration from disk
func Load() (Config, error) {
	path, err := ConfigFile()
	if err != nil {
		return DefaultConfig(), err
	}

	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return DefaultConfig(), nil
	}
	if err != nil {
		return DefaultConfig(), err
	}

	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return DefaultConfig(), err
	}

	return cfg, nil
}

// Save writes the configuration to disk
func Save(cfg Config) error {
	dir, err := ConfigDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	path, err := ConfigFile()
	if err != nil {
		return err
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, data, 0644)
}
