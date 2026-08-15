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
	"runtime"
	"strings"
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
	CategoryRegression  Category = "regression"  // Regression battery runs
	CategoryPersist     Category = "persist"     // Fact snapshot write/read
)

// loggingConfig mirrors the relevant parts of config.LoggingConfig
// to avoid circular imports (config imports logging, never the reverse).
//
// Schema decision (TODO P1 "align json_format vs Format"): `format` is
// canonical. config.LoggingConfig — the struct the rest of the app loads from
// this same .nerd/config.json — carries `format: "json"|"text"` and has no
// json_format field at all, so a config written by the app could never turn
// this package's JSON mode on. `json_format` stays accepted as a legacy alias
// because workspaces and the corpus README already document it; either key
// enables structured output, and `format: "json"` wins nothing over
// `json_format: true` — they are OR'd, not ranked, so neither loader can
// silently disable what the other enabled.
type loggingConfig struct {
	DebugMode  bool            `json:"debug_mode"`
	TraceLLMIO bool            `json:"trace_llm_io"` // Dump full LLM prompt/response to llm_io log
	Categories map[string]bool `json:"categories"`
	Level      string          `json:"level"`
	Format     string          `json:"format"`      // "json" | "text" — canonical, matches config.LoggingConfig
	JSONFormat bool            `json:"json_format"` // Legacy alias for format: "json"
	// TraceLLMIORaw disables secret redaction in the LLM I/O trace. Off by
	// default: the trace is a full prompt dump and prompts carry credentials.
	TraceLLMIORaw bool `json:"trace_llm_io_raw"`
	// PerformanceSampling controls sampling rate for non-slow performance logs (0.0-1.0).
	PerformanceSampling float64 `json:"performance_sampling"`
	// PerformanceThresholdsMs sets per-system slow thresholds in milliseconds.
	PerformanceThresholdsMs map[string]int64 `json:"performance_thresholds_ms"`
	// MaxLogFileMB caps one log segment before rotation (0 = default 32, negative = never rotate on size).
	MaxLogFileMB int64 `json:"max_log_file_mb"`
	// MaxLogFileMinutes rotates a segment once it is this old (0 = age rotation off).
	MaxLogFileMinutes int64 `json:"max_log_file_minutes"`
	// MaxRotatedFiles is how many archived segments to keep per file (0 = default 3, negative = keep none).
	MaxRotatedFiles int `json:"max_rotated_files"`
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

// Logger wraps a standard logger with category and file output.
// sink is nil for no-op loggers and for the in-memory loggers tests build.
type Logger struct {
	category Category
	logger   *log.Logger
	sink     *rotatingFile
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

	// initMu serializes Initialize so a rebind cannot interleave with a
	// concurrent first init or a second rebind.
	initMu sync.Mutex
	// boundWorkspace is the absolute path the current sinks belong to. It is
	// what makes a rebind detectable; see Initialize.
	boundWorkspace string
	// configInjected suppresses the on-disk config read. Set by ApplyConfig
	// when boot hands us an already-parsed config.
	configInjected bool
)

// Log levels
const (
	LevelDebug = 0
	LevelInfo  = 1
	LevelWarn  = 2
	LevelError = 3
)

// Initialize sets up the logging directory and loads config.
//
// Calling it repeatedly with the SAME workspace is idempotent (Bug #1: Init
// Spam) — the second call is a no-op and returns the first call's result.
// Calling it with a DIFFERENT workspace rebinds every sink to the new one.
//
// The rebind is not a nicety, it is the fix for a silent misbinding. main()
// calls Initialize(os.Getwd()) before Cobra has parsed argv, so a plain
// sync.Once guard bound the logger to the current directory and the later
// `nerd --workspace /elsewhere ...` init in PersistentPreRunE hit a consumed
// Once and did nothing: every log line for that run, including the audit
// trail, landed in the wrong workspace with no diagnostic. Binding is now
// last-writer-wins on an absolute path, which matches the flag's semantics —
// --workspace is an override, and an override that arrives late still has to
// win.
func Initialize(ws string) error {
	if ws == "" {
		return fmt.Errorf("workspace path required")
	}
	target := absWorkspace(ws)

	initMu.Lock()
	defer initMu.Unlock()

	// Use sync.Once for the first init so the "already initialized" fast path
	// stays exactly as cheap as before.
	initOnce.Do(func() {
		initErr = initializeInternal(target)
		if initErr == nil {
			initialized = true
			boundWorkspace = target
		}
	})
	if !initialized || boundWorkspace == target {
		return initErr
	}

	previous := boundWorkspace
	closeAllSinks()
	resetLLMIOLogger()
	configMu.Lock()
	configLoaded = false
	config = loggingConfig{}
	configMu.Unlock()

	initErr = initializeInternal(target)
	if initErr != nil {
		initialized = false
		boundWorkspace = ""
		return initErr
	}
	initialized = true
	boundWorkspace = target
	// Recorded in the NEW workspace's boot log: the old one is now orphaned and
	// nobody looking at it would otherwise know why it stops mid-run.
	Get(CategoryBoot).Info("Logging rebound from workspace %s to %s", previous, target)
	return nil
}

// absWorkspace normalizes a workspace path for binding comparison. Relative
// --workspace values and os.Getwd() must compare equal when they name the same
// directory, or every command would look like a rebind.
func absWorkspace(ws string) string {
	if abs, err := filepath.Abs(ws); err == nil {
		return filepath.Clean(abs)
	}
	return filepath.Clean(ws)
}

// BoundWorkspace reports the workspace the sinks are currently attached to, or
// "" before the first successful Initialize. Boot code uses it to check that
// --workspace actually took effect.
func BoundWorkspace() string {
	initMu.Lock()
	defer initMu.Unlock()
	return boundWorkspace
}

// Config is the injectable view of the logging settings. It exists so boot can
// hand over the config it already parsed instead of this package re-reading and
// re-parsing .nerd/config.json — the "same file the rest of the app treats as
// source of truth" problem. internal/config imports this package, so the flow
// has to be config -> logging.ApplyConfig, never logging -> config.
type Config struct {
	DebugMode               bool
	TraceLLMIO              bool
	TraceLLMIORaw           bool
	Categories              map[string]bool
	Level                   string
	Format                  string // "json" | "text"
	JSONFormat              bool   // legacy alias for Format == "json"
	PerformanceSampling     float64
	PerformanceThresholdsMs map[string]int64
	MaxLogFileMB            int64
	MaxLogFileMinutes       int64
	MaxRotatedFiles         int
}

// ApplyConfig installs an externally parsed logging config and pins it, so a
// later Initialize (or ReloadConfig) does not overwrite it from disk. Call it
// BEFORE Initialize when boot has already loaded the user config; calling it
// after is also valid and takes effect for every subsequently created logger.
func ApplyConfig(c Config) {
	configMu.Lock()
	config = loggingConfig{
		DebugMode:               c.DebugMode,
		TraceLLMIO:              c.TraceLLMIO,
		TraceLLMIORaw:           c.TraceLLMIORaw,
		Categories:              c.Categories,
		Level:                   c.Level,
		Format:                  c.Format,
		JSONFormat:              c.JSONFormat,
		PerformanceSampling:     c.PerformanceSampling,
		PerformanceThresholdsMs: c.PerformanceThresholdsMs,
		MaxLogFileMB:            c.MaxLogFileMB,
		MaxLogFileMinutes:       c.MaxLogFileMinutes,
		MaxRotatedFiles:         c.MaxRotatedFiles,
	}
	configLoaded = true
	configInjected = true
	applyLevelLocked(config.Level)
	configMu.Unlock()
}

// ClearInjectedConfig releases the pin set by ApplyConfig so config is read
// from disk again. Tests and `nerd` subcommands that switch workspaces use it.
func ClearInjectedConfig() {
	configMu.Lock()
	configInjected = false
	configMu.Unlock()
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

	runPrefixMu.Lock()
	runPrefix = generateRunPrefix()
	runPrefixMu.Unlock()
	clearOrdinaryLogs(logsDir)

	// Only create logs directory if debug mode is enabled
	if !config.DebugMode {
		return nil // Silent no-op in production mode
	}

	if logsDirSymlinkRejected(logsDir) {
		return fmt.Errorf("refusing symlinked logs directory: %s", logsDir)
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

// loadConfig reads the logging config from .nerd/config.json — the same file
// config.LoadUserConfig treats as the source of truth. This package parses only
// the `logging` object of it, and only because it sits below internal/config in
// the import graph; ApplyConfig is the way to avoid the second parse.
func loadConfig() error {
	configMu.Lock()
	defer configMu.Unlock()

	if configInjected {
		return nil // boot handed us the config; disk must not override it
	}

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
	applyLevelLocked(config.Level)

	return nil
}

// applyLevelLocked maps the configured level name onto logLevel. Caller holds
// configMu.
func applyLevelLocked(level string) {
	switch strings.ToLower(strings.TrimSpace(level)) {
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

	// Create log file with run prefix for isolation (date fallback before Initialize)
	prefix := currentRunPrefix()
	if prefix == "" {
		prefix = time.Now().Format("2006-01-02")
	}
	filename := fmt.Sprintf("%s_%s.log", prefix, category)
	logPath := filepath.Join(logsDir, filename)
	sink, err := openRotatingFile(logPath)
	if err != nil {
		// Fall back to no-op logger
		fmt.Fprintf(os.Stderr, "[logging] Warning: could not open log file %s: %v\n", logPath, err)
		return &Logger{category: category}
	}

	l := &Logger{
		category: category,
		sink:     sink,
		logger:   log.New(sink, "", log.Ldate|log.Ltime|log.Lmicroseconds),
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
	entry.File, entry.Line = callerSite()
	data, err := json.Marshal(entry)
	if err != nil {
		l.logger.Printf("[%s] %s", level, msg) // Fallback to text
		return
	}
	l.logger.Printf("%s", data)
}

// callerSite returns the first stack frame outside this package, so a JSON
// entry points at the code that logged rather than at logger.go. Frames are
// walked (rather than a fixed skip count) because the convenience wrappers in
// logger_convenience.go add one frame and the Context/Request loggers add
// another; a hardcoded depth was wrong for two of the three entry paths.
//
// Only called on the JSON path: runtime.Callers on every text line would tax
// the hot path for a field the text format does not even carry.
func callerSite() (string, int) {
	var pcs [12]uintptr
	n := runtime.Callers(3, pcs[:]) // skip runtime.Callers, callerSite, its caller
	if n == 0 {
		return "", 0
	}
	frames := runtime.CallersFrames(pcs[:n])
	for {
		frame, more := frames.Next()
		if frame.File != "" && !strings.Contains(filepath.ToSlash(frame.File), "/internal/logging/") {
			return filepath.Base(frame.File), frame.Line
		}
		if !more {
			return "", 0
		}
	}
}

// Debug logs a debug message (only if level <= debug)
func (l *Logger) Debug(format string, args ...any) {
	if l.logger == nil || logLevel > LevelDebug {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if IsJSONFormat() {
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
	if IsJSONFormat() {
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
	if IsJSONFormat() {
		l.logJSON("warn", msg)
	} else {
		l.logger.Printf("[WARN] %s", msg)
	}
	mirrorToProblems(l.category, "WARN", msg)
}

// Error logs an error message (always logged if logger exists)
func (l *Logger) Error(format string, args ...any) {
	if l.logger == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	if IsJSONFormat() {
		l.logJSON("error", msg)
	} else {
		l.logger.Printf("[ERROR] %s", msg)
	}
	mirrorToProblems(l.category, "ERROR", msg)
}

// --- Aggregated problems log -------------------------------------------------
//
// Every WARN and ERROR is mirrored, in addition to its own category file, into
// a single <run>_problems.log. Diagnosing a run otherwise means grepping ~25
// category files and manually interleaving them by timestamp, which is how a
// cold start managed to report success while 195 of 196 LLM calls were failing:
// the evidence existed, just nowhere anyone would look.
//
// This is a mirror, never a move — category files keep their own WARN/ERROR
// lines so nothing that reads them today changes.
var (
	problemsMu     sync.Mutex
	problemsLogger *log.Logger
	problemsFile   *rotatingFile
	problemsFailed bool
)

// mirrorToProblems appends one line to the aggregated problems log, tagged with
// the category it came from. Failures are silent after the first: logging must
// never take down the process it is observing.
func mirrorToProblems(category Category, level, msg string) {
	problemsMu.Lock()
	defer problemsMu.Unlock()

	if problemsFailed {
		return
	}
	if problemsLogger == nil {
		dir := logsDir
		if dir == "" {
			return // Initialize() has not run yet; category logger is a no-op too.
		}
		prefix := currentRunPrefix()
		if prefix == "" {
			prefix = time.Now().Format("2006-01-02")
		}
		path := filepath.Join(dir, fmt.Sprintf("%s_problems.log", prefix))
		f, err := openRotatingFile(path)
		if err != nil {
			problemsFailed = true
			fmt.Fprintf(os.Stderr, "[logging] could not open problems log %s: %v\n", path, err)
			return
		}
		problemsFile = f
		problemsLogger = log.New(f, "", log.Ldate|log.Ltime|log.Lmicroseconds)
	}
	problemsLogger.Printf("[%s] [%s] %s", level, category, msg)
}

// closeProblemsLog releases the aggregated log file. Safe to call repeatedly.
func closeProblemsLog() {
	problemsMu.Lock()
	defer problemsMu.Unlock()
	if problemsFile != nil {
		_ = problemsFile.Close()
		problemsFile = nil
		problemsLogger = nil
		problemsFailed = false // a fresh workspace/rebind deserves a fresh attempt
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
	if IsJSONFormat() {
		entry.File, entry.Line = callerSite()
		data, err := json.Marshal(entry)
		if err == nil {
			l.logger.Printf("%s", data)
			return
		}
	}
	// Fallback to text format with fields
	l.logger.Printf("[%s] %s | fields=%v", level, msg, fields)
}

// IsJSONFormat returns whether structured JSON logging is enabled, honouring
// both the canonical `format: "json"` and the legacy `json_format: true`.
func IsJSONFormat() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return jsonFormatEnabledLocked()
}

// jsonFormatEnabledLocked is the single place the two schema spellings are
// reconciled. Caller holds configMu (read or write).
func jsonFormatEnabledLocked() bool {
	return config.JSONFormat || strings.EqualFold(strings.TrimSpace(config.Format), "json")
}

// rawLLMTraceEnabled reports whether the operator opted out of LLM I/O
// redaction for this run.
func rawLLMTraceEnabled() bool {
	configMu.RLock()
	defer configMu.RUnlock()
	return config.TraceLLMIORaw
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

// emit writes one line for a Context/Request logger, honouring json_format.
//
// The two decorated loggers used to hardcode text output, so switching the
// package to JSON produced a file that was *mostly* parseable — every plain
// logger line was an object and every request-scoped or context-scoped line was
// `[INFO] msg | ctx=map[...]`. Anything consuming the file as JSONL (the Mangle
// fact path this format exists for) silently dropped exactly the lines that
// carry correlation IDs. Structured mode now carries the context as fields
// instead of stringifying it into the message.
func emit(l *Logger, level, levelTag, msg, requestID string, fields map[string]any) string {
	suffix := ""
	switch {
	case requestID != "" && len(fields) > 0:
		suffix = fmt.Sprintf("[req:%s] %s | %v", requestID, msg, fields)
	case requestID != "":
		suffix = fmt.Sprintf("[req:%s] %s", requestID, msg)
	default:
		suffix = fmt.Sprintf("%s | ctx=%v", msg, fields)
	}

	if IsJSONFormat() {
		entry := StructuredLogEntry{
			Timestamp: time.Now().UnixMilli(),
			Category:  string(l.category),
			Level:     level,
			Message:   msg,
			RequestID: requestID,
			Fields:    fields,
		}
		entry.File, entry.Line = callerSite()
		if data, err := json.Marshal(entry); err == nil {
			l.logger.Printf("%s", data)
			return suffix
		}
	}
	l.logger.Printf("[%s] %s", levelTag, suffix)
	return suffix
}

func (c *ContextLogger) Debug(format string, args ...any) {
	if c.logger.logger == nil || logLevel > LevelDebug {
		return
	}
	emit(c.logger, "debug", "DEBUG", fmt.Sprintf(format, args...), "", c.context)
}

func (c *ContextLogger) Info(format string, args ...any) {
	if c.logger.logger == nil || logLevel > LevelInfo {
		return
	}
	emit(c.logger, "info", "INFO", fmt.Sprintf(format, args...), "", c.context)
}

func (c *ContextLogger) Warn(format string, args ...any) {
	if c.logger.logger == nil || logLevel > LevelWarn {
		return
	}
	line := emit(c.logger, "warn", "WARN", fmt.Sprintf(format, args...), "", c.context)
	mirrorToProblems(c.logger.category, "WARN", line)
}

func (c *ContextLogger) Error(format string, args ...any) {
	if c.logger.logger == nil {
		return
	}
	line := emit(c.logger, "error", "ERROR", fmt.Sprintf(format, args...), "", c.context)
	mirrorToProblems(c.logger.category, "ERROR", line)
}

// CloseAll closes every sink this package owns: category loggers, the
// aggregated problems log, the audit log, and the LLM I/O trace.
//
// It used to close only the category loggers, so the obvious shutdown call
// leaked two of three sinks — the audit log in particular was left with buffered
// writes and an open handle, which on Windows also blocked the next run's
// fresh-run cleanup from reclaiming the name. Callers that want finer control
// still have CloseAudit and CloseLLMIOLogger; both are idempotent.
func CloseAll() {
	closeAllSinks()
}

// closeAllSinks is CloseAll's body, split out so Initialize can reuse it during
// a workspace rebind without the public function's documented semantics.
func closeAllSinks() {
	loggersMu.Lock()
	for _, l := range loggers {
		if l.sink != nil {
			_ = l.sink.Close()
		}
	}
	loggers = make(map[Category]*Logger)
	loggersMu.Unlock()

	closeProblemsLog()
	CloseAudit()
	CloseLLMIOLogger()
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
	emit(r.logger, "debug", "DEBUG", fmt.Sprintf(format, args...), r.requestID, r.fields)
}

func (r *RequestLogger) Info(format string, args ...any) {
	if r.logger.logger == nil || logLevel > LevelInfo {
		return
	}
	emit(r.logger, "info", "INFO", fmt.Sprintf(format, args...), r.requestID, r.fields)
}

// Warn mirrors to the problems log like every other WARN in the package. The
// request-scoped logger was the one path that did not, which would have made a
// correlated failure the single kind of failure invisible in the one file
// triage actually reads.
func (r *RequestLogger) Warn(format string, args ...any) {
	if r.logger.logger == nil || logLevel > LevelWarn {
		return
	}
	line := emit(r.logger, "warn", "WARN", fmt.Sprintf(format, args...), r.requestID, r.fields)
	mirrorToProblems(r.logger.category, "WARN", line)
}

func (r *RequestLogger) Error(format string, args ...any) {
	if r.logger.logger == nil {
		return
	}
	line := emit(r.logger, "error", "ERROR", fmt.Sprintf(format, args...), r.requestID, r.fields)
	mirrorToProblems(r.logger.category, "ERROR", line)
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
