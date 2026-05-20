package logging

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// =============================================================================
// Helper to reset logging state between tests
// =============================================================================

func resetLoggingState(t *testing.T) {
	t.Helper()
	CloseAll()
	CloseAudit()
	loggers = make(map[Category]*Logger)
	logsDir = ""
	workspace = ""
	configLoaded = false
	config = loggingConfig{}
	initOnce = sync.Once{}
	initErr = nil
	initialized = false
	auditLogger = nil
}

// setupDebugWorkspace creates a temp workspace with debug-mode logging config.
func setupDebugWorkspace(t *testing.T) string {
	t.Helper()
	tempDir, err := os.MkdirTemp("", "logging_comprehensive_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	t.Cleanup(func() { os.RemoveAll(tempDir) })

	configDir := filepath.Join(tempDir, ".nerd")
	if err := os.MkdirAll(configDir, 0755); err != nil {
		t.Fatalf("Failed to create config dir: %v", err)
	}

	configContent := `{
		"logging": {
			"level": "debug",
			"debug_mode": true,
			"categories": {
				"boot": true,
				"session": true,
				"kernel": true,
				"shards": true
			}
		}
	}`
	if err := os.WriteFile(filepath.Join(configDir, "config.json"), []byte(configContent), 0644); err != nil {
		t.Fatalf("Failed to write config: %v", err)
	}
	return tempDir
}

// =============================================================================
// StructuredLogEntry Tests
// =============================================================================

func TestStructuredLogEntry_WhenAllFields_ShouldMarshalCorrectly(t *testing.T) {
	entry := StructuredLogEntry{
		Level:    "INFO",
		Category: string(CategoryKernel),
		Message:  "test message",
		Fields: map[string]interface{}{
			"key": "value",
		},
	}

	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	result := string(data)
	if !strings.Contains(result, "test message") {
		t.Errorf("entry JSON should contain message, got: %q", result)
	}
	if !strings.Contains(result, "INFO") {
		t.Errorf("entry JSON should contain level, got: %q", result)
	}
}

func TestStructuredLogEntry_WhenEmptyMessage_ShouldMarshalWithoutError(t *testing.T) {
	entry := StructuredLogEntry{
		Level:    "WARN",
		Category: "test",
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty JSON even with empty message")
	}
}

func TestStructuredLogEntry_WhenNilFields_ShouldMarshalWithoutError(t *testing.T) {
	entry := StructuredLogEntry{
		Level:    "ERROR",
		Category: "test",
		Message:  "error occurred",
		Fields:   nil,
	}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatalf("json.Marshal error: %v", err)
	}
	result := string(data)
	if !strings.Contains(result, "error occurred") {
		t.Errorf("should contain message, got: %q", result)
	}
}

// =============================================================================
// Get Logger Tests
// =============================================================================

func TestGet_WhenDebugModeEnabled_ShouldReturnNonNilLogger(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	if err := Initialize(tempDir); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	defer resetLoggingState(t)

	logger := Get(CategoryBoot)
	if logger == nil {
		t.Fatal("Get(CategoryBoot) returned nil")
	}
}

func TestGet_WhenCalledMultipleTimes_ShouldReturnSameLogger(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	if err := Initialize(tempDir); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	defer resetLoggingState(t)

	l1 := Get(CategoryKernel)
	l2 := Get(CategoryKernel)
	if l1 != l2 {
		t.Error("Get should return the same logger instance for the same category")
	}
}

