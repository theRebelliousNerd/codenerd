package tactile

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
	"testing"
	"time"
)

// =============================================================================
// Fact.String
// =============================================================================

func TestFact_String_WhenVariousArgTypes_ShouldFormat(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		fact     Fact
		contains []string
	}{
		{
			name:     "StringAtomArg",
			fact:     Fact{Predicate: "p", Args: []any{"/atom"}},
			contains: []string{"p(/atom)."},
		},
		{
			name:     "QuotedStringArg",
			fact:     Fact{Predicate: "p", Args: []any{"hello world"}},
			contains: []string{`p("hello world").`},
		},
		{
			name:     "UnixPathIsNotAtom",
			fact:     Fact{Predicate: "p", Args: []any{"/tmp/work/main.go"}},
			contains: []string{`p("/tmp/work/main.go").`},
		},
		{
			name:     "IntArg",
			fact:     Fact{Predicate: "p", Args: []any{42}},
			contains: []string{"p(42)."},
		},
		{
			name:     "Int64Arg",
			fact:     Fact{Predicate: "p", Args: []any{int64(999)}},
			contains: []string{"p(999)."},
		},
		{
			name:     "Float64Arg",
			fact:     Fact{Predicate: "p", Args: []any{3.14}},
			contains: []string{"p(3.140000)."},
		},
		{
			name:     "BoolTrue",
			fact:     Fact{Predicate: "p", Args: []any{true}},
			contains: []string{"p(/true)."},
		},
		{
			name:     "BoolFalse",
			fact:     Fact{Predicate: "p", Args: []any{false}},
			contains: []string{"p(/false)."},
		},
		{
			name:     "NoArgs",
			fact:     Fact{Predicate: "p", Args: nil},
			contains: []string{"p()."},
		},
		{
			name:     "MultipleArgs",
			fact:     Fact{Predicate: "exec", Args: []any{"req-1", int64(0), "/success"}},
			contains: []string{`exec("req-1", 0, /success).`},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := tt.fact.String()
			for _, c := range tt.contains {
				if got != c {
					t.Errorf("Fact.String() = %q, want %q", got, c)
				}
			}
		})
	}
}

// =============================================================================
// AuditEvent.ToFacts
// =============================================================================

func TestAuditEvent_ToFacts_WhenStartEvent_ShouldProduceStartFacts(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Command: Command{
			Binary:           "go",
			Arguments:        []string{"test"},
			RequestID:        "req-1",
			WorkingDirectory: "/project",
			Tags:             map[string]string{"env": "test"},
		},
		SessionID:    "sess-1",
		ExecutorName: "direct",
	}

	facts := event.ToFacts()

	// Should have execution_started, execution_command, execution_working_dir, execution_tag
	predicates := make(map[string]bool)
	for _, f := range facts {
		predicates[f.Predicate] = true
	}

	if !predicates["execution_started"] {
		t.Error("expected execution_started fact")
	}
	if !predicates["execution_command"] {
		t.Error("expected execution_command fact")
	}
	if !predicates["execution_working_dir"] {
		t.Error("expected execution_working_dir fact")
	}
	if !predicates["execution_tag"] {
		t.Error("expected execution_tag fact")
	}
}

func TestAuditEvent_ToFacts_WhenCommandContainsSecretArgument_ShouldRedactCommandFact(t *testing.T) {
	t.Parallel()
	secret := "secret-command-token"
	event := AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command: Command{
			Binary:    "tool",
			Arguments: []string{"--token", secret, "https://example.test/?api_key=" + secret},
			RequestID: "redacted-command",
		},
	}
	for _, fact := range event.ToFacts() {
		if fact.Predicate != "execution_command" {
			continue
		}
		command, ok := fact.Args[1].(string)
		if !ok {
			t.Fatalf("execution_command arg = %#v", fact.Args[1])
		}
		if strings.Contains(command, secret) || !strings.Contains(command, "[REDACTED]") {
			t.Fatalf("execution command did not redact secret: %q", command)
		}
		return
	}
	t.Fatal("execution_command fact missing")
}

func TestAuditEvent_ToFacts_WhenStartNoWorkDir_ShouldOmitWorkDirFact(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command: Command{
			Binary:    "echo",
			RequestID: "req-2",
		},
	}
	facts := event.ToFacts()
	for _, f := range facts {
		if f.Predicate == "execution_working_dir" {
			t.Error("should not produce execution_working_dir when WorkingDirectory is empty")
		}
	}
}

func TestAuditEvent_ToFacts_WhenCompleteSuccessZeroExit_ShouldProduceSuccessFact(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "req-3"},
		Result: &ExecutionResult{
			Success:     true,
			ExitCode:    0,
			Stdout:      "ok",
			Stderr:      "",
			Duration:    500 * time.Millisecond,
			SandboxUsed: SandboxNone,
		},
	}
	facts := event.ToFacts()

	predicates := make(map[string]bool)
	for _, f := range facts {
		predicates[f.Predicate] = true
	}

	if !predicates["execution_completed"] {
		t.Error("expected execution_completed")
	}
	if !predicates["execution_output"] {
		t.Error("expected execution_output")
	}
	if !predicates["execution_success"] {
		t.Error("expected execution_success")
	}
	if predicates["execution_nonzero"] {
		t.Error("should not have execution_nonzero for exit code 0")
	}
	if !predicates["execution_sandbox"] {
		t.Error("expected execution_sandbox")
	}
}

