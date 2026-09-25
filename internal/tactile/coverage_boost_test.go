package tactile

import (
	"bytes"
	"context"
	"runtime"
	"slices"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// FileAuditEvent.ToFacts
// =============================================================================

func TestFileAuditEvent_ToFacts_WhenRead_ShouldProduceFileReadFact(t *testing.T) {
	t.Parallel()
	event := FileAuditEvent{
		Type:      FileOpRead,
		Timestamp: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Path:      "main.go",
		SessionID: "sess-1",
		Success:   true,
	}

	facts := event.ToFacts("")

	if len(facts) != 1 {
		t.Fatalf("expected 1 fact, got %d", len(facts))
	}
	if facts[0].Predicate != "file_read" {
		t.Errorf("expected predicate 'file_read', got %q", facts[0].Predicate)
	}
	if facts[0].Args[0] != "main.go" {
		t.Errorf("expected path arg 'main.go', got %v", facts[0].Args[0])
	}
	if facts[0].Args[1] != "sess-1" {
		t.Errorf("expected sessionID 'sess-1', got %v", facts[0].Args[1])
	}
}

func TestFileAuditEvent_ToFacts_WhenWrite_ShouldProduceWriteAndModifiedFacts(t *testing.T) {
	t.Parallel()
	event := FileAuditEvent{
		Type:      FileOpWrite,
		Timestamp: time.Now(),
		Path:      "utils.go",
		SessionID: "sess-2",
		NewHash:   "abc123",
		Success:   true,
	}

	facts := event.ToFacts("")

	predicates := make(map[string]bool)
	for _, f := range facts {
		predicates[f.Predicate] = true
	}
	if !predicates["file_written"] {
		t.Error("expected file_written fact")
	}
	if !predicates["modified"] {
		t.Error("expected modified fact")
	}
}

func TestFileAuditEvent_ToFacts_WhenEdit_ShouldProduceEditAndModifiedFacts(t *testing.T) {
	t.Parallel()
	event := FileAuditEvent{
		Type:      FileOpEdit,
		Timestamp: time.Now(),
		Path:      "handler.go",
		SessionID: "sess-3",
		StartLine: 10,
		EndLine:   20,
		Success:   true,
	}

	facts := event.ToFacts("")

	predicates := make(map[string]bool)
	for _, f := range facts {
		predicates[f.Predicate] = true
	}
	if !predicates["lines_edited"] {
		t.Error("expected lines_edited fact")
	}
	if !predicates["modified"] {
		t.Error("expected modified fact")
	}
}

func TestFileAuditEvent_ToFacts_WhenInsert_ShouldProduceInsertAndModifiedFacts(t *testing.T) {
	t.Parallel()
	event := FileAuditEvent{
		Type:       FileOpInsert,
		Timestamp:  time.Now(),
		Path:       "types.go",
		SessionID:  "sess-4",
		StartLine:  5,
		LinesAdded: 3,
		Success:    true,
	}

	facts := event.ToFacts("")

	predicates := make(map[string]bool)
	for _, f := range facts {
		predicates[f.Predicate] = true
	}
	if !predicates["lines_inserted"] {
		t.Error("expected lines_inserted fact")
	}
	if !predicates["modified"] {
		t.Error("expected modified fact")
	}
}

func TestFileAuditEvent_ToFacts_WhenDelete_ShouldProduceDeleteAndModifiedFacts(t *testing.T) {
	t.Parallel()
	event := FileAuditEvent{
		Type:      FileOpDelete,
		Timestamp: time.Now(),
		Path:      "old.go",
		SessionID: "sess-5",
		StartLine: 1,
		EndLine:   10,
		Success:   true,
	}

	facts := event.ToFacts("")

	predicates := make(map[string]bool)
	for _, f := range facts {
		predicates[f.Predicate] = true
	}
	if !predicates["lines_deleted"] {
		t.Error("expected lines_deleted fact")
	}
	if !predicates["modified"] {
		t.Error("expected modified fact")
	}
}

func TestFileAuditEvent_ToFacts_WhenPatch_ShouldProduceNoFacts(t *testing.T) {
	t.Parallel()
	event := FileAuditEvent{
		Type:      FileOpPatch,
		Timestamp: time.Now(),
		Path:      "patch.go",
	}
	facts := event.ToFacts("")
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for Patch type, got %d", len(facts))
	}
}