func TestGet_WhenDebugModeDisabled_ShouldReturnNoOpLogger(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logging_noop_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	configDir := filepath.Join(tempDir, ".nerd")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
		"logging": {"debug_mode": false}
	}`), 0644)

	resetLoggingState(t)
	if err := Initialize(tempDir); err != nil {
		t.Fatalf("Initialize error: %v", err)
	}
	defer resetLoggingState(t)

	logger := Get(CategoryBoot)
	if logger == nil {
		t.Fatal("Get should return a non-nil logger even when disabled")
	}

	// Logging should be a no-op (not panic)
	logger.Info("test %s", "message")
	logger.Debug("test %s", "message")
	logger.Warn("test %s", "message")
	logger.Error("test %s", "message")
}

// =============================================================================
// Initialize Tests
// =============================================================================

func TestInitialize_WhenValidWorkspace_ShouldSucceed(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	defer resetLoggingState(t)

	err := Initialize(tempDir)
	if err != nil {
		t.Fatalf("Initialize failed: %v", err)
	}
	if !initialized {
		t.Error("expected initialized=true after successful Initialize")
	}
}

func TestInitialize_WhenCalledTwice_ShouldBeIdempotent(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	defer resetLoggingState(t)

	err1 := Initialize(tempDir)
	if err1 != nil {
		t.Fatalf("First Initialize failed: %v", err1)
	}

	err2 := Initialize(tempDir)
	if err2 != nil {
		t.Fatalf("Second Initialize should not error: %v", err2)
	}
}

func TestInitialize_WhenNoConfigFile_ShouldUseDefaults(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "logging_noconfig_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	resetLoggingState(t)
	defer resetLoggingState(t)

	err = Initialize(tempDir)
	if err != nil {
		t.Fatalf("Initialize without config should not error: %v", err)
	}

	// Should default to production mode (no debug)
	if IsDebugMode() {
		t.Error("should default to production mode when no config exists")
	}
}

// =============================================================================
// IsDebugMode / IsCategoryEnabled Tests
// =============================================================================

func TestIsDebugMode_WhenEnabled_ShouldReturnTrue(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	Initialize(tempDir)
	defer resetLoggingState(t)

	if !IsDebugMode() {
		t.Error("expected debug mode to be enabled")
	}
}

func TestIsDebugMode_WhenDisabled_ShouldReturnFalse(t *testing.T) {
	tempDir, _ := os.MkdirTemp("", "logging_nodebug")
	defer os.RemoveAll(tempDir)

	configDir := filepath.Join(tempDir, ".nerd")
	os.MkdirAll(configDir, 0755)
	os.WriteFile(filepath.Join(configDir, "config.json"), []byte(`{
		"logging": {"debug_mode": false}
	}`), 0644)

	resetLoggingState(t)
	Initialize(tempDir)
	defer resetLoggingState(t)

	if IsDebugMode() {
		t.Error("expected debug mode to be disabled")
	}
}

func TestIsCategoryEnabled_WhenExplicitlyEnabled_ShouldReturnTrue(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	Initialize(tempDir)
	defer resetLoggingState(t)

	if !IsCategoryEnabled(CategoryBoot) {
		t.Error("boot should be enabled")
	}
	if !IsCategoryEnabled(CategoryKernel) {
		t.Error("kernel should be enabled")
	}
}

func TestIsCategoryEnabled_WhenNotInConfig_ShouldDefaultEnabled(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	Initialize(tempDir)
	defer resetLoggingState(t)

	// CategoryCoder is not in the config but should default to enabled in debug mode
	if !IsCategoryEnabled(CategoryCoder) {
		t.Error("coder (not in config) should default to enabled in debug mode")
	}
}

// =============================================================================
// Logger Log Level Methods
// =============================================================================

func TestLoggerLevels_WhenDebugEnabled_ShouldWriteMessages(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	Initialize(tempDir)
	defer resetLoggingState(t)

	logger := Get(CategoryBoot)

	// These should not panic
	logger.Info("info message %d", 1)
	logger.Debug("debug message %s", "test")
	logger.Warn("warn message %v", true)
	logger.Error("error message %v", fmt.Errorf("test error"))

	CloseAll()

	// Verify log file was created and contains messages
	logsPath := filepath.Join(tempDir, ".nerd", "logs")
	entries, err := os.ReadDir(logsPath)
	if err != nil {
		t.Fatalf("Failed to read logs dir: %v", err)
	}

	found := false
	for _, entry := range entries {
		if strings.Contains(entry.Name(), "boot") {
			content, err := os.ReadFile(filepath.Join(logsPath, entry.Name()))
			if err != nil {
				t.Fatalf("Failed to read boot log: %v", err)
			}
			contentStr := string(content)
			if !strings.Contains(contentStr, "info message 1") {
				t.Error("boot log should contain 'info message 1'")
			}
			if !strings.Contains(contentStr, "error message") {
				t.Error("boot log should contain 'error message'")
			}
			found = true
			break
		}
	}
	if !found {
		t.Error("expected to find boot log file")
	}
}

// =============================================================================
// Convenience Function Tests
// =============================================================================

func TestConvenienceFunctions_WhenDebugEnabled_ShouldNotPanic(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	Initialize(tempDir)
	defer resetLoggingState(t)

	// These should all not panic
	Boot("boot message %d", 1)
	Session("session message %d", 2)
	Kernel("kernel message %d", 3)
	API("api message %d", 4)
	Perception("perception message %d", 5)
	Articulation("articulation message %d", 6)
	Routing("routing message %d", 7)
	Tools("tools message %d", 8)
	VirtualStore("virtualstore message %d", 9)
	Shards("shards message %d", 10)
	Coder("coder message %d", 11)
	Tester("tester message %d", 12)
	Reviewer("reviewer message %d", 13)
	Researcher("researcher message %d", 14)
	SystemShards("systemshards message %d", 15)
	Dream("dream message %d", 16)
	Autopoiesis("autopoiesis message %d", 17)
	Campaign("campaign message %d", 18)
	Context("context message %d", 19)
	World("world message %d", 20)
	Embedding("embedding message %d", 21)
	Store("store message %d", 22)

	// If we get here without panic, test passes
}

func TestConvenienceFunctions_WhenNotInitialized_ShouldNotPanic(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	// These should be no-ops, not panics
	Boot("should not panic")
	Session("should not panic")
	Kernel("should not panic")
}

// =============================================================================
// Concurrent Write Safety Tests
// =============================================================================

func TestConcurrentWrites_WhenMultipleGoroutines_ShouldNotRace(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	Initialize(tempDir)
	defer resetLoggingState(t)

	var wg sync.WaitGroup
	categories := []Category{CategoryBoot, CategoryKernel, CategoryShards, CategorySession}

	for _, cat := range categories {
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(c Category, idx int) {
				defer wg.Done()
				logger := Get(c)
				logger.Info("concurrent message %d from %s", idx, c)
				logger.Debug("concurrent debug %d", idx)
				logger.Warn("concurrent warn %d", idx)
				logger.Error("concurrent error %d", idx)
			}(cat, i)
		}
	}
	wg.Wait()

	// If no race or deadlock, test passes
}

// =============================================================================
// EscapeString Tests
// =============================================================================

func TestEscapeString_WhenSpecialChars_ShouldEscape(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"empty", "", ""},
		{"no special", "hello world", "hello world"},
		{"quotes", `she said "hi"`, `she said \"hi\"`},
		{"backslash", `path\to\file`, `path\\to\\file`},
		{"newline", "line1\nline2", `line1\nline2`},
		{"carriage return", "line1\rline2", `line1\rline2`},
		{"tab", "col1\tcol2", `col1\tcol2`},
		{"mixed", "he said \"hi\"\nand left\\", `he said \"hi\"\nand left\\`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := escapeString(tt.input)
			if got != tt.want {
				t.Errorf("escapeString(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

// =============================================================================
// AuditLogger Constructor Tests
// =============================================================================

func TestAudit_ShouldReturnNonNil(t *testing.T) {
	resetLoggingState(t)
	defer resetLoggingState(t)

	a := Audit()
	if a == nil {
		t.Fatal("Audit() returned nil")
	}
}

func TestAuditWithSession_ShouldSetSessionID(t *testing.T) {
	a := AuditWithSession("sess-123")
	if a.sessionID != "sess-123" {
		t.Errorf("expected sessionID='sess-123', got %q", a.sessionID)
	}
}

func TestAuditWithShard_ShouldSetShardID(t *testing.T) {
	a := AuditWithShard("shard-abc")
	if a.shardID != "shard-abc" {
		t.Errorf("expected shardID='shard-abc', got %q", a.shardID)
	}
}

func TestAuditWithContext_ShouldSetAllFields(t *testing.T) {
	a := AuditWithContext("sess-1", "shard-1", CategoryKernel)
	if a.sessionID != "sess-1" {
		t.Errorf("sessionID = %q, want 'sess-1'", a.sessionID)
	}
	if a.shardID != "shard-1" {
		t.Errorf("shardID = %q, want 'shard-1'", a.shardID)
	}
	if a.category != CategoryKernel {
		t.Errorf("category = %q, want %q", a.category, CategoryKernel)
	}
}

// =============================================================================
// GenerateMangleFact Tests
// =============================================================================

func TestGenerateMangleFact_WhenShardEvent_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		EventType: AuditShardSpawn,
		Timestamp: 1000,
		ShardID:   "coder-1",
		Target:    "TypeA",
		Success:   true,
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "shard_lifecycle(") {
		t.Errorf("expected shard_lifecycle predicate, got: %q", fact)
	}
	if !strings.Contains(fact, "1000") {
		t.Error("should contain timestamp")
	}
	if !strings.Contains(fact, "coder-1") {
		t.Error("should contain shard ID")
	}
	if !strings.HasSuffix(fact, ".") {
		t.Error("Mangle fact should end with period")
	}
}

func TestGenerateMangleFact_WhenActionEvent_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		EventType:  AuditActionRoute,
		Timestamp:  2000,
		Action:     "read_file",
		Target:     "main.go",
		Success:    true,
		DurationMs: 50,
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "action_event(") {
		t.Errorf("expected action_event predicate, got: %q", fact)
	}
}