func TestAuditEvent_ToFacts_WhenSandboxUnset_ShouldEmitNoneName(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "r-unset"},
		Result:    &ExecutionResult{Success: true, ExitCode: 0},
	}
	for _, fact := range event.ToFacts() {
		if fact.Predicate == "execution_sandbox" {
			if !reflect.DeepEqual(fact.Args, []any{"r-unset", "/none"}) {
				t.Fatalf("execution_sandbox args = %#v", fact.Args)
			}
			return
		}
	}
	t.Fatal("execution_sandbox fact missing")
}

func TestAuditEvent_ToFacts_WhenCompleteNonZeroExit_ShouldProduceNonzeroFact(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "req-4"},
		Result: &ExecutionResult{
			Success:     true,
			ExitCode:    1,
			Duration:    100 * time.Millisecond,
			SandboxUsed: SandboxDocker,
		},
	}
	facts := event.ToFacts()

	found := false
	for _, f := range facts {
		if f.Predicate == "execution_nonzero" {
			found = true
			if len(f.Args) < 2 {
				t.Error("execution_nonzero should have at least 2 args")
			}
		}
	}
	if !found {
		t.Error("expected execution_nonzero fact")
	}
}

func TestAuditEvent_ToFacts_WhenCompleteFailure_ShouldProduceFailureFact(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "req-5"},
		Result: &ExecutionResult{
			Success:     false,
			ExitCode:    -1,
			Error:       "could not start process",
			Duration:    0,
			SandboxUsed: SandboxNone,
		},
	}
	facts := event.ToFacts()

	found := false
	for _, f := range facts {
		if f.Predicate == "execution_failure" {
			found = true
		}
	}
	if !found {
		t.Error("expected execution_failure fact")
	}
}

func TestAuditEvent_ToFacts_WhenCompleteNilResult_ShouldProduceNoFacts(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command: Command{
			RequestID: "req-6",
			Tags:      map[string]string{"must": "not-escape"},
		},
		Result: nil,
	}
	facts := event.ToFacts()
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for complete with nil result, got %d", len(facts))
	}
}

func TestAuditEvent_ToFacts_WhenCompleteWithResourceUsage_ShouldProduceResourceFacts(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "req-7"},
		Result: &ExecutionResult{
			Success:     true,
			ExitCode:    0,
			Duration:    200 * time.Millisecond,
			SandboxUsed: SandboxNone,
			ResourceUsage: &ResourceUsage{
				UserTimeMs:     100,
				SystemTimeMs:   50,
				MaxRSSBytes:    1024 * 1024,
				DiskReadBytes:  500,
				DiskWriteBytes: 300,
			},
		},
	}
	facts := event.ToFacts()

	predicates := make(map[string]bool)
	for _, f := range facts {
		predicates[f.Predicate] = true
	}

	if !predicates["execution_resource_usage"] {
		t.Error("expected execution_resource_usage fact")
	}
	if !predicates["execution_io"] {
		t.Error("expected execution_io fact when disk I/O > 0")
	}
}

func TestAuditEvent_ToFacts_WhenCompleteWithResourceUsageNoIO_ShouldOmitIOFact(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "req-8"},
		Result: &ExecutionResult{
			Success:     true,
			ExitCode:    0,
			Duration:    100 * time.Millisecond,
			SandboxUsed: SandboxNone,
			ResourceUsage: &ResourceUsage{
				UserTimeMs:     50,
				SystemTimeMs:   25,
				DiskReadBytes:  0,
				DiskWriteBytes: 0,
			},
		},
	}
	facts := event.ToFacts()

	for _, f := range facts {
		if f.Predicate == "execution_io" {
			t.Error("should not produce execution_io when disk I/O = 0")
		}
	}
}

func TestAuditEvent_ToFacts_WhenKilledEvent_ShouldProduceKilledFact(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventKilled,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "req-9"},
		Result: &ExecutionResult{
			Killed:     true,
			KillReason: "timeout exceeded",
			Duration:   30 * time.Second,
		},
	}
	facts := event.ToFacts()

	found := false
	for _, f := range facts {
		if f.Predicate == "execution_killed" {
			found = true
		}
	}
	if !found {
		t.Error("expected execution_killed fact")
	}
}

func TestAuditEvent_ToFacts_WhenKilledNilResult_ShouldProduceNoFacts(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventKilled,
		Timestamp: time.Now(),
		Command: Command{
			RequestID: "req-10",
			Tags:      map[string]string{"must": "not-escape"},
		},
		Result: nil,
	}
	facts := event.ToFacts()
	if len(facts) != 0 {
		t.Errorf("expected 0 facts for killed with nil result, got %d", len(facts))
	}
}

func TestAuditEvent_ToFacts_WhenErrorEvent_ShouldProduceErrorFact(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventError,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "req-11"},
		Result: &ExecutionResult{
			Error: "cannot allocate memory",
		},
	}
	facts := event.ToFacts()

	found := false
	for _, f := range facts {
		if f.Predicate == "execution_error" {
			found = true
			if len(f.Args) >= 2 {
				if f.Args[1] != "cannot allocate memory" {
					t.Errorf("expected error message in args, got %v", f.Args[1])
				}
			}
		}
	}
	if !found {
		t.Error("expected execution_error fact")
	}
}

func TestAuditEvent_ToFacts_WhenErrorNilResult_ShouldProduceEmptyErrorMsg(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventError,
		Timestamp: time.Now(),
		Command:   Command{RequestID: "req-12"},
		Result:    nil,
	}
	facts := event.ToFacts()

	found := false
	for _, f := range facts {
		if f.Predicate == "execution_error" {
			found = true
			if len(f.Args) >= 2 && f.Args[1] != "" {
				t.Errorf("expected empty error message with nil result, got %v", f.Args[1])
			}
		}
	}
	if !found {
		t.Error("expected execution_error fact even with nil result")
	}
}