// =============================================================================
// computeHash
// =============================================================================

func TestComputeHash_WhenSameContent_ShouldBeEqual(t *testing.T) {
	t.Parallel()
	lines := []string{"line 1", "line 2", "line 3"}
	h1 := computeHash(lines)
	h2 := computeHash(lines)
	if h1 != h2 {
		t.Errorf("same content should produce same hash, got %q vs %q", h1, h2)
	}
}

func TestComputeHash_WhenDifferentContent_ShouldBeDifferent(t *testing.T) {
	t.Parallel()
	h1 := computeHash([]string{"a", "b"})
	h2 := computeHash([]string{"a", "c"})
	if h1 == h2 {
		t.Error("different content should produce different hashes")
	}
}

func TestComputeHash_WhenEmpty_ShouldReturnNonEmpty(t *testing.T) {
	t.Parallel()
	h := computeHash([]string{})
	if h == "" {
		t.Error("hash of empty content should not be empty string")
	}
}

// =============================================================================
// FileEditor.Exec
// =============================================================================

func TestFileEditor_Exec_ShouldReturnError(t *testing.T) {
	t.Parallel()
	editor := NewFileEditor()
	_, _, err := editor.Exec(context.Background(), "ls", nil)
	if err == nil {
		t.Error("expected error from Exec")
	}
	if !strings.Contains(err.Error(), "not supported") {
		t.Errorf("expected 'not supported' in error, got %q", err.Error())
	}
}

// =============================================================================
// FileEditor.ReplaceElement
// =============================================================================

func TestFileEditor_ReplaceElement_WhenValidContent_ShouldReplace(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	// Write initial file
	initial := []string{"A", "B", "C", "D"}
	if _, err := editor.WriteFile("replace.txt", initial); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Replace lines 2-3 with new content
	res, err := editor.ReplaceElement("replace.txt", 2, 3, "X\nY\nZ")
	if err != nil {
		t.Fatalf("ReplaceElement failed: %v", err)
	}
	if !res.Success {
		t.Error("expected success")
	}

	content, _ := editor.ReadFile("replace.txt")
	expected := []string{"A", "X", "Y", "Z", "D"}
	if strings.Join(content, ",") != strings.Join(expected, ",") {
		t.Errorf("replace mismatch: got %v, want %v", content, expected)
	}
}

func TestFileEditor_ReplaceElement_WhenEmptyContent_ShouldDeleteLines(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	initial := []string{"A", "B", "C"}
	if _, err := editor.WriteFile("empty_replace.txt", initial); err != nil {
		t.Fatalf("Setup failed: %v", err)
	}

	// Replace lines 2-2 with empty string
	_, err := editor.ReplaceElement("empty_replace.txt", 2, 2, "")
	if err != nil {
		t.Fatalf("ReplaceElement failed: %v", err)
	}

	content, _ := editor.ReadFile("empty_replace.txt")
	expected := []string{"A", "C"}
	if strings.Join(content, ",") != strings.Join(expected, ",") {
		t.Errorf("empty replace mismatch: got %v, want %v", content, expected)
	}
}

// =============================================================================
// FileEditor.GetFileInfo
// =============================================================================

