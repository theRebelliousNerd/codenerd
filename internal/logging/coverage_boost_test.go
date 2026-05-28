package logging

import (
	"bytes"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// Initialize edge cases
// =============================================================================

func TestInitialize_WhenEmptyWorkspace_ShouldReturnError(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	err := Initialize("")
	if err == nil {
		t.Error("expected error for empty workspace")
	}
	if !strings.Contains(err.Error(), "workspace path required") {
		t.Errorf("expected 'workspace path required', got: %v", err)
	}
}

// =============================================================================
// IsJSONFormat
// =============================================================================

func TestIsJSONFormat_WhenNotSet_ShouldReturnFalse(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	if IsJSONFormat() {
		t.Error("expected JSONFormat to be false by default")
	}
}

func TestIsJSONFormat_WhenEnabled_ShouldReturnTrue(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	config.JSONFormat = true
	configMu.Unlock()

	if !IsJSONFormat() {
		t.Error("expected JSONFormat to be true")
	}
}

// =============================================================================
// ReloadConfig
// =============================================================================

func TestReloadConfig_WhenNoConfigFile_ShouldNotError(t *testing.T) {
	tmpDir := t.TempDir()
	resetLoggingState(t)
	defer resetLoggingState(t)

	workspace = tmpDir

	// No config file exists — should set defaults
	err := ReloadConfig()
	if err != nil {
		t.Errorf("ReloadConfig without config should not error: %v", err)
	}
}

func TestReloadConfig_WhenValidConfig_ShouldUpdateLevel(t *testing.T) {
	tmpDir := t.TempDir()
	resetLoggingState(t)
	defer resetLoggingState(t)

	workspace = tmpDir
	configDir := filepath.Join(tmpDir, ".nerd")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
		"logging": {"level": "error", "debug_mode": true}
	}`), 0644)

	err := ReloadConfig()
	if err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	if logLevel != LevelError {
		t.Errorf("expected logLevel=%d (error), got %d", LevelError, logLevel)
	}
}

func TestReloadConfig_WhenWarnLevel_ShouldSetLevelWarn(t *testing.T) {
	tmpDir := t.TempDir()
	resetLoggingState(t)
	defer resetLoggingState(t)

	workspace = tmpDir
	configDir := filepath.Join(tmpDir, ".nerd")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
		"logging": {"level": "warn"}
	}`), 0644)

	err := ReloadConfig()
	if err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	if logLevel != LevelWarn {
		t.Errorf("expected logLevel=%d (warn), got %d", LevelWarn, logLevel)
	}
}

func TestReloadConfig_WhenWarningLevel_ShouldSetLevelWarn(t *testing.T) {
	tmpDir := t.TempDir()
	resetLoggingState(t)
	defer resetLoggingState(t)

	workspace = tmpDir
	configDir := filepath.Join(tmpDir, ".nerd")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
		"logging": {"level": "warning"}
	}`), 0644)

	err := ReloadConfig()
	if err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	if logLevel != LevelWarn {
		t.Errorf("expected logLevel=%d (warn), got %d", LevelWarn, logLevel)
	}
}

func TestReloadConfig_WhenDebugLevel_ShouldSetLevelDebug(t *testing.T) {
	tmpDir := t.TempDir()
	resetLoggingState(t)
	defer resetLoggingState(t)

	workspace = tmpDir
	configDir := filepath.Join(tmpDir, ".nerd")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
		"logging": {"level": "debug"}
	}`), 0644)

	err := ReloadConfig()
	if err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	if logLevel != LevelDebug {
		t.Errorf("expected logLevel=%d (debug), got %d", LevelDebug, logLevel)
	}
}

func TestReloadConfig_WhenUnknownLevel_ShouldDefaultToInfo(t *testing.T) {
	tmpDir := t.TempDir()
	resetLoggingState(t)
	defer resetLoggingState(t)

	workspace = tmpDir
	configDir := filepath.Join(tmpDir, ".nerd")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
		"logging": {"level": "banana"}
	}`), 0644)

	err := ReloadConfig()
	if err != nil {
		t.Fatalf("ReloadConfig failed: %v", err)
	}
	if logLevel != LevelInfo {
		t.Errorf("expected logLevel=%d (info default), got %d", LevelInfo, logLevel)
	}
}

func TestReloadConfig_WhenInvalidJSON_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	resetLoggingState(t)
	defer resetLoggingState(t)

	workspace = tmpDir
	configDir := filepath.Join(tmpDir, ".nerd")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{INVALID`), 0644)

	err := ReloadConfig()
	if err == nil {
		t.Error("expected error for invalid JSON config")
	}
}