func TestAuditEvent_ToFacts_WhenBlockedEvent_ShouldProduceBlockedFact(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:        AuditEventBlocked,
		Timestamp:   time.Now(),
		Command:     Command{RequestID: "req-13"},
		BlockReason: "permission denied by constitutional gate",
	}
	facts := event.ToFacts()

	found := false
	for _, f := range facts {
		if f.Predicate == "execution_blocked" {
			found = true
			if len(f.Args) >= 2 {
				reason, ok := f.Args[1].(string)
				if !ok || reason != "permission denied by constitutional gate" {
					t.Errorf("unexpected block reason: %v", f.Args[1])
				}
			}
		}
	}
	if !found {
		t.Error("expected execution_blocked fact")
	}
}

func TestAuditEvent_ToFacts_WhenSandboxedEvent_ShouldProduceSandboxedFact(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		sandbox  *SandboxConfig
		wantMode string
	}{
		{"WithDockerSandbox", &SandboxConfig{Mode: SandboxDocker}, "/docker"},
		{"WithNilSandbox", nil, "/none"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			event := AuditEvent{
				Type:      AuditEventSandboxed,
				Timestamp: time.Now(),
				Command: Command{
					RequestID: "req-14",
					Sandbox:   tt.sandbox,
				},
			}
			facts := event.ToFacts()

			found := false
			for _, f := range facts {
				if f.Predicate == "execution_sandboxed" {
					found = true
					if len(f.Args) >= 2 {
						mode, ok := f.Args[1].(string)
						if !ok || mode != tt.wantMode {
							t.Errorf("sandbox mode = %v, want %v", f.Args[1], tt.wantMode)
						}
					}
				}
			}
			if !found {
				t.Error("expected execution_sandboxed fact")
			}
		})
	}
}

func TestAuditEvent_ToFacts_WhenTags_ShouldProduceTagFacts(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command: Command{
			Binary:    "go",
			RequestID: "req-15",
			Tags:      map[string]string{"env": "prod", "team": "core"},
		},
	}
	facts := event.ToFacts()

	tagCount := 0
	for _, f := range facts {
		if f.Predicate == "execution_tag" {
			tagCount++
		}
	}
	if tagCount != 2 {
		t.Errorf("expected 2 tag facts, got %d", tagCount)
	}
}

// =============================================================================
// AuditLogger
// =============================================================================

func TestAuditLogger_WhenMultipleCallbacks_ShouldInvokeAll(t *testing.T) {
	t.Parallel()
	logger := NewAuditLogger()

	count1, count2 := 0, 0
	logger.AddCallback(func(e AuditEvent) { count1++ })
	logger.AddCallback(func(e AuditEvent) { count2++ })

	logger.Log(AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command:   Command{Binary: "test", RequestID: "r1"},
	})

	if count1 != 1 || count2 != 1 {
		t.Errorf("expected both callbacks invoked once, got %d and %d", count1, count2)
	}
}

func TestAuditLogger_WhenFactCallback_ShouldEmitFacts(t *testing.T) {
	t.Parallel()
	logger := NewAuditLogger()

	var facts []Fact
	logger.SetFactCallback(func(f Fact) {
		facts = append(facts, f)
	})

	logger.Log(AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command:   Command{Binary: "go", RequestID: "r2"},
	})

	if len(facts) == 0 {
		t.Error("expected facts to be emitted")
	}
}

func TestAuditLogger_WhenNoFactCallback_ShouldNotPanic(t *testing.T) {
	t.Parallel()
	logger := NewAuditLogger()

	// Should not panic with no fact callback
	logger.Log(AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command:   Command{Binary: "go", RequestID: "r3"},
	})
}

func TestAuditLogger_Close_WhenNoFileLogger_ShouldReturnNil(t *testing.T) {
	t.Parallel()
	logger := NewAuditLogger()
	if err := logger.Close(); err != nil {
		t.Errorf("Close() returned unexpected error: %v", err)
	}
}

func TestAuditLogger_GetMetrics_WhenNilMetrics_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()
	logger := &AuditLogger{}
	metrics := logger.GetMetrics()
	if metrics.TotalExecutions != 0 {
		t.Errorf("expected empty metrics, got %+v", metrics)
	}
}

// =============================================================================
// AuditFileLogger
// =============================================================================

func TestAuditFileLogger_WriteAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit.jsonl")

	fl, err := NewAuditFileLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditFileLogger failed: %v", err)
	}

	event := AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command:   Command{Binary: "test", RequestID: "r1"},
	}

	if err := fl.Write(event); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	if err := fl.Close(); err != nil {
		t.Fatalf("Close failed: %v", err)
	}

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "test") {
		t.Error("expected log file to contain event data")
	}
}

func TestAuditFileLogger_Write_WhenClosed_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit2.jsonl")

	fl, err := NewAuditFileLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditFileLogger failed: %v", err)
	}
	fl.Close()

	err = fl.Write(AuditEvent{})
	if err == nil {
		t.Error("expected error when writing to closed logger")
	}
}

func TestAuditFileLogger_Rotate(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit3.jsonl")

	fl, err := NewAuditFileLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditFileLogger failed: %v", err)
	}
	defer fl.Close()

	// Write an event before rotation
	fl.Write(AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command:   Command{Binary: "pre-rotate"},
	})

	if err := fl.Rotate(); err != nil {
		t.Fatalf("Rotate failed: %v", err)
	}

	// Write after rotation
	fl.Write(AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command:   Command{Binary: "post-rotate"},
	})

	// Check new file has post-rotate data
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "post-rotate") {
		t.Error("expected new log file to contain post-rotate data")
	}
}