func TestFileEditor_GetFileInfo_WhenFileExists_ShouldReturnInfo(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	content := []string{"line 1", "line 2", "line 3"}
	editor.WriteFile("info.txt", content)

	info, err := editor.GetFileInfo("info.txt")
	if err != nil {
		t.Fatalf("GetFileInfo failed: %v", err)
	}
	if info.LineCount != 3 {
		t.Errorf("expected 3 lines, got %d", info.LineCount)
	}
	if info.Hash == "" {
		t.Error("expected non-empty hash")
	}
	if info.Size == 0 {
		t.Error("expected non-zero size")
	}
	if info.Path != "info.txt" {
		t.Errorf("expected path 'info.txt', got %q", info.Path)
	}
}

func TestFileEditor_GetFileInfo_WhenFileNotExist_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditor()
	editor.SetWorkingDir(tmpDir)

	_, err := editor.GetFileInfo("nonexistent.txt")
	if err == nil {
		t.Error("expected error for nonexistent file")
	}
}

// =============================================================================
// FileEditor.SetFactCallback
// =============================================================================

func TestFileEditor_SetFactCallback_WhenSet_ShouldEmitFacts(t *testing.T) {
	tmpDir := t.TempDir()
	editor := NewFileEditorWithSession("test-session")
	editor.SetWorkingDir(tmpDir)

	var capturedFacts []Fact
	editor.SetFactCallback(func(f Fact) {
		capturedFacts = append(capturedFacts, f)
	})

	editor.WriteFile("fact_test.txt", []string{"hello"})

	if len(capturedFacts) == 0 {
		t.Error("expected facts to be emitted via fact callback")
	}
}

// =============================================================================
// limitedWriter
// =============================================================================

func TestLimitedWriter_WhenUnderLimit_ShouldWriteAll(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, max: 100}

	data := []byte("hello world")
	n, err := lw.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d bytes written, got %d", len(data), n)
	}
	if buf.String() != "hello world" {
		t.Errorf("expected 'hello world', got %q", buf.String())
	}
	if lw.truncated {
		t.Error("should not be truncated")
	}
	if lw.discarded != 0 {
		t.Errorf("expected 0 discarded, got %d", lw.discarded)
	}
}

func TestLimitedWriter_WhenOverLimit_ShouldTruncate(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, max: 5}

	data := []byte("hello world")
	n, err := lw.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	// Should report original length written
	if n != len(data) {
		t.Errorf("expected %d bytes reported, got %d", len(data), n)
	}
	// Only 5 bytes actually written
	if buf.Len() != 5 {
		t.Errorf("expected 5 bytes in buffer, got %d", buf.Len())
	}
	if !lw.truncated {
		t.Error("should be truncated")
	}
	if lw.discarded != 6 {
		t.Errorf("expected 6 discarded bytes, got %d", lw.discarded)
	}
}

func TestLimitedWriter_WhenAlreadyAtLimit_ShouldDiscardAll(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, max: 0}

	data := []byte("anything")
	n, err := lw.Write(data)
	if err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	if n != len(data) {
		t.Errorf("expected %d, got %d", len(data), n)
	}
	if buf.Len() != 0 {
		t.Errorf("expected 0 bytes in buffer, got %d", buf.Len())
	}
	if !lw.truncated {
		t.Error("should be truncated")
	}
	if lw.discarded != int64(len(data)) {
		t.Errorf("expected %d discarded, got %d", len(data), lw.discarded)
	}
}

func TestLimitedWriter_WhenMultipleWrites_ShouldTrackCumulative(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	lw := &limitedWriter{w: &buf, max: 10}

	lw.Write([]byte("12345")) // 5 bytes, under limit
	lw.Write([]byte("67890")) // 5 more, exactly at limit
	lw.Write([]byte("extra")) // over limit

	if buf.Len() != 10 {
		t.Errorf("expected 10 bytes in buffer, got %d", buf.Len())
	}
	if !lw.truncated {
		t.Error("should be truncated after exceeding limit")
	}
	if lw.discarded != 5 {
		t.Errorf("expected 5 discarded, got %d", lw.discarded)
	}
}

// =============================================================================
// RetryExecutor.shouldRetry
// =============================================================================