func TestGenerateMangleFact_WhenKernelEvent_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		EventType: AuditKernelAssert,
		Timestamp: 3000,
		Target:    "user_intent",
		Success:   true,
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "kernel_op(") {
		t.Errorf("expected kernel_op predicate, got: %q", fact)
	}
}

func TestGenerateMangleFact_WhenLLMEvent_ShouldIncludeTokens(t *testing.T) {
	event := AuditEvent{
		EventType:  AuditLLMResponse,
		Timestamp:  4000,
		ShardID:    "coder-1",
		Success:    true,
		DurationMs: 1500,
		Fields:     map[string]interface{}{"tokens": 500},
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "llm_call(") {
		t.Errorf("expected llm_call predicate, got: %q", fact)
	}
	if !strings.Contains(fact, "500") {
		t.Error("should contain token count")
	}
}

func TestGenerateMangleFact_WhenFileEvent_ShouldIncludeSize(t *testing.T) {
	event := AuditEvent{
		EventType: AuditFileWrite,
		Timestamp: 5000,
		Target:    "main.go",
		Success:   true,
		Fields:    map[string]interface{}{"size": int64(1024)},
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "file_op(") {
		t.Errorf("expected file_op predicate, got: %q", fact)
	}
	if !strings.Contains(fact, "1024") {
		t.Error("should contain file size")
	}
}