// =============================================================================
// shouldLogLevel
// =============================================================================

func TestShouldLogLevel_WhenVariousLevels_ShouldRespectThreshold(t *testing.T) {
	originalLevel := logLevel
	defer func() { logLevel = originalLevel }()

	tests := []struct {
		name      string
		setLevel  int
		queryLvl  string
		wantAllow bool
	}{
		{"debug at debug", LevelDebug, "debug", true},
		{"info at debug", LevelDebug, "info", true},
		{"warn at debug", LevelDebug, "warn", true},
		{"error at debug", LevelDebug, "error", true},
		{"debug at info", LevelInfo, "debug", false},
		{"info at info", LevelInfo, "info", true},
		{"warn at info", LevelInfo, "warn", true},
		{"error at info", LevelInfo, "error", true},
		{"debug at warn", LevelWarn, "debug", false},
		{"info at warn", LevelWarn, "info", false},
		{"warn at warn", LevelWarn, "warn", true},
		{"error at warn", LevelWarn, "error", true},
		{"debug at error", LevelError, "debug", false},
		{"info at error", LevelError, "info", false},
		{"warn at error", LevelError, "warn", false},
		{"error at error", LevelError, "error", true},
		{"unknown defaults to info", LevelInfo, "banana", true},
		{"unknown blocked at warn", LevelWarn, "banana", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			logLevel = tt.setLevel
			got := shouldLogLevel(tt.queryLvl)
			if got != tt.wantAllow {
				t.Errorf("shouldLogLevel(%q) at level %d = %v, want %v",
					tt.queryLvl, tt.setLevel, got, tt.wantAllow)
			}
		})
	}
}

// =============================================================================
// performanceSamplingRate
// =============================================================================

func TestPerformanceSamplingRate_WhenNotLoaded_ShouldReturn1(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	rate := performanceSamplingRate()
	if rate != 1.0 {
		t.Errorf("expected 1.0, got %v", rate)
	}
}

func TestPerformanceSamplingRate_WhenZero_ShouldReturn1(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	configLoaded = true
	config.PerformanceSampling = 0
	configMu.Unlock()

	rate := performanceSamplingRate()
	if rate != 1.0 {
		t.Errorf("expected 1.0 for zero sampling, got %v", rate)
	}
}

func TestPerformanceSamplingRate_WhenNegative_ShouldReturn1(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	configLoaded = true
	config.PerformanceSampling = -0.5
	configMu.Unlock()

	rate := performanceSamplingRate()
	if rate != 1.0 {
		t.Errorf("expected 1.0 for negative sampling, got %v", rate)
	}
}

func TestPerformanceSamplingRate_WhenGreaterThan1_ShouldReturn1(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	configLoaded = true
	config.PerformanceSampling = 2.0
	configMu.Unlock()

	rate := performanceSamplingRate()
	if rate != 1.0 {
		t.Errorf("expected 1.0 for >1 sampling, got %v", rate)
	}
}

func TestPerformanceSamplingRate_WhenValid_ShouldReturnValue(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	configLoaded = true
	config.PerformanceSampling = 0.5
	configMu.Unlock()

	rate := performanceSamplingRate()
	if rate != 0.5 {
		t.Errorf("expected 0.5, got %v", rate)
	}
}

// =============================================================================
// performanceThresholdMs
// =============================================================================

func TestPerformanceThresholdMs_WhenNotLoaded_ShouldReturn0(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	threshold := performanceThresholdMs(CategoryKernel)
	if threshold != 0 {
		t.Errorf("expected 0, got %d", threshold)
	}
}

func TestPerformanceThresholdMs_WhenCategorySpecific_ShouldReturnIt(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	configLoaded = true
	config.PerformanceThresholdsMs = map[string]int64{
		"kernel":  500,
		"default": 100,
	}
	configMu.Unlock()

	threshold := performanceThresholdMs(CategoryKernel)
	if threshold != 500 {
		t.Errorf("expected 500 for kernel, got %d", threshold)
	}
}

func TestPerformanceThresholdMs_WhenNoCategoryUsesDefault(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	configLoaded = true
	config.PerformanceThresholdsMs = map[string]int64{
		"default": 200,
	}
	configMu.Unlock()

	threshold := performanceThresholdMs(CategoryBoot)
	if threshold != 200 {
		t.Errorf("expected 200 (default), got %d", threshold)
	}
}

