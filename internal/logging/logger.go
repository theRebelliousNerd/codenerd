// Package logging provides config-driven categorized file-based logging for codeNERD.
// Logs are written to .nerd/logs/ with separate files per category.
// Logging is controlled by debug_mode in .nerd/config.json - when false, no logs are written.
package logging

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Category represents a log category/system
type Category string

const (
	// Core system categories
	CategoryBoot       Category = "boot"        // Boot/initialization
	CategorySession    Category = "session"     // Session management, persistence
	CategoryKernel     Category = "kernel"      // Mangle kernel operations
	CategoryAPI        Category = "api"         // LLM API calls

	// Transduction categories
	CategoryPerception   Category = "perception"   // NL -> atoms transduction
	CategoryArticulation Category = "articulation" // Atoms -> NL (Piggyback)

	// Execution categories
	CategoryRouting     Category = "routing"      // Action routing decisions
	CategoryTools       Category = "tools"        // Tool execution
	CategoryVirtualStore Category = "virtual_store" // Virtual store operations

	// Shard categories
	CategoryShards       Category = "shards"        // Shard spawning and lifecycle
	CategoryCoder        Category = "coder"         // Coder shard activity
	CategoryTester       Category = "tester"        // Tester shard activity
	CategoryReviewer     Category = "reviewer"      // Reviewer shard activity
	CategoryResearcher   Category = "researcher"    // Researcher shard activity
	CategorySystemShards Category = "system_shards" // System shards (legislator, etc.)

	// Advanced system categories
	CategoryDream      Category = "dream"      // Dream state / what-if simulations
	CategoryAutopoiesis Category = "autopoiesis" // Self-learning, Ouroboros
	CategoryCampaign   Category = "campaign"   // Campaign orchestration
	CategoryContext    Category = "context"    // Context compression
	CategoryWorld      Category = "world"      // World scanner (filesystem, AST)
	CategoryEmbedding  Category = "embedding"  // Embedding engine
	CategoryStore      Category = "store"      // Store operations (RAM, Vector, Graph, Cold)
)

// loggingConfig mirrors the relevant parts of config.LoggingConfig
// to avoid circular imports
type loggingConfig struct {
	DebugMode  bool            `json:"debug_mode"`
	Categories map[string]bool `json:"categories"`
	Level      string          `json:"level"`
}

// configFile structure for reading .nerd/config.json
type configFile struct {
	Logging loggingConfig `json:"logging"`
}

// Logger wraps a standard logger with category and file output
type Logger struct {
	category Category
	logger   *log.Logger
	file     *os.File
}

var (
	loggers      = make(map[Category]*Logger)
	loggersMu    sync.RWMutex
	logsDir      string
	workspace    string
	config       loggingConfig
	configLoaded bool
	configMu     sync.RWMutex
	logLevel     int // 0=debug, 1=info, 2=warn, 3=error
)

// Log levels
const (
	LevelDebug = 0
	LevelInfo  = 1
	LevelWarn  = 2
	LevelError = 3
)

// Initialize sets up the logging directory and loads config.
// Should be called once at startup with the workspace path.
func Initialize(ws string) error {
	if ws == "" {
		return fmt.Errorf("workspace path required")
	}

	workspace = ws
	logsDir = filepath.Join(workspace, ".nerd", "logs")

	// Load config first to check if debug mode is enabled
	if err := loadConfig(); err != nil {
		// Log to stderr if we can't load config
		fmt.Fprintf(os.Stderr, "[logging] Warning: could not load config: %v\n", err)
		// Default to disabled (production mode)
		config.DebugMode = false
	}

	// Only create logs directory if debug mode is enabled
	if !config.DebugMode {
		return nil // Silent no-op in production mode
	}

	if err := os.MkdirAll(logsDir, 0755); err != nil {
		return fmt.Errorf("failed to create logs directory: %w", err)
	}

	// Create a boot log entry
	bootLogger := Get(CategoryBoot)
	bootLogger.Info("=== codeNERD Logging System Initialized ===")
	bootLogger.Info("Workspace: %s", workspace)
	bootLogger.Info("Logs directory: %s", logsDir)
	bootLogger.Info("Debug mode: %v", config.DebugMode)
	bootLogger.Info("Log level: %s", config.Level)

	// Log enabled categories
	if len(config.Categories) > 0 {
		enabledCount := 0
		for cat, enabled := range config.Categories {
			if enabled {
				enabledCount++
			}
			bootLogger.Debug("Category '%s': %v", cat, enabled)
		}
		bootLogger.Info("Enabled categories: %d/%d", enabledCount, len(config.Categories))
	} else {
		bootLogger.Info("All categories enabled (no category filter)")
	}

	return nil
}

