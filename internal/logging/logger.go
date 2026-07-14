// Package logging provides config-driven categorized file-based logging for codeNERD.
// Logs are written to .nerd/logs/ with separate files per category.
// Logging is controlled by debug_mode in .nerd/config.json - when false, no logs are written.
package logging

import (
	"crypto/rand"
	"encoding/binary"
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
	CategoryBoot        Category = "boot"        // Boot/initialization
	CategorySession     Category = "session"     // Session management, persistence
	CategoryPerformance Category = "performance" // Performance metrics, slow operations
	CategoryKernel      Category = "kernel"      // Mangle kernel operations
	CategoryAPI         Category = "api"         // LLM API calls

	// Transduction categories
	CategoryPerception   Category = "perception"   // NL -> atoms transduction
	CategoryArticulation Category = "articulation" // Atoms -> NL (Piggyback)

	// Execution categories
	CategoryRouting      Category = "routing"       // Action routing decisions
	CategoryTools        Category = "tools"         // Tool execution
	CategoryVirtualStore Category = "virtual_store" // Virtual store operations

	// Shard categories
	CategoryShards       Category = "shards"        // Shard spawning and lifecycle
	CategoryCoder        Category = "coder"         // Coder shard activity
	CategoryTester       Category = "tester"        // Tester shard activity
	CategoryReviewer     Category = "reviewer"      // Reviewer shard activity
	CategoryResearcher   Category = "researcher"    // Researcher shard activity
	CategorySystemShards Category = "system_shards" // System shards (legislator, etc.)

	// Advanced system categories
	CategoryDream       Category = "dream"       // Dream state / what-if simulations
	CategoryAutopoiesis Category = "autopoiesis" // Self-learning, Ouroboros
	CategoryCampaign    Category = "campaign"    // Campaign orchestration
	CategoryContext     Category = "context"     // Context compression
	CategoryWorld       Category = "world"       // World scanner (filesystem, AST)
	CategoryEmbedding   Category = "embedding"   // Embedding engine
	CategoryStore       Category = "store"       // Store operations (RAM, Vector, Graph, Cold)
	CategoryBrowser     Category = "browser"     // Browser automation, DOM events
	CategoryTactile     Category = "tactile"     // Tactile executor, command execution
	CategoryJIT         Category = "jit"         // JIT Prompt Compiler operations
	CategoryBuild       Category = "build"       // Build environment and compilation
	CategoryNorthstar   Category = "northstar"   // Northstar vision guardian
)

// loggingConfig mirrors the relevant parts of config.LoggingConfig
// to avoid circular imports
type loggingConfig struct {
	DebugMode  bool            `json:"debug_mode"`
	TraceLLMIO bool            `json:"trace_llm_io"` // Dump full LLM prompt/response to llm_io log
	Categories map[string]bool `json:"categories"`
	Level      string          `json:"level"`
	JSONFormat bool            `json:"json_format"` // Output structured JSON for Mangle parsing
	// PerformanceSampling controls sampling rate for non-slow performance logs (0.0-1.0).
	PerformanceSampling float64 `json:"performance_sampling"`
	// PerformanceThresholdsMs sets per-system slow thresholds in milliseconds.
	PerformanceThresholdsMs map[string]int64 `json:"performance_thresholds_ms"`
}

// configFile structure for reading .nerd/config.json
type configFile struct {
	Logging loggingConfig `json:"logging"`
}