func TestAuditFileLogger_Rotate_WhenClosed_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "audit4.jsonl")

	fl, err := NewAuditFileLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditFileLogger failed: %v", err)
	}
	fl.Close()

	err = fl.Rotate()
	if err == nil {
		t.Error("expected error when rotating closed logger")
	}
}

func TestAuditLogger_EnableFileLogging(t *testing.T) {
	tmpDir := t.TempDir()
	logPath := filepath.Join(tmpDir, "full_audit.jsonl")

	logger := NewAuditLogger()
	if err := logger.EnableFileLogging(logPath); err != nil {
		t.Fatalf("EnableFileLogging failed: %v", err)
	}
	defer logger.Close()

	logger.Log(AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command:   Command{Binary: "file-logged", RequestID: "r1"},
	})

	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	if !strings.Contains(string(data), "file-logged") {
		t.Error("expected file to contain logged event")
	}
}

func TestAuditLogger_EnableFileLogging_WhenReenabled_ShouldClosePrevious(t *testing.T) {
	tmpDir := t.TempDir()
	logger := NewAuditLogger()
	if err := logger.EnableFileLogging(filepath.Join(tmpDir, "first.jsonl")); err != nil {
		t.Fatalf("first EnableFileLogging failed: %v", err)
	}
	logger.mu.RLock()
	previous := logger.fileLogger
	logger.mu.RUnlock()

	if err := logger.EnableFileLogging(filepath.Join(tmpDir, "second.jsonl")); err != nil {
		t.Fatalf("second EnableFileLogging failed: %v", err)
	}
	defer logger.Close()

	if err := previous.Write(AuditEvent{}); err == nil {
		t.Fatal("previous audit file remained writable after replacement")
	}
}

func TestAuditLogger_Log_WhenFileWriteFails_ShouldExposeFailure(t *testing.T) {
	logger := NewAuditLogger()
	if err := logger.EnableFileLogging(filepath.Join(t.TempDir(), "audit.jsonl")); err != nil {
		t.Fatalf("EnableFileLogging failed: %v", err)
	}
	logger.mu.RLock()
	fileLogger := logger.fileLogger
	logger.mu.RUnlock()
	if err := fileLogger.Close(); err != nil {
		t.Fatalf("close file logger: %v", err)
	}

	logger.Log(AuditEvent{Type: AuditEventStart, Timestamp: time.Now()})
	snapshot := logger.GetMetrics()
	if snapshot.AuditFileWriteErrors != 1 {
		t.Fatalf("AuditFileWriteErrors = %d, want 1", snapshot.AuditFileWriteErrors)
	}
	if !strings.Contains(snapshot.LastAuditFileError, "not open") {
		t.Fatalf("LastAuditFileError = %q, want not-open evidence", snapshot.LastAuditFileError)
	}
}

func TestAuditFileLogger_Write_ShouldRedactSensitiveCommandDataAndBoundOutput(t *testing.T) {
	logPath := filepath.Join(t.TempDir(), "secure.jsonl")
	logger, err := NewAuditFileLogger(logPath)
	if err != nil {
		t.Fatalf("NewAuditFileLogger failed: %v", err)
	}
	defer logger.Close()

	secret := "never-persist-this-secret"
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command: Command{
			Binary:      "tool",
			Arguments:   []string{"--api-key=" + secret},
			Environment: []string{"API_TOKEN=" + secret},
			Stdin:       secret,
		},
		Result: &ExecutionResult{Stdout: strings.Repeat("x", auditOutputMaxBytes+1)},
	}
	if err := logger.Write(event); err != nil {
		t.Fatalf("Write failed: %v", err)
	}
	data, err := os.ReadFile(logPath)
	if err != nil {
		t.Fatalf("ReadFile failed: %v", err)
	}
	text := string(data)
	if strings.Contains(text, secret) {
		t.Fatal("audit file persisted secret environment/stdin data")
	}
	if !strings.Contains(text, "API_TOKEN=[REDACTED]") || !strings.Contains(text, "TRUNCATED IN AUDIT LOG") {
		t.Fatalf("audit file did not retain redaction/truncation evidence: %s", text)
	}
	if runtime.GOOS != "windows" {
		info, statErr := os.Stat(logPath)
		if statErr != nil {
			t.Fatalf("stat audit log: %v", statErr)
		}
		if got := info.Mode().Perm(); got != auditFileMode {
			t.Fatalf("audit log mode = %o, want %o", got, auditFileMode)
		}
	}
}

func TestBoundedAuditOutput_ShouldNotSplitUTF8Rune(t *testing.T) {
	t.Parallel()
	input := strings.Repeat("a", auditOutputMaxBytes-1) + "€suffix"
	output := boundedAuditOutput(input)
	if !strings.Contains(output, "TRUNCATED IN AUDIT LOG") {
		t.Fatal("expected truncation marker")
	}
	if strings.ContainsRune(output, '\uFFFD') {
		t.Fatal("truncation split a UTF-8 rune")
	}
}

func TestAuditFileLogger_Rotate_WhenRenameFails_ShouldNotRetainClosedHandle(t *testing.T) {
	logger, err := NewAuditFileLogger(filepath.Join(t.TempDir(), "audit.jsonl"))
	if err != nil {
		t.Fatalf("NewAuditFileLogger failed: %v", err)
	}
	logger.path = filepath.Join(t.TempDir(), "missing", "audit.jsonl")

	if err := logger.Rotate(); err == nil {
		t.Fatal("Rotate unexpectedly succeeded with a missing active path")
	}
	if logger.file != nil {
		t.Fatal("Rotate retained a file handle after rename and recovery failed")
	}
}

