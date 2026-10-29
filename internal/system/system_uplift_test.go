package system

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// Bootstrap is a resource transaction: a failing step must abort the boot
// (later steps never run), name itself in the error, and return no Cortex.
// bootCortexWithSteps exists precisely so tests can exercise this without
// production hooks — these pins take it up on that.

var errUpliftBootStep = errors.New("uplift boom")

func TestBootSteps_FailingStep_AbortsAndNamesItself(t *testing.T) {
	var ran []string
	steps := []bootStep{
		{name: "first", run: func(bctx *bootContext) error {
			ran = append(ran, "first")
			return nil
		}},
		{name: "nil-run", run: nil}, // skipped, never aborts
		{name: "failing", run: func(bctx *bootContext) error {
			ran = append(ran, "failing")
			return errUpliftBootStep
		}},
		{name: "never", run: func(bctx *bootContext) error {
			ran = append(ran, "never")
			return nil
		}},
	}
	cortex, err := bootCortexWithSteps(context.Background(), BootConfig{}, steps)
	if cortex != nil {
		t.Error("failed boot must return a nil Cortex, not a partial one")
	}
	if err == nil {
		t.Fatal("failed boot must return an error")
	}
	if !errors.Is(err, errUpliftBootStep) {
		t.Errorf("boot error %v does not wrap the step error", err)
	}
	if !strings.Contains(err.Error(), "boot failing:") {
		t.Errorf("boot error %q does not name the failed step", err)
	}
	if len(ran) != 2 || ran[0] != "first" || ran[1] != "failing" {
		t.Errorf("ran steps = %v, want [first failing] (abort, skip nil-run)", ran)
	}
}

func TestBootSteps_AllSucceed_ReturnsCortex(t *testing.T) {
	steps := []bootStep{
		{name: "noop", run: func(bctx *bootContext) error { return nil }},
	}
	cortex, err := bootCortexWithSteps(context.Background(), BootConfig{}, steps)
	if err != nil {
		t.Fatalf("clean boot: %v", err)
	}
	if cortex == nil {
		t.Fatal("clean boot must return a Cortex")
	}
}

func TestRollbackBootContext_NilSafe(t *testing.T) {
	if err := rollbackBootContext(nil); err != nil {
		t.Errorf("rollbackBootContext(nil) = %v, want nil", err)
	}
	if err := rollbackBootContext(&bootContext{}); err != nil {
		t.Errorf("rollbackBootContext(empty) = %v, want nil", err)
	}
}