// loadConfig reads the logging config from .nerd/config.json
func loadConfig() error {
	configMu.Lock()
	defer configMu.Unlock()

	configPath := filepath.Join(workspace, ".nerd", "config.json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// No config = production mode (no logging)
			config.DebugMode = false
			configLoaded = true
			return nil
		}
		return err
	}

	var cf configFile
	if err := json.Unmarshal(data, &cf); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	config = cf.Logging
	configLoaded = true

	// Parse log level
	switch config.Level {
	case "debug":
		logLevel = LevelDebug
	case "info":
		logLevel = LevelInfo
	case "warn", "warning":
		logLevel = LevelWarn
	case "error":
		logLevel = LevelError
	default:
		logLevel = LevelInfo
	}

	return nil
}

// ReloadConfig reloads the config from disk.
// Call this if config changes at runtime.
func ReloadConfig() error {
	return loadConfig()
}

// IsDebugMode returns whether debug logging is enabled
func IsDebugMode() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.DebugMode
}

// IsCategoryEnabled returns whether a specific category is enabled
func IsCategoryEnabled(category Category) bool {
	configMu.RLock()
	defer configMu.RUnlock()

	if !config.DebugMode {
		return false
	}

	if config.Categories == nil {
		return true // All enabled by default in debug mode
	}

	enabled, exists := config.Categories[string(category)]
	if !exists {
		return true // Enable by default if not specified
	}
	return enabled
}

// Get returns (or creates) a logger for the given category.
// Returns a no-op logger if debug mode is disabled or category is disabled.
func Get(category Category) *Logger {
	if !IsCategoryEnabled(category) {
		// Return a no-op logger
		return &Logger{category: category}
	}

	if logsDir == "" {
		return &Logger{category: category}
	}

	loggersMu.RLock()
	if l, ok := loggers[category]; ok {
		loggersMu.RUnlock()
		return l
	}
	loggersMu.RUnlock()

	// Create new logger
	loggersMu.Lock()
	defer loggersMu.Unlock()

	// Double-check after acquiring write lock
	if l, ok := loggers[category]; ok {
		return l
	}

	// Create log file with date prefix for easy rotation
	date := time.Now().Format("2006-01-02")
	filename := fmt.Sprintf("%s_%s.log", date, category)
	logPath := filepath.Join(logsDir, filename)

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
	if err != nil {
		// Fall back to no-op logger
		fmt.Fprintf(os.Stderr, "[logging] Warning: could not open log file %s: %v\n", logPath, err)
		return &Logger{category: category}
	}

	l := &Logger{
		category: category,
		file:     file,
		logger:   log.New(file, "", log.Ldate|log.Ltime|log.Lmicroseconds),
	}
	loggers[category] = l

	return l
}

// Debug logs a debug message (only if level <= debug)
func (l *Logger) Debug(format string, args ...interface{}) {
	if l.logger == nil || logLevel > LevelDebug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("[DEBUG] %s", msg)
}