// =============================================================================
// ExecutionMetrics
// =============================================================================

func TestExecutionMetrics_RecordEvent_WhenAllEventTypes_ShouldTrackCorrectly(t *testing.T) {
	t.Parallel()
	m := NewExecutionMetrics()

	now := time.Now()

	// Start event
	m.RecordEvent(AuditEvent{
		Type:      AuditEventStart,
		Timestamp: now,
		Command:   Command{Binary: "go"},
		SessionID: "s1",
	})

	// Complete event (success)
	m.RecordEvent(AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: now,
		Result: &ExecutionResult{
			Success:  true,
			ExitCode: 0,
			Duration: 500 * time.Millisecond,
			ResourceUsage: &ResourceUsage{
				UserTimeMs:   100,
				SystemTimeMs: 50,
				MaxRSSBytes:  1024,
			},
		},
	})

	// Killed event
	m.RecordEvent(AuditEvent{
		Type:      AuditEventKilled,
		Timestamp: now,
		Result: &ExecutionResult{
			Duration: 1 * time.Second,
		},
	})

	// Error event
	m.RecordEvent(AuditEvent{
		Type:      AuditEventError,
		Timestamp: now,
	})

	// Blocked event
	m.RecordEvent(AuditEvent{
		Type:      AuditEventBlocked,
		Timestamp: now,
	})

	snap := m.Snapshot()

	if snap.TotalExecutions != 1 {
		t.Errorf("TotalExecutions = %d, want 1", snap.TotalExecutions)
	}
	if snap.SuccessfulExecutions != 1 {
		t.Errorf("SuccessfulExecutions = %d, want 1", snap.SuccessfulExecutions)
	}
	if snap.KilledExecutions != 1 {
		t.Errorf("KilledExecutions = %d, want 1", snap.KilledExecutions)
	}
	if snap.FailedExecutions != 1 {
		t.Errorf("FailedExecutions = %d, want 1 (error event)", snap.FailedExecutions)
	}
	if snap.BlockedExecutions != 1 {
		t.Errorf("BlockedExecutions = %d, want 1", snap.BlockedExecutions)
	}
	if snap.TotalCPUTimeMs != 150 {
		t.Errorf("TotalCPUTimeMs = %d, want 150", snap.TotalCPUTimeMs)
	}
	if snap.TotalMemoryBytes != 1024 {
		t.Errorf("TotalMemoryBytes = %d, want 1024", snap.TotalMemoryBytes)
	}
	if _, ok := snap.ExecutionsByBinary["go"]; !ok {
		t.Error("expected 'go' in ExecutionsByBinary")
	}
	if _, ok := snap.ExecutionsBySession["s1"]; !ok {
		t.Error("expected 's1' in ExecutionsBySession")
	}
}

func TestExecutionMetrics_Snapshot_WhenSuccessRate_ShouldCalculateCorrectly(t *testing.T) {
	t.Parallel()
	m := NewExecutionMetrics()

	for range 3 {
		m.RecordEvent(AuditEvent{
			Type:      AuditEventComplete,
			Timestamp: time.Now(),
			Result: &ExecutionResult{
				Success:  true,
				ExitCode: 0,
				Duration: 100 * time.Millisecond,
			},
		})
	}
	m.RecordEvent(AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Result: &ExecutionResult{
			Success: false,
			Error:   "fail",
		},
	})

	snap := m.Snapshot()
	// 3 successful, 1 failed => success rate = 3/4 = 0.75
	if snap.SuccessRate < 0.74 || snap.SuccessRate > 0.76 {
		t.Errorf("SuccessRate = %f, want ~0.75", snap.SuccessRate)
	}
}

func TestExecutionMetrics_Snapshot_WhenNoCompleted_ShouldReturnZeroRates(t *testing.T) {
	t.Parallel()
	m := NewExecutionMetrics()
	snap := m.Snapshot()
	if snap.SuccessRate != 0 {
		t.Errorf("SuccessRate = %f, want 0", snap.SuccessRate)
	}
	if snap.AvgDurationMs != 0 {
		t.Errorf("AvgDurationMs = %f, want 0", snap.AvgDurationMs)
	}
}

func TestExecutionMetrics_Reset_ShouldClearAll(t *testing.T) {
	t.Parallel()
	m := NewExecutionMetrics()

	m.RecordEvent(AuditEvent{
		Type:      AuditEventStart,
		Timestamp: time.Now(),
		Command:   Command{Binary: "go"},
		SessionID: "s1",
	})

	m.Reset()
	snap := m.Snapshot()

	if snap.TotalExecutions != 0 {
		t.Errorf("TotalExecutions after reset = %d, want 0", snap.TotalExecutions)
	}
	if len(snap.ExecutionsByBinary) != 0 {
		t.Errorf("ExecutionsByBinary should be empty after reset")
	}
}

func TestExecutionMetrics_RecordEvent_WhenCompleteNilResult_ShouldNotPanic(t *testing.T) {
	t.Parallel()
	m := NewExecutionMetrics()
	m.RecordEvent(AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Result:    nil,
	})
	// Should not panic
}

