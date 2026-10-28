package config

import "codenerd/internal/logging"

// LoggingConfig configures logging.
type LoggingConfig struct {
	Level      string          `yaml:"level" json:"level,omitempty"`               // debug, info, warn, error
	Format     string          `yaml:"format" json:"format,omitempty"`             // json, text
	File       string          `yaml:"file" json:"file,omitempty"`                 // legacy single file
	DebugMode  bool            `yaml:"debug_mode" json:"debug_mode,omitempty"`     // Master toggle - false = no logging (production)
	DebugSelf  bool            `yaml:"debug_self" json:"debug_self,omitempty"`     // Watch own problems log per chat turn; implies DebugMode (never the work mode)
	TraceLLMIO bool            `yaml:"trace_llm_io" json:"trace_llm_io,omitempty"` // Dump full LLM prompt packages and responses to llm_io log
	Categories map[string]bool `yaml:"categories" json:"categories,omitempty"`     // Per-category toggles
	// PerformanceSampling controls sampling rate for non-slow performance logs (0.0-1.0).
	PerformanceSampling float64 `yaml:"performance_sampling" json:"performance_sampling,omitempty"`
	// PerformanceThresholdsMs sets per-system slow thresholds in milliseconds.
	PerformanceThresholdsMs map[string]int64 `yaml:"performance_thresholds_ms" json:"performance_thresholds_ms,omitempty"`
	// SelfProblemWarnRepeats is how often one warning must recur in the
	// problems log before debug-self calls it hot and names it to the model
	// (self_problem_hot, policy/self_problems.mg). Errors are always hot.
	// Default DefaultSelfProblemWarnRepeats.
	SelfProblemWarnRepeats int `yaml:"self_problem_warn_repeats" json:"self_problem_warn_repeats,omitempty"`
}

// DefaultSelfProblemWarnRepeats is the warning recurrence at which debug-self
// treats a warning as a problem worth the model's attention.
const DefaultSelfProblemWarnRepeats = 3

// Params are the logging knobs the kernel's rules read, as config_param rows
// (config_param_required(/logging, Key), policy/self_problems.mg). Nil-safe.
func (c *LoggingConfig) Params() []Param {
	repeats := DefaultSelfProblemWarnRepeats
	if c != nil && c.SelfProblemWarnRepeats > 0 {
		repeats = c.SelfProblemWarnRepeats
	}
	return []Param{{Key: "/logging_self_problem_warn_repeats", Value: int64(repeats)}}
}

// EffectiveDebugMode reports whether logging is effectively on. DebugSelf
// implies DebugMode: without debug mode no log files exist to watch.
// Nil-safe.
func (c *LoggingConfig) EffectiveDebugMode() bool {
	return c != nil && (c.DebugMode || c.DebugSelf)
}

// ToLoggingConfig converts these settings into the injectable view
// internal/logging accepts, so boot can hand over the config it already
// parsed instead of internal/logging re-reading .nerd/config.json.
func (c *LoggingConfig) ToLoggingConfig() logging.Config {
	if c == nil {
		return logging.Config{}
	}
	// TraceLLMIORaw, MaxLogFileMB, MaxLogFileMinutes and MaxRotatedFiles have
	// no source on this side and remain at their zero values.
	return logging.Config{
		DebugMode:               c.EffectiveDebugMode(),
		DebugSelf:               c.DebugSelf,
		TraceLLMIO:              c.TraceLLMIO,
		Categories:              c.Categories,
		Level:                   c.Level,
		Format:                  c.Format,
		PerformanceSampling:     c.PerformanceSampling,
		PerformanceThresholdsMs: c.PerformanceThresholdsMs,
	}
}

// IsCategoryEnabled returns whether logging is enabled for a category.
// Returns false if debug_mode is false (production mode).
// Returns true if debug_mode is true and category is enabled (or not specified).
func (c *LoggingConfig) IsCategoryEnabled(category string) bool {
	if !c.EffectiveDebugMode() {
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