func TestPerformanceThresholdMs_WhenNilMap_ShouldReturn0(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	configLoaded = true
	config.PerformanceThresholdsMs = nil
	configMu.Unlock()

	threshold := performanceThresholdMs(CategoryKernel)
	if threshold != 0 {
		t.Errorf("expected 0 for nil map, got %d", threshold)
	}
}

// =============================================================================
// ContextLogger tests
// =============================================================================

func TestWithContext_ShouldReturnNonNilContextLogger(t *testing.T) {
	logger := &Logger{category: CategoryKernel}
	ctx := map[string]any{"key": "value"}
	cl := logger.WithContext(ctx)
	if cl == nil {
		t.Fatal("WithContext returned nil")
	}
	if cl.logger != logger {
		t.Error("context logger should reference original logger")
	}
}

func TestContextLogger_WhenNilInternalLogger_ShouldNotPanic(t *testing.T) {
	// No-op logger (nil internal logger)
	logger := &Logger{category: CategoryKernel}
	cl := logger.WithContext(map[string]any{"key": "value"})

	// All methods should be no-ops (no panic)
	cl.Debug("debug msg %d", 1)
	cl.Info("info msg %d", 2)
	cl.Warn("warn msg %d", 3)
	cl.Error("error msg %d", 4)
}

func TestContextLogger_WhenActiveLogger_ShouldWriteMessages(t *testing.T) {
	var buf bytes.Buffer
	inner := log.New(&buf, "", 0)
	logger := &Logger{category: CategoryKernel, logger: inner}

	originalLevel := logLevel
	logLevel = LevelDebug
	defer func() { logLevel = originalLevel }()

	cl := logger.WithContext(map[string]any{"req_id": "abc"})

	cl.Debug("test debug %s", "msg")
	cl.Info("test info %s", "msg")
	cl.Warn("test warn %s", "msg")
	cl.Error("test error %s", "msg")

	output := buf.String()
	if !strings.Contains(output, "[DEBUG]") {
		t.Error("expected DEBUG in output")
	}
	if !strings.Contains(output, "[INFO]") {
		t.Error("expected INFO in output")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Error("expected WARN in output")
	}
	if !strings.Contains(output, "[ERROR]") {
		t.Error("expected ERROR in output")
	}
	if !strings.Contains(output, "req_id") {
		t.Error("expected context key in output")
	}
}

func TestContextLogger_WhenLevelRestricted_ShouldFilter(t *testing.T) {
	var buf bytes.Buffer
	inner := log.New(&buf, "", 0)
	logger := &Logger{category: CategoryKernel, logger: inner}

	originalLevel := logLevel
	logLevel = LevelWarn
	defer func() { logLevel = originalLevel }()

	cl := logger.WithContext(map[string]any{})

	cl.Debug("should not appear")
	cl.Info("should not appear")
	cl.Warn("should appear")
	cl.Error("should also appear")

	output := buf.String()
	if strings.Contains(output, "[DEBUG]") {
		t.Error("DEBUG should not appear at warn level")
	}
	if strings.Contains(output, "[INFO]") {
		t.Error("INFO should not appear at warn level")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Error("WARN should appear")
	}
	if !strings.Contains(output, "[ERROR]") {
		t.Error("ERROR should appear")
	}
}

// =============================================================================
// RequestLogger tests
// =============================================================================

func TestWithRequestID_ShouldReturnNonNil(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	rl := WithRequestID(CategoryKernel, "req-123")
	if rl == nil {
		t.Fatal("WithRequestID returned nil")
	}
	if rl.requestID != "req-123" {
		t.Errorf("expected requestID='req-123', got %q", rl.requestID)
	}
}

func TestRequestLogger_WithField_ShouldChain(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	rl := WithRequestID(CategoryKernel, "req-1")
	result := rl.WithField("key", "value")
	if result != rl {
		t.Error("WithField should return the same RequestLogger for chaining")
	}
	if rl.fields["key"] != "value" {
		t.Errorf("expected field 'key'='value', got %v", rl.fields["key"])
	}
}