// Info logs an informational message (only if level <= info)
func (l *Logger) Info(format string, args ...interface{}) {
	if l.logger == nil || logLevel > LevelInfo {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("[INFO] %s", msg)
}

// Warn logs a warning message (only if level <= warn)
func (l *Logger) Warn(format string, args ...interface{}) {
	if l.logger == nil || logLevel > LevelWarn {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("[WARN] %s", msg)
}

// Error logs an error message (always logged if logger exists)
func (l *Logger) Error(format string, args ...interface{}) {
	if l.logger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	l.logger.Printf("[ERROR] %s", msg)
}

// WithContext returns a context logger for structured logging
func (l *Logger) WithContext(ctx map[string]interface{}) *ContextLogger {
	return &ContextLogger{logger: l, context: ctx}
}

// ContextLogger provides structured logging with key-value context
type ContextLogger struct {
	logger  *Logger
	context map[string]interface{}
}

func (c *ContextLogger) Debug(format string, args ...interface{}) {
	if c.logger.logger == nil || logLevel > LevelDebug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.logger.logger.Printf("[DEBUG] %s | ctx=%v", msg, c.context)
}

func (c *ContextLogger) Info(format string, args ...interface{}) {
	if c.logger.logger == nil || logLevel > LevelInfo {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.logger.logger.Printf("[INFO] %s | ctx=%v", msg, c.context)
}

func (c *ContextLogger) Warn(format string, args ...interface{}) {
	if c.logger.logger == nil || logLevel > LevelWarn {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.logger.logger.Printf("[WARN] %s | ctx=%v", msg, c.context)
}

func (c *ContextLogger) Error(format string, args ...interface{}) {
	if c.logger.logger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.logger.logger.Printf("[ERROR] %s | ctx=%v", msg, c.context)
}

// CloseAll closes all open log files (call at shutdown)
func CloseAll() {
	loggersMu.Lock()
	defer loggersMu.Unlock()

	for _, l := range loggers {
		if l.file != nil {
			l.file.Close()
		}
	}
	loggers = make(map[Category]*Logger)
}

// =============================================================================
// CONVENIENCE FUNCTIONS - Quick logging without getting a logger first
// These are no-ops if the category is disabled
// =============================================================================

// Boot logs to the boot category
func Boot(format string, args ...interface{}) {
	Get(CategoryBoot).Info(format, args...)
}

// BootDebug logs debug to the boot category
func BootDebug(format string, args ...interface{}) {
	Get(CategoryBoot).Debug(format, args...)
}

// Session logs to the session category
func Session(format string, args ...interface{}) {
	Get(CategorySession).Info(format, args...)
}

// SessionDebug logs debug to the session category
func SessionDebug(format string, args ...interface{}) {
	Get(CategorySession).Debug(format, args...)
}

// Kernel logs to the kernel category
func Kernel(format string, args ...interface{}) {
	Get(CategoryKernel).Info(format, args...)
}

// KernelDebug logs debug to the kernel category
func KernelDebug(format string, args ...interface{}) {
	Get(CategoryKernel).Debug(format, args...)
}

// API logs to the api category
func API(format string, args ...interface{}) {
	Get(CategoryAPI).Info(format, args...)
}

// APIDebug logs debug to the api category
func APIDebug(format string, args ...interface{}) {
	Get(CategoryAPI).Debug(format, args...)
}

// Perception logs to the perception category
func Perception(format string, args ...interface{}) {
	Get(CategoryPerception).Info(format, args...)
}

// PerceptionDebug logs debug to the perception category
func PerceptionDebug(format string, args ...interface{}) {
	Get(CategoryPerception).Debug(format, args...)
}

// Articulation logs to the articulation category
func Articulation(format string, args ...interface{}) {
	Get(CategoryArticulation).Info(format, args...)
}

// ArticulationDebug logs debug to the articulation category
func ArticulationDebug(format string, args ...interface{}) {
	Get(CategoryArticulation).Debug(format, args...)
}

// Routing logs to the routing category
func Routing(format string, args ...interface{}) {
	Get(CategoryRouting).Info(format, args...)
}

// RoutingDebug logs debug to the routing category
func RoutingDebug(format string, args ...interface{}) {
	Get(CategoryRouting).Debug(format, args...)
}

// Tools logs to the tools category
func Tools(format string, args ...interface{}) {
	Get(CategoryTools).Info(format, args...)
}

// ToolsDebug logs debug to the tools category
func ToolsDebug(format string, args ...interface{}) {
	Get(CategoryTools).Debug(format, args...)
}

// VirtualStore logs to the virtual_store category
func VirtualStore(format string, args ...interface{}) {
	Get(CategoryVirtualStore).Info(format, args...)
}

// VirtualStoreDebug logs debug to the virtual_store category
func VirtualStoreDebug(format string, args ...interface{}) {
	Get(CategoryVirtualStore).Debug(format, args...)
}

// Shards logs to the shards category
func Shards(format string, args ...interface{}) {
	Get(CategoryShards).Info(format, args...)
}

// ShardsDebug logs debug to the shards category
func ShardsDebug(format string, args ...interface{}) {
	Get(CategoryShards).Debug(format, args...)
}

// Coder logs to the coder category
func Coder(format string, args ...interface{}) {
	Get(CategoryCoder).Info(format, args...)
}

// CoderDebug logs debug to the coder category
func CoderDebug(format string, args ...interface{}) {
	Get(CategoryCoder).Debug(format, args...)
}

// Tester logs to the tester category
func Tester(format string, args ...interface{}) {
	Get(CategoryTester).Info(format, args...)
}

// TesterDebug logs debug to the tester category
func TesterDebug(format string, args ...interface{}) {
	Get(CategoryTester).Debug(format, args...)
}

// Reviewer logs to the reviewer category
func Reviewer(format string, args ...interface{}) {
	Get(CategoryReviewer).Info(format, args...)
}

// ReviewerDebug logs debug to the reviewer category
func ReviewerDebug(format string, args ...interface{}) {
	Get(CategoryReviewer).Debug(format, args...)
}

// Researcher logs to the researcher category
func Researcher(format string, args ...interface{}) {
	Get(CategoryResearcher).Info(format, args...)
}

// ResearcherDebug logs debug to the researcher category
func ResearcherDebug(format string, args ...interface{}) {
	Get(CategoryResearcher).Debug(format, args...)
}

// SystemShards logs to the system_shards category
func SystemShards(format string, args ...interface{}) {
	Get(CategorySystemShards).Info(format, args...)
}

// SystemShardsDebug logs debug to the system_shards category
func SystemShardsDebug(format string, args ...interface{}) {
	Get(CategorySystemShards).Debug(format, args...)
}

// Dream logs to the dream category
func Dream(format string, args ...interface{}) {
	Get(CategoryDream).Info(format, args...)
}

// DreamDebug logs debug to the dream category
func DreamDebug(format string, args ...interface{}) {
	Get(CategoryDream).Debug(format, args...)
}

// Autopoiesis logs to the autopoiesis category
func Autopoiesis(format string, args ...interface{}) {
	Get(CategoryAutopoiesis).Info(format, args...)
}

// AutopoiesisDebug logs debug to the autopoiesis category
func AutopoiesisDebug(format string, args ...interface{}) {
	Get(CategoryAutopoiesis).Debug(format, args...)
}

// Campaign logs to the campaign category
func Campaign(format string, args ...interface{}) {
	Get(CategoryCampaign).Info(format, args...)
}

// CampaignDebug logs debug to the campaign category
func CampaignDebug(format string, args ...interface{}) {
	Get(CategoryCampaign).Debug(format, args...)
}

// Context logs to the context category
func Context(format string, args ...interface{}) {
	Get(CategoryContext).Info(format, args...)
}

// ContextDebug logs debug to the context category
func ContextDebug(format string, args ...interface{}) {
	Get(CategoryContext).Debug(format, args...)
}

// World logs to the world category
func World(format string, args ...interface{}) {
	Get(CategoryWorld).Info(format, args...)
}

// WorldDebug logs debug to the world category
func WorldDebug(format string, args ...interface{}) {
	Get(CategoryWorld).Debug(format, args...)
}

// Embedding logs to the embedding category
func Embedding(format string, args ...interface{}) {
	Get(CategoryEmbedding).Info(format, args...)
}

// EmbeddingDebug logs debug to the embedding category
func EmbeddingDebug(format string, args ...interface{}) {
	Get(CategoryEmbedding).Debug(format, args...)
}

// Store logs to the store category
func Store(format string, args ...interface{}) {
	Get(CategoryStore).Info(format, args...)
}

// StoreDebug logs debug to the store category
func StoreDebug(format string, args ...interface{}) {
	Get(CategoryStore).Debug(format, args...)
}

// =============================================================================
// TIMING HELPERS - For performance logging
// =============================================================================

// Timer helps measure operation duration
type Timer struct {
	category Category
	op       string
	start    time.Time
}

// StartTimer begins timing an operation
func StartTimer(category Category, operation string) *Timer {
	return &Timer{
		category: category,
		op:       operation,
		start:    time.Now(),
	}
}

// Stop ends the timer and logs the duration
func (t *Timer) Stop() time.Duration {
	elapsed := time.Since(t.start)
	Get(t.category).Debug("%s completed in %v", t.op, elapsed)
	return elapsed
}

// StopWithInfo ends the timer and logs at info level
func (t *Timer) StopWithInfo() time.Duration {
	elapsed := time.Since(t.start)
	Get(t.category).Info("%s completed in %v", t.op, elapsed)
	return elapsed
}

// StopWithThreshold logs warning if duration exceeds threshold
func (t *Timer) StopWithThreshold(threshold time.Duration) time.Duration {
	elapsed := time.Since(t.start)
	if elapsed > threshold {
		Get(t.category).Warn("%s took %v (threshold: %v)", t.op, elapsed, threshold)
	} else {
		Get(t.category).Debug("%s completed in %v", t.op, elapsed)
	}
	return elapsed
}
