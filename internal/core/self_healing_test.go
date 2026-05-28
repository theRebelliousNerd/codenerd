package core

import (
	"context"
	"math"
	"testing"
	"time"
)

// mockActionExecutor implements ActionExecutor for testing
type mockActionExecutor struct {
	execCount  int
	shouldFail bool
}

func (m *mockActionExecutor) Execute(ctx context.Context, req ActionRequest) (ActionResult, error) {
	m.execCount++
	if m.shouldFail {
		return ActionResult{Success: false, Error: "mock failure"}, nil
	}
	return ActionResult{Success: true}, nil
}

func TestSelfHealer_New(t *testing.T) {
	k := setupMockKernel(t)
	config := DefaultSelfHealerConfig()

	healer := NewSelfHealer(k, nil, config)
	if healer == nil {
		t.Fatal("NewSelfHealer returned nil")
	}
}

func TestSelfHealer_SetExecutor(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	executor := &mockActionExecutor{}
	healer.SetExecutor(executor)

	// No error means success
}

func TestSelfHealer_HandleValidationFailure_Retry(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	executor := &mockActionExecutor{shouldFail: false}
	healer.SetExecutor(executor)

	req := ActionRequest{
		ActionID: "action-1",
		Type:     ActionWriteFile,
	}
	vr := ValidationResult{
		Verified: false,
		Error:    "content mismatch",
	}

	ctx := context.Background()
	result, err := healer.HandleValidationFailure(ctx, req, vr)

	if err != nil {
		t.Logf("HandleValidationFailure returned error: %v", err)
	}

	if result != nil {
		t.Logf("Healing result: strategy=%s, success=%v", result.Strategy, result.Success)
	}
}

func TestSelfHealer_DetermineHealingType(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	vr := ValidationResult{
		Verified: false,
		Error:    "file not found",
	}

	healingType := healer.determineHealingType("action-1", vr)

	// Should return some healing type
	if healingType == "" {
		t.Error("Expected non-empty healing type")
	}
}

func TestSelfHealer_ClearHealingAttempts(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	// Clear should not panic
	healer.ClearHealingAttempts("action-1")

	// Get attempts should be 0
	attempts := healer.GetHealingAttempts("action-1")
	if attempts != 0 {
		t.Errorf("Expected 0 attempts after clear, got %d", attempts)
	}
}

func TestDefaultSelfHealerConfig(t *testing.T) {
	config := DefaultSelfHealerConfig()

	if config.MaxRetries <= 0 {
		t.Errorf("Expected positive MaxRetries, got %d", config.MaxRetries)
	}

	if config.RetryBackoff <= 0 {
		t.Error("Expected positive RetryBackoff")
	}
}

// TODO: TEST_GAP: [Null/Undefined/Empty] Verify HandleValidationFailure behaves gracefully (no panic) when called with a completely zero-initialized SelfHealer (nil kernel, nil validators) and an empty string for ActionID.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify determineHealingType correctly defaults to HealingEscalate when vr.Error is empty or purely whitespace.
// TODO: TEST_GAP: [Null/Undefined/Empty] Verify HandleValidationFailure returns a clean error instead of panicking when the executor is explicitly nil.
// TODO: TEST_GAP: [Type Coercion] Verify SelfHealer handles negative or extreme RetryBackoff durations (e.g., math.MaxInt64) without deadlocking the time.After select block.
// TODO: TEST_GAP: [Type Coercion] Verify behavior when config.MaxRetries is 0 or negative; it should immediately bypass retries and fallback to escalate.
// TODO: TEST_GAP: [User Request Extremes] Verify HandleValidationFailure bails out instantly and correctly propagates ctx.Err() if the context is cancelled immediately before or during the backoff period.
// TODO: TEST_GAP: [State Conflicts] Verify the "Zombie Retry" race condition: If ClearHealingAttempts is called while a retry is waiting in time.After, verify the retry count isn't reset causing an infinite retry bypass.
// TODO: TEST_GAP: [State Conflicts] Verify performance and safety under extreme concurrency (e.g., 10,000 goroutines) hitting HandleValidationFailure and ClearHealingAttempts for the same ActionID simultaneously to stress the global mutex.