func TestRequestLogger_FormatMsg_WhenNoFields_ShouldIncludeRequestID(t *testing.T) {
	rl := &RequestLogger{
		logger:    &Logger{category: CategoryKernel},
		requestID: "req-abc",
		fields:    make(map[string]any),
	}
	msg := rl.formatMsg("hello %s", "world")
	if !strings.Contains(msg, "[req:req-abc]") {
		t.Errorf("expected request ID in message, got: %q", msg)
	}
	if !strings.Contains(msg, "hello world") {
		t.Errorf("expected formatted message, got: %q", msg)
	}
}

func TestRequestLogger_FormatMsg_WhenFields_ShouldIncludeFields(t *testing.T) {
	rl := &RequestLogger{
		logger:    &Logger{category: CategoryKernel},
		requestID: "req-xyz",
		fields:    map[string]any{"op": "read"},
	}
	msg := rl.formatMsg("test %d", 42)
	if !strings.Contains(msg, "[req:req-xyz]") {
		t.Errorf("expected request ID, got: %q", msg)
	}
	if !strings.Contains(msg, "test 42") {
		t.Errorf("expected formatted message, got: %q", msg)
	}
	// Fields should be included
	if !strings.Contains(msg, "op") {
		t.Errorf("expected field key in message, got: %q", msg)
	}
}

func TestRequestLogger_WhenNilInternalLogger_ShouldNotPanic(t *testing.T) {
	rl := &RequestLogger{
		logger:    &Logger{category: CategoryKernel}, // nil internal logger
		requestID: "req-1",
		fields:    make(map[string]any),
	}

	// All should be no-ops
	rl.Debug("test %s", "msg")
	rl.Info("test %s", "msg")
	rl.Warn("test %s", "msg")
	rl.Error("test %s", "msg")
}

func TestRequestLogger_WhenActiveLogger_ShouldWriteAllLevels(t *testing.T) {
	var buf bytes.Buffer
	inner := log.New(&buf, "", 0)

	originalLevel := logLevel
	logLevel = LevelDebug
	defer func() { logLevel = originalLevel }()

	rl := &RequestLogger{
		logger:    &Logger{category: CategoryKernel, logger: inner},
		requestID: "req-test",
		fields:    make(map[string]any),
	}

	rl.Debug("debug msg")
	rl.Info("info msg")
	rl.Warn("warn msg")
	rl.Error("error msg")

	output := buf.String()
	if !strings.Contains(output, "[DEBUG]") {
		t.Error("expected DEBUG")
	}
	if !strings.Contains(output, "[INFO]") {
		t.Error("expected INFO")
	}
	if !strings.Contains(output, "[WARN]") {
		t.Error("expected WARN")
	}
	if !strings.Contains(output, "[ERROR]") {
		t.Error("expected ERROR")
	}
	if !strings.Contains(output, "req-test") {
		t.Error("expected request ID in output")
	}
}

func TestRequestLogger_WhenLevelRestricted_ShouldFilter(t *testing.T) {
	var buf bytes.Buffer
	inner := log.New(&buf, "", 0)

	originalLevel := logLevel
	logLevel = LevelError
	defer func() { logLevel = originalLevel }()

	rl := &RequestLogger{
		logger:    &Logger{category: CategoryKernel, logger: inner},
		requestID: "req-filter",
		fields:    make(map[string]any),
	}

	rl.Debug("no")
	rl.Info("no")
	rl.Warn("no")
	rl.Error("yes")

	output := buf.String()
	if strings.Contains(output, "[DEBUG]") || strings.Contains(output, "[INFO]") || strings.Contains(output, "[WARN]") {
		t.Error("only ERROR should appear at error level")
	}
	if !strings.Contains(output, "[ERROR]") {
		t.Error("ERROR should appear")
	}
}

// =============================================================================
// Logger JSON mode tests
// =============================================================================

func TestLogger_WhenJSONFormat_ShouldWriteJSON(t *testing.T) {
	var buf bytes.Buffer
	inner := log.New(&buf, "", 0)
	logger := &Logger{category: CategoryKernel, logger: inner}

	originalLevel := logLevel
	logLevel = LevelDebug
	defer func() { logLevel = originalLevel }()

	// Enable JSON format
	configMu.Lock()
	origJSON := config.JSONFormat
	config.JSONFormat = true
	configMu.Unlock()
	defer func() {
		configMu.Lock()
		config.JSONFormat = origJSON
		configMu.Unlock()
	}()

	logger.Debug("json debug")
	logger.Info("json info")
	logger.Warn("json warn")
	logger.Error("json error")

	output := buf.String()
	// JSON output should contain the json keys
	if !strings.Contains(output, `"lvl"`) {
		t.Error("expected JSON level key in output")
	}
	if !strings.Contains(output, `"msg"`) {
		t.Error("expected JSON message key in output")
	}
}