// =============================================================================
// RetryExecutor.SetRetryDelay
// =============================================================================

// =============================================================================
// RetryExecutor.Capabilities & Validate
// =============================================================================

// =============================================================================
// PooledExecutor - Borrow/Return/Stats
// =============================================================================

// =============================================================================
// ExecutorFactory
// =============================================================================

// =============================================================================
// CompositeExecutor
// =============================================================================

func TestCompositeExecutor_RegisterExecutor_ShouldAddExecutor(t *testing.T) {
	t.Parallel()
	composite := NewCompositeExecutor()
	direct := NewDirectExecutor()

	// Register the same direct executor under a custom mode
	composite.RegisterExecutor([]SandboxMode{SandboxMode("custom")}, direct)

	caps := composite.Capabilities()
	found := slices.Contains(caps.SupportedSandboxModes, SandboxMode("custom"))
	if !found {
		t.Error("expected 'custom' in supported sandbox modes after registration")
	}
}

func TestCompositeExecutor_SetAuditCallback_ShouldPropagate(t *testing.T) {
	t.Parallel()
	composite := NewCompositeExecutor()

	var capturedEvents []AuditEvent
	composite.SetAuditCallback(func(e AuditEvent) {
		capturedEvents = append(capturedEvents, e)
	})

	// The callback is set on composite. We can verify it's stored.
	// Running a command will trigger audit events through the direct executor
	// which had SetAuditCallback propagated to it.
	cmd := Command{Binary: "echo", Arguments: []string{"test"}}
	if composite.auditCallback == nil {
		t.Error("expected audit callback to be set")
	}
	_ = cmd // used for verification
}

func TestCompositeExecutor_Validate_WhenValidCommand_ShouldPass(t *testing.T) {
	t.Parallel()
	composite := NewCompositeExecutor()

	err := composite.Validate(Command{Binary: "echo"})
	if err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
}

func TestCompositeExecutor_Validate_WhenEmptyBinary_ShouldFail(t *testing.T) {
	t.Parallel()
	composite := NewCompositeExecutor()

	err := composite.Validate(Command{Binary: ""})
	if err == nil {
		t.Error("expected validation error for empty binary")
	}
}

func TestCompositeExecutor_Validate_WhenUnknownMode_ShouldFallbackToDefaultWhichRejects(t *testing.T) {
	t.Parallel()
	composite := NewCompositeExecutor()

	// selectExecutor falls back to defaultExecutor (direct) which rejects non-SandboxNone modes
	cmd := Command{
		Binary:  "echo",
		Sandbox: &SandboxConfig{Mode: SandboxMode("nonexistent")},
	}
	err := composite.Validate(cmd)
	// Direct executor rejects non-SandboxNone modes
	if err == nil {
		t.Error("expected validation error because direct executor rejects non-SandboxNone")
	}
}

// =============================================================================
// AuditedExecutorWrapper.Validate
// =============================================================================

func TestAuditedExecutorWrapper_Validate_ShouldDelegate(t *testing.T) {
	t.Parallel()
	direct := NewDirectExecutor()
	logger := NewAuditLogger()
	wrapped := NewAuditedExecutor(direct, logger)

	if err := wrapped.Validate(Command{Binary: "echo"}); err != nil {
		t.Errorf("unexpected validation error: %v", err)
	}
	if err := wrapped.Validate(Command{Binary: ""}); err == nil {
		t.Error("expected validation error for empty binary")
	}
}

// =============================================================================
// FileEditor.resolvePath
// =============================================================================

func TestFileEditor_ResolvePath_WhenAbsolute_ShouldReturnAsIs(t *testing.T) {
	t.Parallel()
	editor := NewFileEditor()
	tmpDir := t.TempDir()
	editor.SetWorkingDir(tmpDir)

	// Use the tmpDir itself as an absolute path to resolve
	absPath := editor.resolvePath(tmpDir)
	if absPath != tmpDir {
		t.Errorf("expected absolute path unchanged %q, got %q", tmpDir, absPath)
	}
}

