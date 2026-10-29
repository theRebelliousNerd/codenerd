package tactile

import (
	"crypto/sha256"
	"encoding/hex"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ExecutionReceiptVersion versions the receipt schema.
const ExecutionReceiptVersion = "tactile-execution-receipt-v1"

// maxReceiptPreviewBytes bounds each output preview kept in a receipt.
const maxReceiptPreviewBytes = 512

// maxRetainedReceipts bounds the receipts an AuditLogger keeps in memory.
const maxRetainedReceipts = 256

// Receipt outcomes.
const (
	ReceiptOutcomeSuccess     = "success"
	ReceiptOutcomeNonZeroExit = "nonzero_exit"
	ReceiptOutcomeKilled      = "killed"
	ReceiptOutcomeInfraError  = "infra_error"
)

// ExecutionReceipt is the one bounded record of a completed execution: what
// ran, on which backend, under which effective limits, how it ended, what was
// truncated, digests and redacted previews of its output, and how many of its
// facts the kernel accepted. It is evidence, never authorization: a command
// the constitution denied never reaches an executor and has no receipt. It
// holds no environment values, no stdin and no full output.
type ExecutionReceipt struct {
	Version        string      `json:"version"`
	IdempotencyKey string      `json:"idempotency_key"`
	SessionID      string      `json:"session_id,omitempty"`
	RequestID      string      `json:"request_id,omitempty"`
	Executor       string      `json:"executor"`
	Sandbox        SandboxMode `json:"sandbox"`
	Binary         string      `json:"binary"`
	ArgsDigest     string      `json:"args_digest"`

	TimeoutMs      int64 `json:"timeout_ms,omitempty"`
	MaxOutputBytes int64 `json:"max_output_bytes,omitempty"`
	MaxMemoryBytes int64 `json:"max_memory_bytes,omitempty"`
	MaxProcesses   int   `json:"max_processes,omitempty"`

	StartedAt  time.Time `json:"started_at"`
	FinishedAt time.Time `json:"finished_at"`
	DurationMs int64     `json:"duration_ms"`

	Outcome    string `json:"outcome"`
	ExitCode   int    `json:"exit_code"`
	KillReason string `json:"kill_reason,omitempty"`
	Error      string `json:"error,omitempty"`

	Truncated      bool  `json:"truncated,omitempty"`
	TruncatedBytes int64 `json:"truncated_bytes,omitempty"`

	StdoutDigest  string `json:"stdout_digest"`
	StderrDigest  string `json:"stderr_digest"`
	StdoutPreview string `json:"stdout_preview,omitempty"`
	StderrPreview string `json:"stderr_preview,omitempty"`

	FactsEmitted  int `json:"facts_emitted"`
	FactsRejected int `json:"facts_rejected"`
}

// receiptTokenPattern catches bearer tokens and common API-key shapes that
// secretAssignmentPattern (key=value) does not.
var receiptTokenPattern = regexp.MustCompile(`(?i)bearer\s+[A-Za-z0-9._\-]+|\b(?:sk|pk|rk|ghp|gho|xox[abp])[-_][A-Za-z0-9_\-]{8,}`)

// redactSecretsInText removes key=value secrets and token shapes from output
// kept in a receipt.
func redactSecretsInText(s string) string {
	s = secretAssignmentPattern.ReplaceAllString(s, "$1=[REDACTED]")
	return receiptTokenPattern.ReplaceAllString(s, "[REDACTED]")
}

func sha256Hex(s string) string {
	sum := sha256.Sum256([]byte(s))
	return hex.EncodeToString(sum[:])
}

// receiptPreview bounds and redacts output kept in a receipt.
func receiptPreview(s string) string {
	s = redactSecretsInText(s)
	if len(s) <= maxReceiptPreviewBytes {
		return s
	}
	end := maxReceiptPreviewBytes
	for end > 0 && s[end]&0xc0 == 0x80 {
		end--
	}
	return s[:end] + "...[truncated]"
}

// BuildExecutionReceipt builds the receipt for a terminal audit event (one
// that carries a result). ok is false for start events and events without a
// result.
func BuildExecutionReceipt(event AuditEvent, factsEmitted, factsRejected int) (ExecutionReceipt, bool) {
	if event.Result == nil || event.Type == AuditEventStart {
		return ExecutionReceipt{}, false
	}
	cmd := event.Command
	res := event.Result
	args := redactArguments(cmd.Arguments)

	started := res.StartedAt
	if started.IsZero() {
		started = event.Timestamp
	}
	finished := res.FinishedAt
	if finished.IsZero() {
		finished = event.Timestamp
	}

	r := ExecutionReceipt{
		Version:        ExecutionReceiptVersion,
		SessionID:      cmd.SessionID,
		RequestID:      cmd.RequestID,
		Executor:       event.ExecutorName,
		Sandbox:        res.SandboxUsed,
		Binary:         cmd.Binary,
		ArgsDigest:     sha256Hex(strings.Join(args, "\x00")),
		StartedAt:      started,
		FinishedAt:     finished,
		DurationMs:     res.Duration.Milliseconds(),
		ExitCode:       res.ExitCode,
		KillReason:     res.KillReason,
		Error:          receiptPreview(res.Error),
		Truncated:      res.Truncated,
		TruncatedBytes: res.TruncatedBytes,
		StdoutDigest:   sha256Hex(res.Stdout),
		StderrDigest:   sha256Hex(res.Stderr),
		StdoutPreview:  receiptPreview(res.Stdout),
		StderrPreview:  receiptPreview(res.Stderr),
		FactsEmitted:   factsEmitted,
		FactsRejected:  factsRejected,
	}
	if r.Sandbox == "" {
		r.Sandbox = SandboxNone
	}
	if cmd.Limits != nil {
		r.TimeoutMs = cmd.Limits.TimeoutMs
		r.MaxOutputBytes = cmd.Limits.MaxOutputBytes
		r.MaxMemoryBytes = cmd.Limits.MaxMemoryBytes
		r.MaxProcesses = cmd.Limits.MaxProcesses
	}
	switch {
	case res.Killed:
		r.Outcome = ReceiptOutcomeKilled
	case !res.Success:
		r.Outcome = ReceiptOutcomeInfraError
	case res.ExitCode != 0:
		r.Outcome = ReceiptOutcomeNonZeroExit
	default:
		r.Outcome = ReceiptOutcomeSuccess
	}
	r.IdempotencyKey = sha256Hex(strings.Join([]string{
		ExecutionReceiptVersion, cmd.SessionID, cmd.RequestID, cmd.Binary, r.ArgsDigest,
		strconv.FormatInt(started.UnixNano(), 10),
	}, "\x00"))
	return r, true
}

// receiptLedger keeps the most recent receipts, deduplicated by key.
type receiptLedger struct {
	mu        sync.Mutex
	byKey     map[string]ExecutionReceipt
	order     []string
	byRequest map[string]string
}

func newReceiptLedger() *receiptLedger {
	return &receiptLedger{byKey: map[string]ExecutionReceipt{}, byRequest: map[string]string{}}
}

// add records r; it reports false when a receipt with the same idempotency
// key was already recorded (the duplicate converges on the first).
func (l *receiptLedger) add(r ExecutionReceipt) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, dup := l.byKey[r.IdempotencyKey]; dup {
		return false
	}
	l.byKey[r.IdempotencyKey] = r
	l.order = append(l.order, r.IdempotencyKey)
	if r.RequestID != "" {
		l.byRequest[r.RequestID] = r.IdempotencyKey
	}
	for len(l.order) > maxRetainedReceipts {
		oldest := l.order[0]
		l.order = l.order[1:]
		if old, ok := l.byKey[oldest]; ok && l.byRequest[old.RequestID] == oldest {
			delete(l.byRequest, old.RequestID)
		}
		delete(l.byKey, oldest)
	}
	return true
}

func (l *receiptLedger) forRequest(requestID string) (ExecutionReceipt, bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	key, ok := l.byRequest[requestID]
	if !ok {
		return ExecutionReceipt{}, false
	}
	r, ok := l.byKey[key]
	return r, ok
}

func (l *receiptLedger) all() []ExecutionReceipt {
	l.mu.Lock()
	defer l.mu.Unlock()
	out := make([]ExecutionReceipt, 0, len(l.order))
	for _, k := range l.order {
		out = append(out, l.byKey[k])
	}
	return out
}