// =============================================================================
// StructuredLog tests
// =============================================================================

func TestStructuredLog_WhenNilLogger_ShouldNotPanic(t *testing.T) {
	logger := &Logger{category: CategoryKernel}
	logger.StructuredLog("info", "test", map[string]any{"key": "val"})
}

func TestStructuredLog_WhenJSONFormat_ShouldOutputJSON(t *testing.T) {
	var buf bytes.Buffer
	inner := log.New(&buf, "", 0)
	logger := &Logger{category: CategoryKernel, logger: inner}

	configMu.Lock()
	origJSON := config.JSONFormat
	config.JSONFormat = true
	configMu.Unlock()
	defer func() {
		configMu.Lock()
		config.JSONFormat = origJSON
		configMu.Unlock()
	}()

	logger.StructuredLog("info", "test message", map[string]any{"op": "test"})

	output := buf.String()
	if !strings.Contains(output, `"msg"`) {
		t.Error("expected JSON msg key")
	}
	if !strings.Contains(output, "test message") {
		t.Error("expected message content")
	}
}

func TestStructuredLog_WhenTextFormat_ShouldOutputText(t *testing.T) {
	var buf bytes.Buffer
	inner := log.New(&buf, "", 0)
	logger := &Logger{category: CategoryKernel, logger: inner}

	configMu.Lock()
	origJSON := config.JSONFormat
	config.JSONFormat = false
	configMu.Unlock()
	defer func() {
		configMu.Lock()
		config.JSONFormat = origJSON
		configMu.Unlock()
	}()

	logger.StructuredLog("warn", "fallback test", map[string]any{"k": "v"})

	output := buf.String()
	if !strings.Contains(output, "[warn]") {
		t.Error("expected text-format level bracket")
	}
	if !strings.Contains(output, "fallback test") {
		t.Error("expected message in text output")
	}
}

// =============================================================================
// Timer variant tests
// =============================================================================

func TestTimer_StopWithInfo_ShouldReturnPositiveDuration(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	timer := StartTimer(CategoryKernel, "test_info_op")
	elapsed := timer.StopWithInfo()
	if elapsed < 0 {
		t.Errorf("elapsed should be >= 0, got %v", elapsed)
	}
}

func TestTimer_StopWithThreshold_WhenBelowThreshold_ShouldReturnDuration(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	timer := StartTimer(CategoryKernel, "fast_op")
	elapsed := timer.StopWithThreshold(10 * time.Second)
	if elapsed < 0 {
		t.Errorf("elapsed should be >= 0, got %v", elapsed)
	}
}

func TestTimer_StopWithThreshold_WhenAboveThreshold_ShouldReturnDuration(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	// Create a timer with an artificially old start time
	timer := &Timer{
		category: CategoryKernel,
		op:       "slow_op",
		start:    time.Now().Add(-5 * time.Second),
	}
	elapsed := timer.StopWithThreshold(1 * time.Millisecond)
	if elapsed < 1*time.Millisecond {
		t.Errorf("elapsed should exceed threshold, got %v", elapsed)
	}
}

// =============================================================================
// Convenience function Warn/Error/Debug variants (no-op path)
// These exercise the convenience wrappers that delegate to Get().Warn/Error/Debug.
// When not initialized, these are all no-ops — the test ensures no panics
// and covers the function call chains.
// =============================================================================