// StructuredLogEntry represents a JSON log entry for Mangle parsing
// Format: log_entry(Timestamp, Category, Level, Message, File, Line)
type StructuredLogEntry struct {
	Timestamp int64          `json:"ts"`               // Unix milliseconds
	Category  string         `json:"cat"`              // Log category
	Level     string         `json:"lvl"`              // debug/info/warn/error
	Message   string         `json:"msg"`              // Log message
	File      string         `json:"file"`             // Source file (optional)
	Line      int            `json:"line"`             // Source line (optional)
	RequestID string         `json:"req,omitempty"`    // Request correlation ID
	Fields    map[string]any `json:"fields,omitempty"` // Additional structured fields
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

	// Initialization guard (Bug #1 fix)
	initOnce    sync.Once
	initErr     error
	initialized bool
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
// Multiple calls are safe - only the first call will take effect (idempotent).
func Initialize(ws string) error {
	if ws == "" {
		return fmt.Errorf("workspace path required")
	}

	// Use sync.Once to prevent re-initialization (Bug #1: Init Spam fix)
	initOnce.Do(func() {
		initErr = initializeInternal(ws)
		if initErr == nil {
			initialized = true
		}
	})

	return initErr
}

// initializeInternal performs the actual initialization logic.
// This is only called once via sync.Once in Initialize().
func initializeInternal(ws string) error {
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

	if err := os.MkdirAll(logsDir, 0o700); err != nil {
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

	if err := InitAudit(); err != nil {
		bootLogger.Warn("Failed to initialize audit logging: %v", err)
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

	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
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

// logJSON writes a structured JSON log entry
func (l *Logger) logJSON(level, msg string) {
	entry := StructuredLogEntry{
		Timestamp: time.Now().UnixMilli(),
		Category:  string(l.category),
		Level:     level,
		Message:   msg,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		l.logger.Printf("[%s] %s", level, msg) // Fallback to text
		return
	}
	l.logger.Printf("%s", data)
}

// Debug logs a debug message (only if level <= debug)
func (l *Logger) Debug(format string, args ...any) {
	if l.logger == nil || logLevel > LevelDebug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if config.JSONFormat {
		l.logJSON("debug", msg)
	} else {
		l.logger.Printf("[DEBUG] %s", msg)
	}
}

// Info logs an informational message (only if level <= info)
func (l *Logger) Info(format string, args ...any) {
	if l.logger == nil || logLevel > LevelInfo {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if config.JSONFormat {
		l.logJSON("info", msg)
	} else {
		l.logger.Printf("[INFO] %s", msg)
	}
}

// Warn logs a warning message (only if level <= warn)
func (l *Logger) Warn(format string, args ...any) {
	if l.logger == nil || logLevel > LevelWarn {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if config.JSONFormat {
		l.logJSON("warn", msg)
	} else {
		l.logger.Printf("[WARN] %s", msg)
	}
}

// Error logs an error message (always logged if logger exists)
func (l *Logger) Error(format string, args ...any) {
	if l.logger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if config.JSONFormat {
		l.logJSON("error", msg)
	} else {
		l.logger.Printf("[ERROR] %s", msg)
	}
}

// StructuredLog writes a fully structured log entry with custom fields
func (l *Logger) StructuredLog(level string, msg string, fields map[string]any) {
	if l.logger == nil {
		return
	}
	entry := StructuredLogEntry{
		Timestamp: time.Now().UnixMilli(),
		Category:  string(l.category),
		Level:     level,
		Message:   msg,
		Fields:    fields,
	}
	if config.JSONFormat {
		data, err := json.Marshal(entry)
		if err == nil {
			l.logger.Printf("%s", data)
			return
		}
	}
	// Fallback to text format with fields
	l.logger.Printf("[%s] %s | fields=%v", level, msg, fields)
}

// IsJSONFormat returns whether JSON logging is enabled
func IsJSONFormat() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.JSONFormat
}

// WithContext returns a context logger for structured logging
func (l *Logger) WithContext(ctx map[string]any) *ContextLogger {
	return &ContextLogger{logger: l, context: ctx}
}

// ContextLogger provides structured logging with key-value context
type ContextLogger struct {
	logger  *Logger
	context map[string]any
}

func (c *ContextLogger) Debug(format string, args ...any) {
	if c.logger.logger == nil || logLevel > LevelDebug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.logger.logger.Printf("[DEBUG] %s | ctx=%v", msg, c.context)
}

func (c *ContextLogger) Info(format string, args ...any) {
	if c.logger.logger == nil || logLevel > LevelInfo {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.logger.logger.Printf("[INFO] %s | ctx=%v", msg, c.context)
}

func (c *ContextLogger) Warn(format string, args ...any) {
	if c.logger.logger == nil || logLevel > LevelWarn {
		return
	}
	msg := fmt.Sprintf(format, args...)
	c.logger.logger.Printf("[WARN] %s | ctx=%v", msg, c.context)
}

func (c *ContextLogger) Error(format string, args ...any) {
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
// REQUEST ID TRACING - For distributed request tracing
// =============================================================================

// RequestLogger provides request-scoped logging with a correlation ID
type RequestLogger struct {
	logger    *Logger
	requestID string
	fields    map[string]any
}

// WithRequestID creates a request-scoped logger for distributed tracing
func WithRequestID(category Category, requestID string) *RequestLogger {
	return &RequestLogger{
		logger:    Get(category),
		requestID: requestID,
		fields:    make(map[string]any),
	}
}

// WithField adds a field to the request logger
func (r *RequestLogger) WithField(key string, value any) *RequestLogger {
	r.fields[key] = value
	return r
}

func (r *RequestLogger) formatMsg(format string, args ...any) string {
	msg := fmt.Sprintf(format, args...)
	if len(r.fields) > 0 {
		return fmt.Sprintf("[req:%s] %s | %v", r.requestID, msg, r.fields)
	}
	return fmt.Sprintf("[req:%s] %s", r.requestID, msg)
}

func (r *RequestLogger) Debug(format string, args ...any) {
	if r.logger.logger == nil || logLevel > LevelDebug {
		return
	}
	r.logger.logger.Printf("[DEBUG] %s", r.formatMsg(format, args...))
}

func (r *RequestLogger) Info(format string, args ...any) {
	if r.logger.logger == nil || logLevel > LevelInfo {
		return
	}
	r.logger.logger.Printf("[INFO] %s", r.formatMsg(format, args...))
}

func (r *RequestLogger) Warn(format string, args ...any) {
	if r.logger.logger == nil || logLevel > LevelWarn {
		return
	}
	r.logger.logger.Printf("[WARN] %s", r.formatMsg(format, args...))
}

func (r *RequestLogger) Error(format string, args ...any) {
	if r.logger.logger == nil {
		return
	}
	r.logger.logger.Printf("[ERROR] %s", r.formatMsg(format, args...))
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

func shouldLogLevel(level string) bool {
	switch level {
	case "debug":
		return logLevel <= LevelDebug
	case "info":
		return logLevel <= LevelInfo
	case "warn":
		return logLevel <= LevelWarn
	case "error":
		return logLevel <= LevelError
	default:
		return logLevel <= LevelInfo
	}
}

func performanceSamplingRate() float64 {
	configMu.RLock()
	defer configMu.RUnlock()
	if !configLoaded {
		return 1.0
	}
	if config.PerformanceSampling <= 0 {
		return 1.0
	}
	if config.PerformanceSampling > 1 {
		return 1.0
	}
	return config.PerformanceSampling
}

func performanceThresholdMs(category Category) int64 {
	configMu.RLock()
	defer configMu.RUnlock()
	if !configLoaded || config.PerformanceThresholdsMs == nil {
		return 0
	}
	if threshold, ok := config.PerformanceThresholdsMs[string(category)]; ok {
		return threshold
	}
	if threshold, ok := config.PerformanceThresholdsMs["default"]; ok {
		return threshold
	}
	return 0
}

func logPerformance(category Category, operation string, elapsed time.Duration, threshold *time.Duration) {
	if category == CategoryPerformance {
		return
	}
	if !IsCategoryEnabled(CategoryPerformance) {
		return
	}

	thresholdMs := int64(0)
	if threshold != nil {
		thresholdMs = threshold.Milliseconds()
	} else {
		thresholdMs = performanceThresholdMs(category)
	}

	elapsedMs := elapsed.Milliseconds()
	isSlow := thresholdMs > 0 && elapsedMs > thresholdMs
	if !isSlow {
		sampleRate := performanceSamplingRate()
		if sampleRate < 1 && secureFloat64() > sampleRate {
			return
		}
	}

	level := "info"
	if isSlow {
		level = "warn"
	}
	if !shouldLogLevel(level) {
		return
	}

	logger := Get(CategoryPerformance)
	if logger.logger == nil {
		return
	}

	fields := map[string]any{
		"system":      string(category),
		"operation":   operation,
		"duration_ms": elapsedMs,
	}
	if thresholdMs > 0 {
		fields["threshold_ms"] = thresholdMs
	}

	logger.StructuredLog(level, fmt.Sprintf("%s.%s", category, operation), fields)

	auditLogger := AuditWithContext("", "", category)
	auditLogger.PerfMetric(operation, elapsedMs, thresholdMs)
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
	logPerformance(t.category, t.op, elapsed, nil)
	return elapsed
}

// StopWithInfo ends the timer and logs at info level
func (t *Timer) StopWithInfo() time.Duration {
	elapsed := time.Since(t.start)
	Get(t.category).Info("%s completed in %v", t.op, elapsed)
	logPerformance(t.category, t.op, elapsed, nil)
	return elapsed
}

// StopWithThreshold logs warning if duration exceeds threshold
func (t *Timer) StopWithThreshold(threshold time.Duration) time.Duration {
	elapsed := time.Since(t.start)
	if elapsed > threshold {
		Get(t.category).Warn("%s took %v (threshold: %v)", t.op, elapsed, threshold)
		logPerformance(t.category, t.op, elapsed, &threshold)
	} else {
		Get(t.category).Debug("%s completed in %v", t.op, elapsed)
		logPerformance(t.category, t.op, elapsed, &threshold)
	}
	return elapsed
}

// secureFloat64 returns a random float64 in [0.0, 1.0) using crypto/rand.
func secureFloat64() float64 {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return float64(binary.LittleEndian.Uint64(b[:])&(1<<53-1)) / (1 << 53)
}