func TestExecutionMetrics_RecordEvent_WhenKilledNilResult_ShouldNotPanic(t *testing.T) {
	t.Parallel()
	m := NewExecutionMetrics()
	m.RecordEvent(AuditEvent{
		Type:      AuditEventKilled,
		Timestamp: time.Now(),
		Result:    nil,
	})
	snap := m.Snapshot()
	if snap.KilledExecutions != 1 {
		t.Errorf("KilledExecutions = %d, want 1", snap.KilledExecutions)
	}
}

func TestExecutionMetrics_RecordEvent_WhenCompleteNonZeroNonFailure_ShouldNotCountAsFailed(t *testing.T) {
	t.Parallel()
	m := NewExecutionMetrics()
	m.RecordEvent(AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Result: &ExecutionResult{
			Success:  true,
			ExitCode: 1, // non-zero but success=true
			Duration: 250 * time.Millisecond,
		},
	})
	snap := m.Snapshot()
	// Success=true but ExitCode=1 → not counted as successful (only ExitCode==0 counts)
	if snap.SuccessfulExecutions != 0 {
		t.Errorf("SuccessfulExecutions = %d, want 0 (non-zero exit)", snap.SuccessfulExecutions)
	}
	if snap.FailedExecutions != 0 {
		t.Errorf("FailedExecutions = %d, want 0 (success=true)", snap.FailedExecutions)
	}
	if snap.AvgDurationMs != 250 {
		t.Errorf("AvgDurationMs = %f, want 250 for the observed non-zero completion", snap.AvgDurationMs)
	}
}

// =============================================================================
// OutputAnalyzer
// =============================================================================

func TestOutputAnalyzer_AnalyzeTestOutput_WhenAllPass_ShouldSetOverallPass(t *testing.T) {
	t.Parallel()
	analyzer := NewOutputAnalyzer()
	output := `=== RUN   TestA
--- PASS: TestA (0.01s)
=== RUN   TestB
--- PASS: TestB (0.02s)
PASS`
	analysis := analyzer.AnalyzeTestOutput(output)

	if analysis.Passed != 2 {
		t.Errorf("Passed = %d, want 2", analysis.Passed)
	}
	if analysis.Failed != 0 {
		t.Errorf("Failed = %d, want 0", analysis.Failed)
	}
	if !analysis.OverallPass {
		t.Error("expected OverallPass = true")
	}
	if analysis.Total != 2 {
		t.Errorf("Total = %d, want 2", analysis.Total)
	}
}

func TestOutputAnalyzer_AnalyzeTestOutput_WhenEmpty_ShouldReturnZeros(t *testing.T) {
	t.Parallel()
	analyzer := NewOutputAnalyzer()
	analysis := analyzer.AnalyzeTestOutput("")

	if analysis.Passed != 0 || analysis.Failed != 0 || analysis.Skipped != 0 {
		t.Errorf("expected all zeros for empty output, got %+v", analysis)
	}
}

func TestOutputAnalyzer_AnalyzeTestOutput_ShouldRequireExactPassSummary(t *testing.T) {
	t.Parallel()
	analysis := NewOutputAnalyzer().AnalyzeTestOutput("--- PASS: TestA (0.01s)\nPASSING is user output")
	if analysis.OverallPass {
		t.Fatal("PASSING user output was mistaken for the go test PASS summary")
	}
}

func TestOutputAnalyzer_AnalyzeTestOutput_FailureCannotBeOverwrittenByLaterPass(t *testing.T) {
	t.Parallel()
	output := "--- FAIL: TestBroken (0.01s)\nPASS"
	analysis := NewOutputAnalyzer().AnalyzeTestOutput(output)
	if analysis.OverallPass {
		t.Fatalf("OverallPass = true despite %d parsed failure", analysis.Failed)
	}
}

func TestOutputAnalyzer_AnalyzeTestOutput_BareFailureCannotBeOverwrittenByLaterPass(t *testing.T) {
	t.Parallel()
	analysis := NewOutputAnalyzer().AnalyzeTestOutput("FAIL\nPASS")
	if analysis.OverallPass {
		t.Fatal("bare package FAIL was overwritten by a later PASS")
	}
}

func TestAuditEvent_ToFacts_WhenGoTestCompletes_ShouldEmitAnalyzerFacts(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command: Command{
			Binary:    `C:\Go\bin\go.exe`,
			Arguments: []string{"-C", `C:\workspace`, "test", "./..."},
			RequestID: "go-test-request",
		},
		Result: &ExecutionResult{
			Success:  true,
			ExitCode: 0,
			Stdout:   "--- PASS: TestOne (0.01s)\nPASS\ncoverage: 82.4% of statements",
		},
	}
	predicates := make(map[string]Fact)
	for _, fact := range event.ToFacts() {
		predicates[fact.Predicate] = fact
	}
	if _, ok := predicates["execution_test_summary"]; !ok {
		t.Fatalf("go test completion omitted analyzer facts: %#v", predicates)
	}
	if got := predicates["execution_test_state"].Args; !reflect.DeepEqual(got, []any{"go-test-request", "/passing"}) {
		t.Fatalf("execution_test_state args = %#v", got)
	}
	if got := predicates["execution_test_coverage"].Args; !reflect.DeepEqual(got, []any{"go-test-request", int64(82)}) {
		t.Fatalf("execution_test_coverage args = %#v", got)
	}
}

func TestAuditEvent_ToFacts_WhenGoBuildExitsNonzero_ShouldEmitFailedBuildSummary(t *testing.T) {
	t.Parallel()
	event := AuditEvent{
		Type:      AuditEventComplete,
		Timestamp: time.Now(),
		Command: Command{
			Binary:    "go",
			Arguments: []string{"build", "./..."},
			RequestID: "go-build-request",
		},
		Result: &ExecutionResult{Success: true, ExitCode: 1, Stderr: "linker failed"},
	}
	for _, fact := range event.ToFacts() {
		if fact.Predicate == "execution_build_summary" {
			if !reflect.DeepEqual(fact.Args, []any{"go-build-request", "/false", int64(0), int64(0)}) {
				t.Fatalf("execution_build_summary args = %#v", fact.Args)
			}
			return
		}
	}
	t.Fatal("go build completion omitted execution_build_summary")
}