// =============================================================================
// Boundary Analysis Coverage (QA 2026-05-20 self_healing_boundary_analysis)
// =============================================================================

// TestSelfHealer_NilKernel verifies that operating with a nil kernel does not
// panic. determineHealingType must short-circuit to HealingEscalate and
// HandleValidationFailure must surface a clean error when the executor is
// also unset, instead of dereferencing nil.
func TestSelfHealer_NilKernel(t *testing.T) {
	// Construct directly with nil kernel; healingAttempts must still be initialized
	// for the lock path to be safe.
	healer := NewSelfHealer(nil, nil, DefaultSelfHealerConfig())
	if healer == nil {
		t.Fatal("NewSelfHealer returned nil")
	}

	vr := ValidationResult{Verified: false, Error: "content hash mismatch"}
	got := healer.determineHealingType("act-nil-kernel", vr)
	if got != HealingEscalate {
		t.Errorf("expected HealingEscalate with nil kernel, got %q", got)
	}

	// HandleValidationFailure with no executor must error (not panic).
	_, err := healer.HandleValidationFailure(context.Background(), ActionRequest{ActionID: "act-nil-kernel"}, vr)
	if err == nil {
		t.Error("expected error when executor is unset, got nil")
	}

	// Fact emission helpers must be no-ops with nil kernel, not panic.
	healer.emitMaxRetriesFact("act-nil-kernel")
	healer.emitValidationAttemptFact("act-nil-kernel", 1, false)
	healer.emitHealingAttemptFact("act-nil-kernel", HealingRetry, false, "x")
	healer.emitEscalationFact("act-nil-kernel", "x")
}

// TestSelfHealer_NilValidatorRegistry verifies the retry path tolerates a
// nil validators registry (the success-revalidate block is skipped) and the
// retry recurses until maxRetries triggers a max-retries result.
func TestSelfHealer_NilValidatorRegistry(t *testing.T) {
	k := setupMockKernel(t)
	cfg := SelfHealerConfig{MaxRetries: 2, RetryBackoff: time.Millisecond}
	healer := NewSelfHealer(k, nil, cfg)
	healer.SetExecutor(&mockActionExecutor{shouldFail: false})

	req := ActionRequest{ActionID: "act-nil-validators", Type: ActionWriteFile}
	vr := ValidationResult{Verified: false, Error: "content hash mismatch"}

	result, err := healer.HandleValidationFailure(context.Background(), req, vr)
	if err != nil {
		t.Fatalf("HandleValidationFailure returned unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected non-nil result")
	}
	if result.Success {
		t.Error("expected Success=false when validators are nil (no revalidation possible)")
	}
}

// TestSelfHealer_EmptyActionID verifies that an empty ActionID does not pollute
// the shared healingAttempts map across unrelated callers: each successive
// invocation should accumulate against the "" key independently of other IDs.
func TestSelfHealer_EmptyActionID(t *testing.T) {
	k := setupMockKernel(t)
	healer := NewSelfHealer(k, nil, DefaultSelfHealerConfig())

	if got := healer.GetHealingAttempts(""); got != 0 {
		t.Errorf("expected 0 attempts for empty ID before activity, got %d", got)
	}

	// Establish a baseline count under a real ID; it must not be affected by
	// later activity under the empty ID.
	healer.mu.Lock()
	healer.healingAttempts["real-id"] = 7
	healer.healingAttempts[""] = 0
	healer.mu.Unlock()

	if got := healer.GetHealingAttempts(""); got != 0 {
		t.Errorf("empty-ID slot leaked from real-id, got %d", got)
	}

	healer.ClearHealingAttempts("")
	if got := healer.GetHealingAttempts("real-id"); got != 7 {
		t.Errorf("clearing empty ID corrupted real-id tracking: got %d, want 7", got)
	}
}

