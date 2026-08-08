package autopoiesis

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"
)

// The defect this guards (F-OURO-1, measured live on qwen3.8-max): one
// LLMCodeGeneration call took 597s, a second took 410s, GenerateTool consumed
// the entire 20-minute deadline, and LLMCodeRegeneration then ran for 0ms
// because the context was already dead. The pipeline could not succeed at ANY
// total budget -- generation always ate all of it before validation had a
// verdict for regeneration to act on.

func TestStageBudget_ReservesRemainderForLaterStages(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	stage, stageCancel := stageBudget(parent, "code generation", 0.5)
	defer stageCancel()

	deadline, ok := stage.Deadline()
	if !ok {
		t.Fatal("budgeted stage has no deadline")
	}

	got := time.Until(deadline)
	if got > 11*time.Minute {
		t.Errorf("stage got %s of a 20m budget; more than half leaves nothing for regeneration", got.Round(time.Second))
	}
	if got < 9*time.Minute {
		t.Errorf("stage got only %s of a 20m budget; too small to complete a 10-minute generation call", got.Round(time.Second))
	}
}

// A parent with no deadline must pass through untouched rather than inventing
// one -- inventing a limit would make an unbounded run fail artificially.
func TestStageBudget_NoParentDeadlinePassesThrough(t *testing.T) {
	stage, cancel := stageBudget(context.Background(), "code generation", 0.5)
	defer cancel()

	if _, ok := stage.Deadline(); ok {
		t.Error("a parent with no deadline produced a bounded child")
	}
}

// When very little time is left, subdividing guarantees failure. Better to let
// the stage race the parent deadline and possibly succeed.
func TestStageBudget_TinyRemainderIsNotSubdivided(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	stage, stageCancel := stageBudget(parent, "code regeneration", 0.5)
	defer stageCancel()

	deadline, ok := stage.Deadline()
	if !ok {
		t.Fatal("expected the parent deadline to be inherited")
	}
	// Should equal the parent's, not half of it.
	if remaining := time.Until(deadline); remaining < 25*time.Second {
		t.Errorf("a 30s remainder was subdivided to %s, guaranteeing failure", remaining.Round(time.Second))
	}
}

// An already-expired parent must not produce a negative budget.
func TestStageBudget_ExpiredParentIsSafe(t *testing.T) {
	parent, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	time.Sleep(2 * time.Millisecond)

	stage, stageCancel := stageBudget(parent, "code generation", 0.5)
	defer stageCancel()

	if stage == nil {
		t.Fatal("expired parent produced a nil context")
	}
}

// A bare "context deadline exceeded" cannot be acted on by an unattended run:
// a too-small budget and a broken feature produce byte-identical errors.
func TestDescribeStageTimeout_NamesStageAndBudget(t *testing.T) {
	err := describeStageTimeout("tool code generation", 597*time.Second, context.DeadlineExceeded)
	if err == nil {
		t.Fatal("expected an error")
	}

	msg := err.Error()
	if !strings.Contains(msg, "tool code generation") {
		t.Errorf("error does not name the stage: %s", msg)
	}
	if !strings.Contains(msg, "9m57s") {
		t.Errorf("error does not report how long the stage had: %s", msg)
	}
	if !strings.Contains(msg, "--timeout") {
		t.Errorf("error does not point at the knob that fixes it: %s", msg)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Error("wrapping lost the sentinel; callers can no longer test for a timeout")
	}
}

// Non-timeout errors must pass through unchanged -- dressing an unrelated
// failure as a budget problem sends the next reader in the wrong direction.
func TestDescribeStageTimeout_PassesThroughOtherErrors(t *testing.T) {
	orig := errors.New("model returned no code fence")
	got := describeStageTimeout("tool code generation", time.Minute, orig)

	if got != orig {
		t.Errorf("non-timeout error was rewritten: %v", got)
	}
	if describeStageTimeout("x", time.Minute, nil) != nil {
		t.Error("nil error became non-nil")
	}
}