func TestConvenienceWarnError_WhenNotInitialized_ShouldNotPanic(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	// Debug variants
	BootDebug("test %d", 1)
	SessionDebug("test %d", 2)
	KernelDebug("test %d", 3)
	APIDebug("test %d", 4)
	PerceptionDebug("test %d", 5)
	ArticulationDebug("test %d", 6)
	RoutingDebug("test %d", 7)
	ToolsDebug("test %d", 8)
	VirtualStoreDebug("test %d", 9)
	ShardsDebug("test %d", 10)
	CoderDebug("test %d", 11)
	TesterDebug("test %d", 12)
	ReviewerDebug("test %d", 13)
	ResearcherDebug("test %d", 14)
	SystemShardsDebug("test %d", 15)
	DreamDebug("test %d", 16)
	AutopoiesisDebug("test %d", 17)
	CampaignDebug("test %d", 18)
	ContextDebug("test %d", 19)
	WorldDebug("test %d", 20)
	EmbeddingDebug("test %d", 21)
	StoreDebug("test %d", 22)

	// Warn variants
	BootWarn("test %d", 1)
	SessionWarn("test %d", 2)
	KernelWarn("test %d", 3)
	APIWarn("test %d", 4)
	PerceptionWarn("test %d", 5)
	ArticulationWarn("test %d", 6)
	RoutingWarn("test %d", 7)
	ToolsWarn("test %d", 8)
	VirtualStoreWarn("test %d", 9)
	ShardsWarn("test %d", 10)
	CoderWarn("test %d", 11)
	TesterWarn("test %d", 12)
	ReviewerWarn("test %d", 13)
	ResearcherWarn("test %d", 14)
	SystemShardsWarn("test %d", 15)
	DreamWarn("test %d", 16)
	AutopoiesisWarn("test %d", 17)
	CampaignWarn("test %d", 18)
	ContextWarn("test %d", 19)
	WorldWarn("test %d", 20)
	EmbeddingWarn("test %d", 21)
	StoreWarn("test %d", 22)

	// Error variants
	BootError("test %d", 1)
	SessionError("test %d", 2)
	KernelError("test %d", 3)
	APIError("test %d", 4)
	PerceptionError("test %d", 5)
	ArticulationError("test %d", 6)
	RoutingError("test %d", 7)
	ToolsError("test %d", 8)
	VirtualStoreError("test %d", 9)
	ShardsError("test %d", 10)
	CoderError("test %d", 11)
	TesterError("test %d", 12)
	ReviewerError("test %d", 13)
	ResearcherError("test %d", 14)
	SystemShardsError("test %d", 15)
	DreamError("test %d", 16)
	AutopoiesisError("test %d", 17)
	CampaignError("test %d", 18)
	ContextError("test %d", 19)
	WorldError("test %d", 20)
	EmbeddingError("test %d", 21)
	StoreError("test %d", 22)

	// Browser-specific variants
	BrowserWarn("test %d", 1)
	BrowserError("test %d", 2)
	BrowserDebug("test %d", 3)
	Browser("test %d", 4)

	// Tactile-specific variants
	TactileWarn("test %d", 1)
	TactileError("test %d", 2)
	TactileDebug("test %d", 3)
	Tactile("test %d", 4)

	// JIT-specific variants
	JITWarn("test %d", 1)
	JITError("test %d", 2)
	JITDebug("test %d", 3)
	JIT("test %d", 4)

	// Build-specific variants
	BuildWarn("test %d", 1)
	BuildError("test %d", 2)
	BuildDebug("test %d", 3)
	Build("test %d", 4)
}