// TestSelfHealer_ZeroOrNegativeMaxRetries verifies that non-positive MaxRetries
// values bypass retries immediately and route through escalate. The current
// logic checks `attempts >= h.maxRetries`, so 0 retries means even the first
// healing decision must escalate.
func TestSelfHealer_ZeroOrNegativeMaxRetries(t *testing.T) {
	for _, max := range []int{0, -1, -100} {
		k := setupMockKernel(t)
		cfg := SelfHealerConfig{MaxRetries: max, RetryBackoff: time.Millisecond}
		healer := NewSelfHealer(k, nil, cfg)

		vr := ValidationResult{Verified: false, Error: "content hash mismatch"}
		got := healer.determineHealingType("act-zero-retries", vr)
		if got != HealingEscalate {
			t.Errorf("MaxRetries=%d: expected HealingEscalate, got %q", max, got)
		}
	}
}

// TestSelfHealer_NegativeDuration verifies that a negative retry backoff does
// not deadlock: time.After with a non-positive duration fires effectively
// immediately, so the retry path must complete promptly.
func TestSelfHealer_NegativeDuration(t *testing.T) {
	k := setupMockKernel(t)
	cfg := SelfHealerConfig{MaxRetries: 2, RetryBackoff: -1 * time.Second}
	healer := NewSelfHealer(k, nil, cfg)
	healer.SetExecutor(&mockActionExecutor{shouldFail: false})

	req := ActionRequest{ActionID: "act-neg-backoff", Type: ActionWriteFile}
	vr := ValidationResult{Verified: false, Error: "content hash mismatch"}

	done := make(chan struct{})
	go func() {
		defer close(done)
		_, _ = healer.HandleValidationFailure(context.Background(), req, vr)
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("HandleValidationFailure deadlocked with negative RetryBackoff")
	}
}

// TestSelfHealer_MaxInt64Duration verifies that an extreme backoff does not
// hang the test: a cancelled context must short-circuit the time.After select
// arm and return ctx.Err() immediately.
func TestSelfHealer_MaxInt64Duration(t *testing.T) {
	k := setupMockKernel(t)
	cfg := SelfHealerConfig{MaxRetries: 2, RetryBackoff: time.Duration(math.MaxInt64)}
	healer := NewSelfHealer(k, nil, cfg)
	healer.SetExecutor(&mockActionExecutor{shouldFail: false})

	req := ActionRequest{ActionID: "act-maxint-backoff", Type: ActionWriteFile}
	vr := ValidationResult{Verified: false, Error: "content hash mismatch"}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := healer.HandleValidationFailure(ctx, req, vr)
		done <- err
	}()

	// Give the goroutine a moment to enter the select, then cancel.
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected ctx.Err() to propagate after cancel, got nil")
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HandleValidationFailure did not unblock after ctx cancel with MaxInt64 backoff")
	}
}

// TestSelfHealer_ContextCancellation verifies that a context cancelled during
// the backoff wait causes immediate bail-out with the context error returned.
func TestSelfHealer_ContextCancellation(t *testing.T) {
	k := setupMockKernel(t)
	// Long enough backoff that we'll definitely cancel before time.After fires.
	cfg := SelfHealerConfig{MaxRetries: 3, RetryBackoff: 10 * time.Second}
	healer := NewSelfHealer(k, nil, cfg)
	healer.SetExecutor(&mockActionExecutor{shouldFail: false})

	req := ActionRequest{ActionID: "act-ctx-cancel", Type: ActionWriteFile}
	vr := ValidationResult{Verified: false, Error: "content hash mismatch"}

	ctx, cancel := context.WithCancel(context.Background())

	done := make(chan error, 1)
	go func() {
		_, err := healer.HandleValidationFailure(ctx, req, vr)
		done <- err
	}()

	// Cancel mid-backoff.
	time.Sleep(20 * time.Millisecond)
	cancel()

	select {
	case err := <-done:
		if err == nil {
			t.Error("expected non-nil error (ctx.Err) when ctx cancelled during backoff")
		} else if err != context.Canceled {
			t.Logf("got error: %v (acceptable if wrapped)", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("HandleValidationFailure did not bail out after ctx cancel during backoff")
	}
}