func TestOutputAnalyzer_AnalyzeBuildOutput_WhenSuccess_ShouldReportSuccess(t *testing.T) {
	t.Parallel()
	analyzer := NewOutputAnalyzer()
	analysis := analyzer.AnalyzeBuildOutput("Build succeeded")

	if !analysis.Success {
		t.Error("expected Success = true for no errors")
	}
	if analysis.Errors != 0 {
		t.Errorf("Errors = %d, want 0", analysis.Errors)
	}
}

func TestOutputAnalyzer_AnalyzeBuildOutput_WhenEmpty_ShouldBeSuccess(t *testing.T) {
	t.Parallel()
	analyzer := NewOutputAnalyzer()
	analysis := analyzer.AnalyzeBuildOutput("")

	if !analysis.Success {
		t.Error("expected Success = true for empty output")
	}
}

func TestOutputAnalyzer_AnalyzeBuildOutput_WhenWarning_ShouldCountWarnings(t *testing.T) {
	t.Parallel()
	analyzer := NewOutputAnalyzer()
	output := "util.go:10:5: warning: unused variable"
	analysis := analyzer.AnalyzeBuildOutput(output)

	if analysis.Warnings != 1 {
		t.Errorf("Warnings = %d, want 1", analysis.Warnings)
	}
	if !analysis.Success {
		t.Error("expected Success = true when only warnings (no errors)")
	}
}

func TestOutputAnalyzer_AnalyzeBuildOutput_ShouldPreserveWindowsDrivePath(t *testing.T) {
	t.Parallel()
	analysis := NewOutputAnalyzer().AnalyzeBuildOutput(`C:\CodeProjects\codeNERD\main.go:12:7: undefined: thing`)
	if len(analysis.Diagnostics) != 1 {
		t.Fatalf("Diagnostics = %d, want 1: %+v", len(analysis.Diagnostics), analysis)
	}
	diagnostic := analysis.Diagnostics[0]
	if diagnostic.File != `C:\CodeProjects\codeNERD\main.go` || diagnostic.Line != 12 || diagnostic.Column != 7 {
		t.Fatalf("diagnostic = %+v, want preserved Windows path and location", diagnostic)
	}
}

// =============================================================================
// TestAnalysis.ToFacts
// =============================================================================

func TestTestAnalysis_ToFacts_WhenPassing_ShouldProducePassingState(t *testing.T) {
	t.Parallel()
	analysis := TestAnalysis{
		Passed:      5,
		Failed:      0,
		Skipped:     1,
		OverallPass: true,
		Coverage:    85.5,
	}

	facts := analysis.ToFacts("req-100")

	predicates := make(map[string]bool)
	for _, f := range facts {
		predicates[f.Predicate] = true
	}

	if !predicates["execution_test_summary"] {
		t.Error("expected execution_test_summary fact")
	}
	if !predicates["execution_test_state"] {
		t.Error("expected execution_test_state fact")
	}
	if !predicates["execution_test_coverage"] {
		t.Error("expected execution_test_coverage fact")
	}
	if predicates["execution_failed_test"] {
		t.Error("should not have execution_failed_test fact when no failures")
	}

	want := Fact{Predicate: "execution_test_coverage", Args: []any{"req-100", int64(86)}}
	found := false
	for _, fact := range facts {
		if reflect.DeepEqual(fact, want) {
			found = true
		}
	}
	if !found {
		t.Fatalf("coverage fact did not use bounded integer percent: %+v", facts)
	}
}

func TestTestAnalysis_ToFacts_WhenFailing_ShouldProduceFailingStateAndFailedTests(t *testing.T) {
	t.Parallel()
	analysis := TestAnalysis{
		Passed:      2,
		Failed:      1,
		OverallPass: false,
		FailedTests: []string{"TestBroken"},
	}

	facts := analysis.ToFacts("req-101")

	hasFailedTest := false
	hasFailingState := false
	for _, f := range facts {
		if f.Predicate == "execution_failed_test" {
			hasFailedTest = true
		}
		if f.Predicate == "execution_test_state" && reflect.DeepEqual(f.Args, []any{"req-101", "/failing"}) {
			hasFailingState = true
		}
	}

	if !hasFailedTest {
		t.Error("expected execution_failed_test fact")
	}
	if !hasFailingState {
		t.Error("expected execution_test_state correlated with req-101 and /failing")
	}
}

func TestTestAnalysis_ToFacts_WhenZeroCoverage_ShouldOmitCoverageFact(t *testing.T) {
	t.Parallel()
	analysis := TestAnalysis{
		OverallPass: true,
		Coverage:    0,
	}

	facts := analysis.ToFacts("req-102")
	for _, f := range facts {
		if f.Predicate == "execution_test_coverage" {
			t.Error("should not produce execution_test_coverage when coverage = 0")
		}
	}
}

// =============================================================================
// BuildAnalysis.ToFacts
// =============================================================================