func TestGenerateMangleFact_WhenSessionEvent_ShouldContainSessionID(t *testing.T) {
	event := AuditEvent{
		EventType: AuditSessionStart,
		Timestamp: 6000,
		SessionID: "sess-abc",
		Success:   true,
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "session_event(") {
		t.Errorf("expected session_event predicate, got: %q", fact)
	}
	if !strings.Contains(fact, "sess-abc") {
		t.Error("should contain session ID")
	}
}

func TestGenerateMangleFact_WhenSafetyEvent_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		EventType: AuditSafetyBlock,
		Timestamp: 7000,
		Action:    "rm -rf /",
		Success:   false,
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "safety_check(") {
		t.Errorf("expected safety_check predicate, got: %q", fact)
	}
}

func TestGenerateMangleFact_WhenErrorEvent_ShouldEscapeMessage(t *testing.T) {
	event := AuditEvent{
		EventType: AuditErrorCritical,
		Timestamp: 8000,
		Category:  "kernel",
		Error:     `error with "quotes" and\backslash`,
		Success:   false,
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "error_event(") {
		t.Errorf("expected error_event predicate, got: %q", fact)
	}
}

func TestGenerateMangleFact_WhenCampaignEvent_ShouldIncludePhase(t *testing.T) {
	event := AuditEvent{
		EventType: AuditCampaignStart,
		Timestamp: 9000,
		SessionID: "campaign-1",
		Success:   true,
		Fields:    map[string]interface{}{"phase": "planning"},
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "campaign_event(") {
		t.Errorf("expected campaign_event predicate, got: %q", fact)
	}
	if !strings.Contains(fact, "planning") {
		t.Error("should contain phase")
	}
}

func TestGenerateMangleFact_WhenLearningEvent_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		EventType: AuditToolGenerated,
		Timestamp: 10000,
		ShardID:   "ouroboros-1",
		Target:    "new_tool",
		Success:   true,
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "learning_event(") {
		t.Errorf("expected learning_event predicate, got: %q", fact)
	}
}

func TestGenerateMangleFact_WhenUnknownEventType_ShouldFallbackToDefault(t *testing.T) {
	event := AuditEvent{
		EventType: AuditEventType("unknown_event"),
		Timestamp: 11000,
		Category:  "test",
		Message:   "test message",
		Success:   true,
	}

	fact := generateMangleFact(event)
	if !strings.Contains(fact, "audit_event(") {
		t.Errorf("expected audit_event fallback predicate, got: %q", fact)
	}
}

// =============================================================================
// Timer Tests
// =============================================================================

func TestStartTimer_WhenStopped_ShouldReturnPositiveDuration(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	Initialize(tempDir)
	defer resetLoggingState(t)

	timer := StartTimer(CategoryKernel, "test_op")
	// Note: not using time.Sleep per rules, just check that it returns
	elapsed := timer.Stop()
	if elapsed < 0 {
		t.Errorf("elapsed should be >= 0, got %v", elapsed)
	}
}

// =============================================================================
// LLMMessage struct test
// =============================================================================

func TestLLMMessage_ShouldHoldRoleAndContent(t *testing.T) {
	msg := LLMMessage{
		Role:    "user",
		Content: "hello",
	}
	if msg.Role != "user" {
		t.Errorf("Role = %q, want 'user'", msg.Role)
	}
	if msg.Content != "hello" {
		t.Errorf("Content = %q, want 'hello'", msg.Content)
	}
}

// =============================================================================
// CloseAll Tests
// =============================================================================

func TestCloseAll_WhenCalledMultipleTimes_ShouldNotPanic(t *testing.T) {
	tempDir := setupDebugWorkspace(t)
	resetLoggingState(t)
	Initialize(tempDir)

	CloseAll()
	CloseAll() // Second close should not panic
	CloseAudit()
	CloseAudit() // Second close should not panic
}

func TestCloseAll_WhenNeverInitialized_ShouldNotPanic(t *testing.T) {
	resetLoggingState(t)
	CloseAll()  // Should not panic
	CloseAudit() // Should not panic
}
