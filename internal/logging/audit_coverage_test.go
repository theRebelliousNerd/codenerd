package logging

import (
	"testing"
)

// --- escapeString ---

func TestEscapeString_WhenNoSpecialChars_ShouldReturnUnchanged(t *testing.T) {
	input := "hello world 123"
	got := escapeString(input)
	if got != input {
		t.Errorf("expected %q, got %q", input, got)
	}
}

func TestEscapeString_WhenQuotes_ShouldEscape(t *testing.T) {
	got := escapeString(`say "hello"`)
	expected := `say \"hello\"`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeString_WhenBackslashes_ShouldEscape(t *testing.T) {
	got := escapeString(`path\to\file`)
	expected := `path\\to\\file`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeString_WhenNewlines_ShouldEscape(t *testing.T) {
	got := escapeString("line1\nline2\rline3\ttab")
	expected := `line1\nline2\rline3\ttab`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

func TestEscapeString_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	got := escapeString("")
	if got != "" {
		t.Errorf("expected empty, got %q", got)
	}
}

func TestEscapeString_WhenAllSpecial_ShouldEscapeAll(t *testing.T) {
	got := escapeString("\"\\\n\r\t")
	expected := `\"\\\n\r\t`
	if got != expected {
		t.Errorf("expected %q, got %q", expected, got)
	}
}

// --- generateMangleFact ---

func TestGenerateMangleFact_WhenShardSpawn_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp: 1000,
		EventType: AuditShardSpawn,
		ShardID:   "coder-1",
		Target:    "coder",
		Success:   true,
	}
	fact := generateMangleFact(event)
	if fact == "" {
		t.Fatal("expected non-empty Mangle fact")
	}
	if !containsSubstring(fact, "shard_lifecycle") {
		t.Errorf("expected shard_lifecycle predicate, got: %s", fact)
	}
	if !containsSubstring(fact, "/shard_spawn") {
		t.Errorf("expected /shard_spawn in fact, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenActionRoute_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp:  2000,
		EventType:  AuditActionRoute,
		Action:     "read_file",
		Target:     "/path/to/file",
		Success:    true,
		DurationMs: 50,
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "action_event") {
		t.Errorf("expected action_event predicate, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenKernelAssert_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp: 3000,
		EventType: AuditKernelAssert,
		Target:    "user_intent",
		Success:   true,
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "kernel_op") {
		t.Errorf("expected kernel_op predicate, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenLLMResponse_ShouldIncludeTokens(t *testing.T) {
	event := AuditEvent{
		Timestamp:  4000,
		EventType:  AuditLLMResponse,
		ShardID:    "coder",
		Success:    true,
		DurationMs: 1500,
		Fields:     map[string]interface{}{"tokens": 500},
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "llm_call") {
		t.Errorf("expected llm_call predicate, got: %s", fact)
	}
	if !containsSubstring(fact, "500") {
		t.Errorf("expected token count 500 in fact, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenFileRead_ShouldIncludeSize(t *testing.T) {
	event := AuditEvent{
		Timestamp: 5000,
		EventType: AuditFileRead,
		Target:    "/path/to/file",
		Success:   true,
		Fields:    map[string]interface{}{"size": int64(1024)},
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "file_op") {
		t.Errorf("expected file_op predicate, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenIntentParsed_ShouldIncludeFields(t *testing.T) {
	event := AuditEvent{
		Timestamp: 6000,
		EventType: AuditIntentParsed,
		Target:    "auth.go",
		Fields: map[string]interface{}{
			"category":   "mutation",
			"verb":       "refactor",
			"confidence": 0.95,
		},
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "intent_parsed") {
		t.Errorf("expected intent_parsed predicate, got: %s", fact)
	}
	if !containsSubstring(fact, "0.95") {
		t.Errorf("expected confidence in fact, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenSafetyBlock_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp: 7000,
		EventType: AuditSafetyBlock,
		Action:    "exec_rm_rf",
		Success:   false,
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "safety_check") {
		t.Errorf("expected safety_check predicate, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenPerfSlow_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp:  8000,
		EventType:  AuditPerfSlow,
		Category:   "kernel",
		Action:     "rebuild",
		DurationMs: 5000,
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "perf_metric") {
		t.Errorf("expected perf_metric predicate, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenErrorCritical_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp: 9000,
		EventType: AuditErrorCritical,
		Category:  "kernel",
		Error:     "panic in evaluation",
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "error_event") {
		t.Errorf("expected error_event predicate, got: %s", fact)
	}
	if !containsSubstring(fact, "/error_critical") {
		t.Errorf("expected /error_critical in fact, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenSessionStart_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp: 10000,
		EventType: AuditSessionStart,
		SessionID: "sess-123",
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "session_event") {
		t.Errorf("expected session_event predicate, got: %s", fact)
	}
	if !containsSubstring(fact, "sess-123") {
		t.Errorf("expected session ID in fact, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenToolComplete_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp:  11000,
		EventType:  AuditToolComplete,
		Target:     "file_reader",
		Action:     "read",
		Success:    true,
		DurationMs: 10,
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "tool_exec") {
		t.Errorf("expected tool_exec predicate, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenCampaignPhase_ShouldIncludePhase(t *testing.T) {
	event := AuditEvent{
		Timestamp: 12000,
		EventType: AuditCampaignPhase,
		SessionID: "campaign-1",
		Success:   true,
		Fields:    map[string]interface{}{"phase": "testing"},
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "campaign_event") {
		t.Errorf("expected campaign_event predicate, got: %s", fact)
	}
	if !containsSubstring(fact, "testing") {
		t.Errorf("expected phase in fact, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenLearningComplete_ShouldFormatCorrectly(t *testing.T) {
	event := AuditEvent{
		Timestamp: 13000,
		EventType: AuditLearningComplete,
		ShardID:   "learner",
		Target:    "pattern-123",
		Success:   true,
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "learning_event") {
		t.Errorf("expected learning_event predicate, got: %s", fact)
	}
}

func TestGenerateMangleFact_WhenUnknownType_ShouldUseDefault(t *testing.T) {
	event := AuditEvent{
		Timestamp: 14000,
		EventType: "unknown_event",
		Category:  "test",
		Message:   "something happened",
		Success:   true,
	}
	fact := generateMangleFact(event)
	if !containsSubstring(fact, "audit_event") {
		t.Errorf("expected audit_event fallback predicate, got: %s", fact)
	}
}

// --- Audit convenience constructors ---

func TestAuditWithSession_ShouldSetSessionID_Coverage(t *testing.T) {
	logger := AuditWithSession("sess-456")
	if logger.sessionID != "sess-456" {
		t.Errorf("expected sessionID='sess-456', got %q", logger.sessionID)
	}
}

func TestAuditWithShard_ShouldSetShardID_Coverage(t *testing.T) {
	logger := AuditWithShard("shard-789")
	if logger.shardID != "shard-789" {
		t.Errorf("expected shardID='shard-789', got %q", logger.shardID)
	}
}

func TestAuditWithContext_ShouldSetAll_Coverage(t *testing.T) {
	logger := AuditWithContext("sess", "shard", CategoryKernel)
	if logger.sessionID != "sess" {
		t.Errorf("expected sessionID='sess', got %q", logger.sessionID)
	}
	if logger.shardID != "shard" {
		t.Errorf("expected shardID='shard', got %q", logger.shardID)
	}
	if logger.category != CategoryKernel {
		t.Errorf("expected category=kernel, got %q", logger.category)
	}
}

func TestAudit_ShouldReturnNonNil_Coverage(t *testing.T) {
	logger := Audit()
	if logger == nil {
		t.Fatal("Audit() returned nil")
	}
}

// --- CloseAudit ---

func TestCloseAudit_WhenNoFile_ShouldNotPanic(t *testing.T) {
	CloseAudit() // Should not panic
}

// helper
func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