func TestBuildAnalysis_ToFacts_ShouldProduceBuildResultAndDiagnostics(t *testing.T) {
	t.Parallel()
	analysis := BuildAnalysis{
		Success:  false,
		Errors:   2,
		Warnings: 1,
		Diagnostics: []Diagnostic{
			{File: "main.go", Line: 10, Column: 5, Message: "undefined: foo", Severity: "error"},
			{File: "util.go", Line: 20, Column: 3, Message: "unused var", Severity: "warning"},
		},
	}

	facts := analysis.ToFacts("req-200")

	hasBuildResult := false
	diagnosticCount := 0
	for _, f := range facts {
		if f.Predicate == "execution_build_summary" {
			hasBuildResult = true
			if !reflect.DeepEqual(f.Args, []any{"req-200", "/false", int64(2), int64(1)}) {
				t.Errorf("execution_build_summary args = %#v", f.Args)
			}
		}
		if f.Predicate == "execution_diagnostic" {
			diagnosticCount++
			if len(f.Args) != 6 {
				t.Errorf("execution_diagnostic arity = %d, want 6", len(f.Args))
			}
		}
	}

	if !hasBuildResult {
		t.Error("expected execution_build_summary fact")
	}
	if diagnosticCount != 2 {
		t.Errorf("expected 2 diagnostic facts, got %d", diagnosticCount)
	}
}

func TestAnalyzerFacts_ShouldBoundPerItemDetails(t *testing.T) {
	t.Parallel()
	failedTests := make([]string, auditDetailFactLimit+5)
	diagnostics := make([]Diagnostic, auditDetailFactLimit+5)
	for i := range failedTests {
		failedTests[i] = "TestFailure"
		diagnostics[i] = Diagnostic{File: "main.go", Line: i + 1, Column: 1, Severity: "error"}
	}

	testDetailCount := 0
	for _, fact := range (TestAnalysis{FailedTests: failedTests}).ToFacts("tests") {
		if fact.Predicate == "execution_failed_test" {
			testDetailCount++
		}
	}
	if testDetailCount != auditDetailFactLimit {
		t.Fatalf("failed-test detail facts = %d, want bounded %d", testDetailCount, auditDetailFactLimit)
	}

	diagnosticCount := 0
	for _, fact := range (BuildAnalysis{Diagnostics: diagnostics}).ToFacts("build") {
		if fact.Predicate == "execution_diagnostic" {
			diagnosticCount++
		}
	}
	if diagnosticCount != auditDetailFactLimit {
		t.Fatalf("diagnostic detail facts = %d, want bounded %d", diagnosticCount, auditDetailFactLimit)
	}
}

type unauditedExecutorStub struct {
	result *ExecutionResult
	err    error
}

func (s *unauditedExecutorStub) Execute(context.Context, Command) (*ExecutionResult, error) {
	return s.result, s.err
}

func (s *unauditedExecutorStub) Capabilities() ExecutorCapabilities {
	return ExecutorCapabilities{Name: "unaudited-stub"}
}

func (s *unauditedExecutorStub) Validate(Command) error { return nil }

func TestAuditedExecutorWrapper_WhenExecutorHasNoCallback_ShouldEmitLifecycle(t *testing.T) {
	t.Parallel()
	logger := NewAuditLogger()
	var events []AuditEvent
	logger.AddCallback(func(event AuditEvent) { events = append(events, event) })
	wrapper := NewAuditedExecutor(&unauditedExecutorStub{
		result: &ExecutionResult{Success: true, ExitCode: 0},
	}, logger)

	result, err := wrapper.Execute(context.Background(), Command{
		Binary:    "stub",
		SessionID: "session-1",
		RequestID: "request-1",
	})
	if err != nil || result == nil {
		t.Fatalf("Execute = (%+v, %v), want result", result, err)
	}
	if len(events) != 2 || events[0].Type != AuditEventStart || events[1].Type != AuditEventComplete {
		t.Fatalf("events = %+v, want start then complete", events)
	}
	if events[1].ExecutorName != "unaudited-stub" || events[1].SessionID != "session-1" {
		t.Fatalf("completion correlation = %+v", events[1])
	}
}

func TestAuditedExecutorWrapper_WhenKilledResultHasError_ShouldEmitKilled(t *testing.T) {
	t.Parallel()
	logger := NewAuditLogger()
	var events []AuditEvent
	logger.AddCallback(func(event AuditEvent) { events = append(events, event) })
	wrapper := NewAuditedExecutor(&unauditedExecutorStub{
		result: &ExecutionResult{
			Success:    false,
			ExitCode:   -1,
			Killed:     true,
			KillReason: "timeout",
		},
		err: errors.New("context deadline exceeded"),
	}, logger)

	_, _ = wrapper.Execute(context.Background(), Command{Binary: "stub", RequestID: "killed"})
	if len(events) != 2 || events[1].Type != AuditEventKilled {
		t.Fatalf("events = %+v, want killed terminal event", events)
	}
	if events[1].Result == nil || events[1].Result.KillReason != "timeout" {
		t.Fatalf("killed result lost context: %+v", events[1].Result)
	}
}

// =============================================================================
// AuditedExecutorWrapper
// =============================================================================

func TestNewAuditedExecutor_ShouldWrapCorrectly(t *testing.T) {
	t.Parallel()
	direct := NewDirectExecutor()
	logger := NewAuditLogger()

	wrapped := NewAuditedExecutor(direct, logger)
	if wrapped == nil {
		t.Fatal("NewAuditedExecutor returned nil")
	}

	caps := wrapped.Capabilities()
	if caps.Name != "direct" {
		t.Errorf("expected wrapped capabilities from direct executor, got %s", caps.Name)
	}

	if wrapped.GetLogger() != logger {
		t.Error("GetLogger should return the logger passed to NewAuditedExecutor")
	}
}