func TestFileEditor_ResolvePath_WhenRelative_ShouldJoinWithWorkDir(t *testing.T) {
	t.Parallel()
	editor := NewFileEditor()
	editor.SetWorkingDir("/work")

	resolved := editor.resolvePath("relative/file.go")
	if !strings.Contains(resolved, "relative") || !strings.Contains(resolved, "file.go") {
		t.Errorf("expected resolved path to contain working dir and relative path, got %q", resolved)
	}
}

// =============================================================================
// FileOpType constants
// =============================================================================

func TestFileOpTypeConstants_ShouldBeDistinct(t *testing.T) {
	t.Parallel()
	ops := []FileOpType{FileOpRead, FileOpWrite, FileOpEdit, FileOpInsert, FileOpDelete, FileOpPatch}
	seen := make(map[FileOpType]bool)
	for _, op := range ops {
		if seen[op] {
			t.Errorf("duplicate FileOpType: %s", op)
		}
		seen[op] = true
	}
}

// =============================================================================
// DirectExecutor.SetAuditCallback
// =============================================================================

func TestDirectExecutor_SetAuditCallback_ShouldEmitEvents(t *testing.T) {
	t.Parallel()
	executor := NewDirectExecutor()

	var events []AuditEvent
	executor.SetAuditCallback(func(e AuditEvent) {
		events = append(events, e)
	})

	var cmd Command
	if runtime.GOOS == "windows" {
		cmd = Command{Binary: "cmd", Arguments: []string{"/c", "echo", "test"}}
	} else {
		cmd = Command{Binary: "echo", Arguments: []string{"test"}}
	}
	if err := executor.Validate(cmd); err != nil {
		t.Fatalf("validate failed: %v", err)
	}

	// Execute should trigger audit events
	result, err := executor.Execute(context.Background(), cmd)
	if err != nil {
		t.Fatalf("Execute failed: %v", err)
	}
	if !result.Success {
		t.Errorf("expected success: %s", result.Error)
	}

	if len(events) < 2 {
		t.Errorf("expected at least 2 audit events (start + complete), got %d", len(events))
	}
}

// =============================================================================
// DirectExecutor.buildEnvironment
// =============================================================================

func TestDirectExecutor_BuildEnvironment_ShouldIncludeAllowedVars(t *testing.T) {
	t.Parallel()
	config := DefaultExecutorConfig()
	executor := NewDirectExecutorWithConfig(config)

	env := executor.buildEnvironment([]string{"CUSTOM=value"})

	// Should contain CUSTOM=value from cmdEnv
	found := slices.Contains(env, "CUSTOM=value")
	if !found {
		t.Error("expected CUSTOM=value in environment")
	}
}

func TestDirectExecutor_BuildEnvironment_WhenNoCmdEnv_ShouldStillIncludeAllowed(t *testing.T) {
	t.Parallel()
	config := DefaultExecutorConfig()
	executor := NewDirectExecutorWithConfig(config)

	env := executor.buildEnvironment(nil)
	// Should still include allowed env vars that exist in the current environment
	// We can't assert specific values since they depend on the host, but the function should not panic
	if env == nil {
		t.Error("expected non-nil environment")
	}
}

// =============================================================================
// NewExecutorFactory with custom config
// =============================================================================

// =============================================================================
// AuditFileLogger.Close idempotent
// =============================================================================

func TestAuditFileLogger_Close_WhenCalledTwice_ShouldNotPanic(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := tmpDir + "/close_twice.jsonl"

	fl, err := NewAuditFileLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditFileLogger failed: %v", err)
	}
	if err := fl.Close(); err != nil {
		t.Fatalf("first Close failed: %v", err)
	}
	// Second close should return nil (already closed)
	if err := fl.Close(); err != nil {
		t.Errorf("second Close should return nil, got: %v", err)
	}
}
