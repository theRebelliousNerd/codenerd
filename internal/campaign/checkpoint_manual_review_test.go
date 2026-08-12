package campaign

import (
	"codenerd/internal/session"
	"context"
	"strings"
	"testing"
)

// TestManualReviewCheckpoint verifies that /manual_review no longer
// rubber-stamps in non-interactive mode. Previously this checkpoint returned
// true with "skipped (non-interactive mode)", making "we did not check"
// indistinguishable from "we checked and it was fine". It now escalates to
// shard validation, which fails closed when no TaskExecutor is wired.
func TestManualReviewCheckpoint(t *testing.T) {
	cr := NewCheckpointRunner(nil, nil, t.TempDir())
	phase := &Phase{Name: "unverifiable-phase"}

	t.Run("fail closed without task executor", func(t *testing.T) {
		// This is the exact case that returned true before — a genuine
		// behavioural RED against the old code. A checkpoint that cannot
		// be verified must never report PASSED.
		passed, details, err := cr.runManualReviewCheckpoint(context.Background(), phase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if passed {
			t.Fatal("manual review checkpoint passed without a task executor: an unrun verification reported success")
		}
		// Details must explain it could not be verified. The underlying
		// shard validation fail-closed message names TaskExecutor and says
		// unverified.
		if !strings.Contains(details, "TaskExecutor") {
			t.Errorf("details should name the missing collaborator so the fix is obvious; got %q", details)
		}
		lower := strings.ToLower(details)
		if !strings.Contains(lower, "unverified") && !strings.Contains(lower, "could not run") && !strings.Contains(lower, "could not be verified") {
			t.Errorf("details should explain that the phase could not be verified; got %q", details)
		}
	})

	t.Run("details mention escalation", func(t *testing.T) {
		_, details, err := cr.runManualReviewCheckpoint(context.Background(), phase)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lower := strings.ToLower(details)
		// Must be visible in the persisted checkpoint record that this was
		// not a human review but an escalation.
		if !strings.Contains(lower, "manual review") {
			t.Errorf("details should mention manual review was requested; got %q", details)
		}
		if !strings.Contains(lower, "non-interactive") && !strings.Contains(lower, "no human") {
			t.Errorf("details should mention non-interactive / no human was present; got %q", details)
		}
		if !strings.Contains(lower, "escalat") {
			t.Errorf("details should mention escalation to shard validation; got %q", details)
		}
		if !strings.Contains(lower, "shard validation") && !strings.Contains(lower, "shard") {
			t.Errorf("details should mention shard validation as the escalation target; got %q", details)
		}
	})

	t.Run("via Run with VerifyManualReview fails closed", func(t *testing.T) {
		passed, details, err := cr.Run(context.Background(), phase, VerifyManualReview)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if passed {
			t.Fatal("Run with VerifyManualReview passed without a task executor")
		}
		if !strings.Contains(details, "TaskExecutor") {
			t.Errorf("Run details should name TaskExecutor; got %q", details)
		}
		if !strings.Contains(strings.ToLower(details), "escalat") {
			t.Errorf("Run details should mention escalation; got %q", details)
		}
	})
}

func TestManualReviewCheckpoint_CancelledContext(t *testing.T) {
	cr := NewCheckpointRunner(nil, nil, t.TempDir())
	phase := &Phase{Name: "cancellable-phase"}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	passed, details, err := cr.runManualReviewCheckpoint(ctx, phase)
	if err == nil {
		t.Fatalf("expected context.Canceled error, got nil (passed=%v details=%q)", passed, details)
	}
	if err != context.Canceled {
		// Accept wrapped context.Canceled as well
		if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "cancelled") {
			t.Fatalf("expected context.Canceled, got %v", err)
		}
	}
	if passed {
		t.Fatal("cancelled context should not report PASSED")
	}

	// Also via the public Run path
	ctx2, cancel2 := context.WithCancel(context.Background())
	cancel2()
	passed, _, err = cr.Run(ctx2, phase, VerifyManualReview)
	if err == nil {
		t.Fatalf("expected context.Canceled from Run, got nil (passed=%v)", passed)
	}
	if passed {
		t.Fatal("cancelled Run should not report PASSED")
	}
}

func TestManualReviewCheckpoint_EscalatesToShardValidation(t *testing.T) {
	// When a TaskExecutor is present, manual review should delegate to
	// shard validation and the prefix should remain visible even on success.
	executor := &MockTaskExecutor{
		ExecuteFunc: func(ctx context.Context, req session.TaskRequest) (string, error) {
			// Simulate a reviewer shard that approves.
			if req.IntentVerb != "/review" {
				t.Errorf("expected /review intent, got %q", req.IntentVerb)
			}
			return "PASS: objectives met, all tasks reviewed", nil
		},
	}
	cr := NewCheckpointRunner(nil, executor, t.TempDir())
	phase := &Phase{
		Name: "escalated-phase",
		Objectives: []PhaseObjective{
			{Description: "do the thing", VerificationMethod: VerifyManualReview},
		},
		Tasks: []Task{
			{Description: "task 1", Status: TaskCompleted},
		},
	}

	passed, details, err := cr.runManualReviewCheckpoint(context.Background(), phase)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !passed {
		t.Fatalf("expected escalated shard validation to pass, got passed=false details=%q", details)
	}
	lower := strings.ToLower(details)
	if !strings.Contains(lower, "manual review") || !strings.Contains(lower, "escalat") {
		t.Errorf("successful escalation should still prefix with manual-review escalation note; got %q", details)
	}
	if !strings.Contains(lower, "shard validation") {
		t.Errorf("successful escalation should mention shard validation; got %q", details)
	}
	if !strings.Contains(details, "PASS") && !strings.Contains(lower, "review passed") {
		t.Errorf("details should contain the underlying shard validation result; got %q", details)
	}
}
