package config

// LoggingConfig configures logging.
type LoggingConfig struct {
	Level      string          `yaml:"level" json:"level,omitempty"`               // debug, info, warn, error
	Format     string          `yaml:"format" json:"format,omitempty"`             // json, text
	File       string          `yaml:"file" json:"file,omitempty"`                 // legacy single file
	DebugMode  bool            `yaml:"debug_mode" json:"debug_mode,omitempty"`     // Master toggle - false = no logging (production)
	TraceLLMIO bool            `yaml:"trace_llm_io" json:"trace_llm_io,omitempty"` // Dump full LLM prompt packages and responses to llm_io log
	Categories map[string]bool `yaml:"categories" json:"categories,omitempty"`     // Per-category toggles
	// PerformanceSampling controls sampling rate for non-slow performance logs (0.0-1.0).
	PerformanceSampling float64 `yaml:"performance_sampling" json:"performance_sampling,omitempty"`
	// PerformanceThresholdsMs sets per-system slow thresholds in milliseconds.
	PerformanceThresholdsMs map[string]int64 `yaml:"performance_thresholds_ms" json:"performance_thresholds_ms,omitempty"`
}

// IsCategoryEnabled returns whether logging is enabled for a category.
// Returns false if debug_mode is false (production mode).
// Returns true if debug_mode is true and category is enabled (or not specified).
func (c *LoggingConfig) IsCategoryEnabled(category string) bool {
	if !c.DebugMode {
		return false
	}
	if c.Categories == nil {
		return true // All enabled by default in debug mode
	}
	enabled, exists := c.Categories[category]
	if !exists {
		return true // Enable by default if not specified
	}
	return enabled
}

// DefaultLoggingConfig returns a LoggingConfig with sensible defaults.
func DefaultLoggingConfig() *LoggingConfig {
	return &LoggingConfig{
		Level:      "info",
		Format:     "text",
		File:       "codenerd.log",
		DebugMode:  false,
		TraceLLMIO: false,
		Categories: map[string]bool{
			"boot":    true,
			"kernel":  true,
			"session": true,
		},
		PerformanceSampling: 1.0,
		PerformanceThresholdsMs: map[string]int64{
			"kernel_eval": 100,
			"llm_call":    1000,
		},
	}
}
