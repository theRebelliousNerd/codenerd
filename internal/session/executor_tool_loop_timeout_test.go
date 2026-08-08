package session

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// The defect this guards (F-TIMEOUT-1, observed live on
// `nerd analyze internal/projectdoc`):
//
//	Error: shard execution failed: execution failed: LLM generation failed:
//	tool-result follow-up failed: context deadline exceeded
//
// That names neither the budget that expired, nor how much work had already
// succeeded, nor the flag that controls it. It is indistinguishable from a
// broken shard, and it sent this session chasing a non-defect — the run was
// progressing normally and simply needed longer, exactly as `nerd security`
// had minutes earlier.
//
// This is the third command where a timeout looked like a failure:
// `nerd tool generate` (fixed by describeStageTimeout) and `nerd dream` (fixed
// by dreamSummary) came first. The recurrence is why this is worth a helper
// rather than a one-off string.

func TestDescribeToolLoopFailure_NamesBudgetAndProgress(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Minute)
	defer cancel()

	err := describeToolLoopFailure(ctx, 3, 5, context.DeadlineExceeded)
	if err == nil {
		t.Fatal("expected an error")
	}
	msg := err.Error()

	if !strings.Contains(msg, "4 tool iteration") {
		t.Errorf("does not report how many iterations completed: %s", msg)
	}
	if !strings.Contains(msg, "5 tool call") {
		t.Errorf("does not report the work done in the final round: %s", msg)
	}
	if !strings.Contains(msg, "--timeout") {
		t.Errorf("does not name the flag that fixes it: %s", msg)
	}
	if !strings.Contains(msg, "not stuck") {
		t.Errorf("does not distinguish a slow run from a broken one: %s", msg)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("wrapping lost the sentinel; callers can no longer test for a timeout")
	}
}

// Non-timeout failures must pass through with their original framing. Dressing
// an unrelated error as a budget problem sends the reader the wrong way — the
// exact mistake this helper exists to stop.
func TestDescribeToolLoopFailure_PassesThroughOtherErrors(t *testing.T) {
	orig := errors.New("provider returned malformed tool_use block")

	got := describeToolLoopFailure(context.Background(), 1, 1, orig)
	if got == nil {
		t.Fatal("expected an error")
	}
	if !errors.Is(got, orig) {
		t.Errorf("original error was not wrapped: %v", got)
	}
	if strings.Contains(got.Error(), "--timeout") {
		t.Errorf("a non-timeout failure was blamed on the budget: %v", got)
	}
}

func TestDescribeToolLoopFailure_NilStaysNil(t *testing.T) {
	if err := describeToolLoopFailure(context.Background(), 0, 0, nil); err != nil {
		t.Errorf("nil became %v", err)
	}
}

// A context with no deadline must still produce a usable message rather than
// claiming a budget it cannot name.
func TestDescribeToolLoopFailure_NoDeadlineStillExplains(t *testing.T) {
	err := describeToolLoopFailure(context.Background(), 0, 2, context.DeadlineExceeded)
	if err == nil {
		t.Fatal("expected an error")
	}
	if !strings.Contains(err.Error(), "--timeout") {
		t.Errorf("lost the actionable hint: %v", err)
	}
}
