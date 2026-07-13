package system

import (
	"strings"
	"testing"
	"time"
)

func TestRunCloseStep_Success(t *testing.T) {
	called := false
	err := runCloseStep("ok", time.Second, func() error {
		called = true
		return nil
	})
	if err != nil {
		t.Fatalf("runCloseStep: %v", err)
	}
	if !called {
		t.Fatal("expected step fn to run")
	}
}

func TestRunCloseStep_Timeout(t *testing.T) {
	start := time.Now()
	err := runCloseStep("hang", 40*time.Millisecond, func() error {
		time.Sleep(2 * time.Second)
		return nil
	})
	elapsed := time.Since(start)

	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "timed out") {
		t.Fatalf("error=%v, want timed out", err)
	}
	// Must return near the timeout, not after the full sleep.
	if elapsed > 500*time.Millisecond {
		t.Fatalf("runCloseStep returned too slowly: %v", elapsed)
	}
	if elapsed < 20*time.Millisecond {
		t.Fatalf("runCloseStep returned too quickly: %v", elapsed)
	}
}

func TestRunCloseStep_PropagatesError(t *testing.T) {
	err := runCloseStep("fail", time.Second, func() error {
		return errCloseStepProbe
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "probe-fail") {
		t.Fatalf("error=%v, want probe-fail", err)
	}
}

var errCloseStepProbe = &closeStepProbeError{}

type closeStepProbeError struct{}

func (e *closeStepProbeError) Error() string { return "probe-fail" }
