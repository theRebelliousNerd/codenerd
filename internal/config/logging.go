package config

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"codenerd/internal/logging"
)

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
	// Self-repair (policy/self_repair.mg): while debug_self is on, codeNERD
	// diagnoses and fixes its own hot problems, and then its costliest
	// recurring operation, in the background, and restarts onto each fix.
	// SelfRepairDisabled keeps debug-self to reporting only.
	SelfRepairDisabled bool `yaml:"self_repair_disabled" json:"self_repair_disabled,omitempty"`
	// SelfRepairCooldownSec is how long a target rests after its attempt
	// changed state before it is tried again. Default
	// DefaultSelfRepairCooldownSec.
	SelfRepairCooldownSec int `yaml:"self_repair_cooldown_sec" json:"self_repair_cooldown_sec,omitempty"`
	// SelfRepairMaxFailures is how many failed or regressed attempts leave a
	// target alone for good. Default DefaultSelfRepairMaxFailures.
	SelfRepairMaxFailures int `yaml:"self_repair_max_failures" json:"self_repair_max_failures,omitempty"`
	// SelfRepairMaxJobs bounds the self-repair jobs running at once. Default
	// DefaultSelfRepairMaxJobs.
	SelfRepairMaxJobs int `yaml:"self_repair_max_jobs" json:"self_repair_max_jobs,omitempty"`
	// SelfRepairVerifyWindowSec is how long a repaired problem must stay out
	// of the problems log of the process running its fix to count as fixed.
	// Default DefaultSelfRepairVerifyWindowSec.
	SelfRepairVerifyWindowSec int `yaml:"self_repair_verify_window_sec" json:"self_repair_verify_window_sec,omitempty"`
	// SelfRepairPollSec is how often an idle chat asks the kernel whether a
	// self-repair is due. Default DefaultSelfRepairPollSec.
	SelfRepairPollSec int `yaml:"self_repair_poll_sec" json:"self_repair_poll_sec,omitempty"`
	// SelfOptimizeMinCount is how often an operation must have been timed in
	// the run before the optimize lane may choose it, and before its fix is
	// judged. Default DefaultSelfOptimizeMinCount.
	SelfOptimizeMinCount int `yaml:"self_optimize_min_count" json:"self_optimize_min_count,omitempty"`
	// SelfRepairCheckTimeoutSec bounds each self-check step a rebuilt binary
	// must pass before it replaces the running one. Default
	// DefaultSelfRepairCheckTimeoutSec.
	SelfRepairCheckTimeoutSec int `yaml:"self_repair_check_timeout_sec" json:"self_repair_check_timeout_sec,omitempty"`
	// SessionTTL is how long a session folder (.nerd/sessions/<id>/) is kept
	// after its last activity, as a duration string ("14d", "36h"). Unset keeps
	// DefaultSessionTTL; "0" or "off" keeps every session forever. Expired
	// sessions are deleted at boot, never the booting one or a running one.
	SessionTTL string `yaml:"session_ttl" json:"session_ttl,omitempty"`
}

// DefaultSessionTTL is the retention when logging.session_ttl is unset.
const DefaultSessionTTL = 14 * 24 * time.Hour

// SessionRetention parses logging.session_ttl: unset is DefaultSessionTTL,
// "0" or "off" is zero (keep everything), anything else is a positive
// duration. Go durations are accepted, plus a day suffix ("14d"). Nil-safe.
func (c *LoggingConfig) SessionRetention() (time.Duration, error) {
	if c == nil {
		return DefaultSessionTTL, nil
	}
	raw := strings.ToLower(strings.TrimSpace(c.SessionTTL))
	switch raw {
	case "":
		return DefaultSessionTTL, nil
	case "0", "off":
		return 0, nil
	}
	var d time.Duration
	if days, ok := strings.CutSuffix(raw, "d"); ok {
		n, err := strconv.ParseFloat(days, 64)
		if err != nil {
			return 0, fmt.Errorf("logging.session_ttl %q is not a duration: %w", c.SessionTTL, err)
		}
		d = time.Duration(n * float64(24*time.Hour))
	} else {
		var err error
		if d, err = time.ParseDuration(raw); err != nil {
			return 0, fmt.Errorf("logging.session_ttl %q is not a duration: %w", c.SessionTTL, err)
		}
	}
	if d <= 0 {
		return 0, fmt.Errorf("logging.session_ttl %q must be positive; use \"off\" to keep every session", c.SessionTTL)
	}
	return d, nil
}

// DefaultSelfProblemWarnRepeats is the warning recurrence at which debug-self
// treats a warning as a problem worth the model's attention.
const DefaultSelfProblemWarnRepeats = 3

// Self-repair defaults (LoggingConfig's self_repair_* keys).
const (
	DefaultSelfRepairCooldownSec     = 600
	DefaultSelfRepairMaxFailures     = 2
	DefaultSelfRepairMaxJobs         = 1
	DefaultSelfRepairVerifyWindowSec = 300
	DefaultSelfRepairPollSec         = 30
	DefaultSelfOptimizeMinCount      = 20
	DefaultSelfRepairCheckTimeoutSec = 300
)

// orDefault is v when the config set it, else def.
func orDefault(v, def int) int {
	if v > 0 {
		return v
	}
	return def
}

// Params are the logging knobs the kernel's rules read, as config_param rows
// (config_param_required(/logging, Key), policy/self_problems.mg and
// policy/self_repair.mg). Nil-safe.
func (c *LoggingConfig) Params() []Param {
	var l LoggingConfig
	if c != nil {
		l = *c
	}
	enabled := int64(1)
	if l.SelfRepairDisabled {
		enabled = 0
	}
	return []Param{
		{Key: "/logging_self_problem_warn_repeats", Value: int64(orDefault(l.SelfProblemWarnRepeats, DefaultSelfProblemWarnRepeats))},
		{Key: "/logging_self_repair_enabled", Value: enabled},
		{Key: "/logging_self_repair_cooldown_sec", Value: int64(orDefault(l.SelfRepairCooldownSec, DefaultSelfRepairCooldownSec))},
		{Key: "/logging_self_repair_max_failures", Value: int64(orDefault(l.SelfRepairMaxFailures, DefaultSelfRepairMaxFailures))},
		{Key: "/logging_self_repair_max_jobs", Value: int64(orDefault(l.SelfRepairMaxJobs, DefaultSelfRepairMaxJobs))},
		{Key: "/logging_self_repair_verify_window_sec", Value: int64(orDefault(l.SelfRepairVerifyWindowSec, DefaultSelfRepairVerifyWindowSec))},
		{Key: "/logging_self_optimize_min_count", Value: int64(orDefault(l.SelfOptimizeMinCount, DefaultSelfOptimizeMinCount))},
	}
}

// SelfRepairPoll is how often an idle chat asks whether a self-repair is due.
// Nil-safe.
func (c *LoggingConfig) SelfRepairPoll() time.Duration {
	v := 0
	if c != nil {
		v = c.SelfRepairPollSec
	}
	return time.Duration(orDefault(v, DefaultSelfRepairPollSec)) * time.Second
}

// SelfRepairCheckTimeout bounds each self-check step of a rebuilt binary.
// Nil-safe.
func (c *LoggingConfig) SelfRepairCheckTimeout() time.Duration {
	v := 0
	if c != nil {
		v = c.SelfRepairCheckTimeoutSec
	}
	return time.Duration(orDefault(v, DefaultSelfRepairCheckTimeoutSec)) * time.Second
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