func TestConvenienceWarnError_WhenDebugEnabled_ShouldNotPanic(t *testing.T) {
	tmpDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	if err := Initialize(tmpDir); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	defer resetLoggingState(t)

	// Warn + Error functions with active loggers
	BootWarn("warn %d", 1)
	BootError("error %d", 1)
	SessionWarn("warn %d", 2)
	SessionError("error %d", 2)
	KernelWarn("warn %d", 3)
	KernelError("error %d", 3)
	APIWarn("warn %d", 4)
	APIError("error %d", 4)
	PerceptionWarn("warn %d", 5)
	PerceptionError("error %d", 5)
	ArticulationWarn("warn %d", 6)
	ArticulationError("error %d", 6)
	RoutingWarn("warn %d", 7)
	RoutingError("error %d", 7)
	ToolsWarn("warn %d", 8)
	ToolsError("error %d", 8)
	VirtualStoreWarn("warn %d", 9)
	VirtualStoreError("error %d", 9)
	ShardsWarn("warn %d", 10)
	ShardsError("error %d", 10)
	CoderWarn("warn %d", 11)
	CoderError("error %d", 11)
	TesterWarn("warn %d", 12)
	TesterError("error %d", 12)
	ReviewerWarn("warn %d", 13)
	ReviewerError("error %d", 13)
	ResearcherWarn("warn %d", 14)
	ResearcherError("error %d", 14)
	SystemShardsWarn("warn %d", 15)
	SystemShardsError("error %d", 15)
	DreamWarn("warn %d", 16)
	DreamError("error %d", 16)
	AutopoiesisWarn("warn %d", 17)
	AutopoiesisError("error %d", 17)
	CampaignWarn("warn %d", 18)
	CampaignError("error %d", 18)
	ContextWarn("warn %d", 19)
	ContextError("error %d", 19)
	WorldWarn("warn %d", 20)
	WorldError("error %d", 20)
	EmbeddingWarn("warn %d", 21)
	EmbeddingError("error %d", 21)
	StoreWarn("warn %d", 22)
	StoreError("error %d", 22)

	BrowserWarn("warn %d", 23)
	BrowserError("error %d", 23)
	TactileWarn("warn %d", 24)
	TactileError("error %d", 24)
	JITWarn("warn %d", 25)
	JITError("error %d", 25)
	BuildWarn("warn %d", 26)
	BuildError("error %d", 26)

	// Debug variants
	BootDebug("debug %d", 1)
	SessionDebug("debug %d", 2)
	KernelDebug("debug %d", 3)
	APIDebug("debug %d", 4)
	PerceptionDebug("debug %d", 5)
	ArticulationDebug("debug %d", 6)
	RoutingDebug("debug %d", 7)
	ToolsDebug("debug %d", 8)
	VirtualStoreDebug("debug %d", 9)
	ShardsDebug("debug %d", 10)
	CoderDebug("debug %d", 11)
	TesterDebug("debug %d", 12)
	ReviewerDebug("debug %d", 13)
	ResearcherDebug("debug %d", 14)
	SystemShardsDebug("debug %d", 15)
	DreamDebug("debug %d", 16)
	AutopoiesisDebug("debug %d", 17)
	CampaignDebug("debug %d", 18)
	ContextDebug("debug %d", 19)
	WorldDebug("debug %d", 20)
	EmbeddingDebug("debug %d", 21)
	StoreDebug("debug %d", 22)
	BrowserDebug("debug %d", 23)
	TactileDebug("debug %d", 24)
	JITDebug("debug %d", 25)
	BuildDebug("debug %d", 26)
}

// =============================================================================
// Northstar category (exists but may not have convenience func test)
// =============================================================================

func TestNorthstarCategory_ShouldExist(t *testing.T) {
	if CategoryNorthstar != "northstar" {
		t.Errorf("expected CategoryNorthstar='northstar', got %q", CategoryNorthstar)
	}
}

// =============================================================================
// LLM IO Logger — no-op paths (not initialized)
// =============================================================================

func TestIsLLMIOTracingEnabled_WhenNotInitialized_ShouldReturnFalse(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	// Reset llm io state
	llmIO = nil
	llmIOOnce = sync.Once{}

	// With no config/logs dir, tracing should be disabled
	enabled := IsLLMIOTracingEnabled()
	if enabled {
		t.Error("LLM IO tracing should be disabled when not initialized")
	}
}

func TestLogLLMRequest_WhenDisabled_ShouldNotPanic(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	llmIO = nil
	llmIOOnce = sync.Once{}

	LogLLMRequest("test-callsite", "system prompt", "user prompt",
		[]LLMMessage{{Role: "user", Content: "hello"}}, "gpt-4", 0.7)
}

func TestLogLLMResponse_WhenDisabled_ShouldNotPanic(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	llmIO = nil
	llmIOOnce = sync.Once{}

	LogLLMResponse("test-callsite", "response text", 100*time.Millisecond, 50)
}

func TestLogLLMError_WhenDisabled_ShouldNotPanic(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	llmIO = nil
	llmIOOnce = sync.Once{}

	LogLLMError("test-callsite", fmt.Errorf("test error"), 50*time.Millisecond)
}

func TestCloseLLMIOLogger_WhenNil_ShouldNotPanic(t *testing.T) {
	llmIO = nil
	CloseLLMIOLogger()
}

func TestCloseLLMIOLogger_WhenDisabledNoFile_ShouldNotPanic(t *testing.T) {
	llmIO = &llmIOLogger{enabled: false}
	CloseLLMIOLogger()
}

// =============================================================================
// Audit convenience methods (no-op when auditFile is nil)
// =============================================================================

func TestAuditLogger_ShardSpawn_WhenNoFile_ShouldNotPanic(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	a := Audit()
	a.ShardSpawn("coder-1", "TypeA")
}

func TestAuditLogger_ShardExecute_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{shardID: "shard-1"}
	a.ShardExecute("shard-1", "generate code")
}

func TestAuditLogger_ShardComplete_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.ShardComplete("shard-1", "task-1", 100, true, "")
	a.ShardComplete("shard-1", "task-2", 200, false, "some error")
}

