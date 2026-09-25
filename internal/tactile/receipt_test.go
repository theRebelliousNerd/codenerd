package tactile

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func terminalEvent(requestID string, result *ExecutionResult) AuditEvent {
	return AuditEvent{
		Type:         AuditEventComplete,
		Timestamp:    time.Now(),
		Command:      Command{Binary: "go", Arguments: []string{"test", "--api-key", "hunter2"}, RequestID: requestID, SessionID: "s1", Limits: &ResourceLimits{TimeoutMs: 5000, MaxOutputBytes: 1 << 20}},
		Result:       result,
		ExecutorName: "direct",
	}
}

// One bounded receipt per execution, whatever its outcome
// (tactile-effect-receipt-v1).
func TestExecutionReceiptOutcomes(t *testing.T) {
	start := time.Now().Add(-time.Second)
	cases := []struct {
		name    string
		result  *ExecutionResult
		outcome string
	}{
		{"success", &ExecutionResult{Success: true, ExitCode: 0, StartedAt: start, FinishedAt: start.Add(time.Second)}, ReceiptOutcomeSuccess},
		{"nonzero", &ExecutionResult{Success: true, ExitCode: 2, StartedAt: start}, ReceiptOutcomeNonZeroExit},
		{"timeout", &ExecutionResult{Success: true, ExitCode: -1, Killed: true, KillReason: "timeout after 5s", StartedAt: start}, ReceiptOutcomeKilled},
		{"canceled", &ExecutionResult{Success: true, ExitCode: -1, Killed: true, KillReason: "context canceled", StartedAt: start}, ReceiptOutcomeKilled},
		{"infra", &ExecutionResult{Success: false, ExitCode: -1, Error: "exec: not found", StartedAt: start}, ReceiptOutcomeInfraError},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			logger := NewAuditLogger()
			logger.Log(AuditEvent{Type: AuditEventStart, Command: Command{Binary: "go", RequestID: "req-" + tc.name}})
			if _, ok := logger.Receipt("req-" + tc.name); ok {
				t.Fatal("a start event must not produce a receipt")
			}
			logger.Log(terminalEvent("req-"+tc.name, tc.result))
			r, ok := logger.Receipt("req-" + tc.name)
			if !ok {
				t.Fatal("no receipt for a completed execution")
			}
			if r.Version != ExecutionReceiptVersion || r.Outcome != tc.outcome || r.Executor != "direct" || r.TimeoutMs != 5000 {
				t.Fatalf("receipt = %+v, want outcome %s", r, tc.outcome)
			}
			if r.IdempotencyKey == "" || r.ArgsDigest == "" {
				t.Fatalf("receipt lacks identity: %+v", r)
			}
		})
	}
}

// Output is kept as a digest plus a bounded, redacted preview; arguments only
// as a digest of their redacted form.
func TestExecutionReceiptBoundsAndRedacts(t *testing.T) {
	big := strings.Repeat("x", 10_000) + " token=SECRETVALUE Bearer abc.def.ghi"
	logger := NewAuditLogger()
	logger.Log(terminalEvent("req-big", &ExecutionResult{Success: true, Stdout: "api_key=SECRETVALUE sk-live-0123456789abcdef " + big, Truncated: true, TruncatedBytes: 42}))

	r, ok := logger.Receipt("req-big")
	if !ok {
		t.Fatal("no receipt")
	}
	if len(r.StdoutPreview) > maxReceiptPreviewBytes+len("...[truncated]") {
		t.Fatalf("preview not bounded: %d bytes", len(r.StdoutPreview))
	}
	for _, secret := range []string{"SECRETVALUE", "sk-live-0123456789abcdef", "hunter2"} {
		if strings.Contains(r.StdoutPreview, secret) {
			t.Fatalf("receipt preview leaks %q: %q", secret, r.StdoutPreview)
		}
	}
	if r.StdoutDigest != sha256Hex("api_key=SECRETVALUE sk-live-0123456789abcdef "+big) {
		t.Fatal("stdout digest does not cover the full output")
	}
	if !r.Truncated || r.TruncatedBytes != 42 {
		t.Fatalf("truncation not recorded: %+v", r)
	}
}

// The same terminal event logged twice (a composite callback and a wrapper
// both reporting it) converges on one receipt.
func TestExecutionReceiptIdempotent(t *testing.T) {
	logger := NewAuditLogger()
	ev := terminalEvent("req-dup", &ExecutionResult{Success: true, StartedAt: time.Now()})
	logger.Log(ev)
	logger.Log(ev)
	if n := len(logger.Receipts()); n != 1 {
		t.Fatalf("duplicate event produced %d receipts", n)
	}
}

// A fact the kernel rejects is counted on the receipt; the execution is not
// repeated.
func TestExecutionReceiptCountsRejectedFacts(t *testing.T) {
	logger := NewAuditLogger()
	calls := 0
	logger.SetFactSink(func(f Fact) error {
		calls++
		if f.Predicate == "execution_output" {
			return errors.New("kernel rejected it")
		}
		return nil
	})
	logger.Log(terminalEvent("req-facts", &ExecutionResult{Success: true, Stdout: "ok"}))
	r, ok := logger.Receipt("req-facts")
	if !ok {
		t.Fatal("no receipt")
	}
	if r.FactsEmitted != calls || r.FactsEmitted == 0 {
		t.Fatalf("FactsEmitted = %d, sink saw %d", r.FactsEmitted, calls)
	}
	if r.FactsRejected != 1 {
		t.Fatalf("FactsRejected = %d, want 1", r.FactsRejected)
	}
}

// A real execution through the audited wrapper yields a receipt, and the file
// sink persists it without the redacted argument value.
func TestExecutionReceiptFromARealExecutionIsPersisted(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.jsonl")
	logger := NewAuditLogger()
	if err := logger.EnableFileLogging(path); err != nil {
		t.Fatalf("EnableFileLogging: %v", err)
	}
	defer logger.Close()
	exec := NewAuditedExecutor(NewDirectExecutorWithConfig(DefaultExecutorConfig()), logger)
	if _, err := exec.Execute(context.Background(), Command{Binary: "go", Arguments: []string{"version"}, RequestID: "req-real"}); err != nil {
		t.Fatalf("execute: %v", err)
	}
	r, ok := logger.Receipt("req-real")
	if !ok || r.Outcome != ReceiptOutcomeSuccess || r.StdoutPreview == "" {
		t.Fatalf("receipt for a real execution: %+v ok=%v", r, ok)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open audit log: %v", err)
	}
	defer f.Close()
	found := false
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 1<<20), 1<<20)
	for scanner.Scan() {
		var line struct {
			Type    string           `json:"type"`
			Receipt ExecutionReceipt `json:"receipt"`
		}
		if json.Unmarshal(scanner.Bytes(), &line) == nil && line.Type == "execution_receipt" && line.Receipt.RequestID == "req-real" {
			found = true
		}
	}
	if !found {
		t.Fatal("receipt was not persisted to the audit file")
	}
}