func TestAuditLogger_ActionRoute_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.ActionRoute("read_file", "main.go")
}

func TestAuditLogger_ActionComplete_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.ActionComplete("write_file", "main.go", 50, true, "")
	a.ActionComplete("delete_file", "old.go", 10, false, "permission denied")
}

func TestAuditLogger_KernelAssert_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.KernelAssert("user_intent", 5)
}

func TestAuditLogger_KernelQuery_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.KernelQuery("next_action", 3, 15)
}

func TestAuditLogger_LLMCall_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.LLMCall("gpt-4", 500, 1500, true, "")
	a.LLMCall("gpt-4", 0, 100, false, "rate limited")
}

func TestAuditLogger_FileOp_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.FileOp(AuditFileRead, "main.go", 1024, true, "")
	a.FileOp(AuditFileWrite, "output.go", 2048, true, "")
	a.FileOp(AuditFileError, "missing.go", 0, false, "not found")
}

func TestAuditLogger_IntentParsed_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.IntentParsed("mutation", "refactor", "auth.go", 0.95)
}

func TestAuditLogger_SafetyCheck_WhenAllowed_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.SafetyCheck("read_file", true, "safe action")
}

func TestAuditLogger_SafetyCheck_WhenBlocked_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.SafetyCheck("rm -rf /", false, "dangerous")
}

func TestAuditLogger_PerfMetric_WhenBelowThreshold_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.PerfMetric("rebuild", 50, 100)
}

func TestAuditLogger_PerfMetric_WhenAboveThreshold_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.PerfMetric("rebuild", 200, 100)
}

func TestAuditLogger_PerfMetric_WhenNoThreshold_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.PerfMetric("rebuild", 50, 0)
}

func TestAuditLogger_Error_WhenCritical_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.Error("kernel", fmt.Errorf("critical failure"), true)
}

func TestAuditLogger_Error_WhenNonCritical_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.Error("tools", fmt.Errorf("minor issue"), false)
}

func TestAuditLogger_Error_WhenNilError_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.Error("kernel", nil, false)
}

func TestAuditLogger_SessionStart_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.SessionStart("sess-abc")
}

func TestAuditLogger_SessionEnd_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.SessionEnd("sess-abc", 10, 5000)
}

func TestAuditLogger_TurnStart_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.TurnStart("sess-1", 1, 100)
}

func TestAuditLogger_TurnEnd_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.TurnEnd("sess-1", 1, 500, true)
	a.TurnEnd("sess-1", 2, 300, false)
}

func TestAuditLogger_ToolExec_WhenSuccess_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.ToolExec("file_reader", "read", 10, true, "")
}

func TestAuditLogger_ToolExec_WhenFailure_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.ToolExec("shell", "exec", 100, false, "command not found")
}

func TestAuditLogger_CampaignEvent_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.CampaignEvent(AuditCampaignStart, "campaign-1", "planning", true)
	a.CampaignEvent(AuditCampaignPhase, "campaign-1", "execution", true)
	a.CampaignEvent(AuditCampaignComplete, "campaign-1", "done", true)
	a.CampaignEvent(AuditCampaignAbort, "campaign-2", "failed", false)
}

func TestAuditLogger_LearningEvent_WhenNoFile_ShouldNotPanic(t *testing.T) {
	a := &AuditLogger{}
	a.LearningEvent(AuditLearningStart, "learner-1", "pattern-abc", true)
	a.LearningEvent(AuditLearningComplete, "learner-1", "pattern-abc", true)
	a.LearningEvent(AuditToolGenerated, "ouroboros-1", "new_tool", true)
}

// =============================================================================
// IsCategoryEnabled edge cases
// =============================================================================

func TestIsCategoryEnabled_WhenCategoriesNil_ShouldReturnTrue(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	config.DebugMode = true
	config.Categories = nil
	configMu.Unlock()

	if !IsCategoryEnabled(CategoryKernel) {
		t.Error("expected true when categories map is nil and debug is on")
	}
}

func TestIsCategoryEnabled_WhenCategoryExplicitlyFalse_ShouldReturnFalse(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	configMu.Lock()
	config.DebugMode = true
	config.Categories = map[string]bool{"kernel": false}
	configMu.Unlock()

	if IsCategoryEnabled(CategoryKernel) {
		t.Error("expected false when category is explicitly disabled")
	}
}
