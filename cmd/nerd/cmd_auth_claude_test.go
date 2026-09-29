package main

import (
	"context"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/spf13/cobra"
)

// stubClaudeProbe replaces the executable lookup and the probe constructor so
// the `claude --version` check runs without a real Claude CLI. The probe
// binary does not exist, so the check fails fast and returns before any
// config load; capt returns the context the probe ran on.
func stubClaudeProbe(t *testing.T, capt func(context.Context)) {
	t.Helper()
	savedLookPath := execLookPath
	savedExec := newExecCommand
	t.Cleanup(func() {
		execLookPath = savedLookPath
		newExecCommand = savedExec
	})
	execLookPath = func(file string) (string, error) { return "claude", nil }
	newExecCommand = func(ctx context.Context, name string, args ...string) *exec.Cmd {
		capt(ctx)
		return exec.Command("definitely-not-a-real-binary-xyz")
	}
}

func savedTimeout(t *testing.T) {
	t.Helper()
	saved := timeout
	t.Cleanup(func() { timeout = saved })
}

// The `claude --version` probe runs on commandContext, so the user's
// --timeout covers it. Before the fix the probe ran on cmd.Context(), which
// carries no deadline.
func TestRunAuthClaude_ProbeCoveredByTimeout(t *testing.T) {
	savedTimeout(t)
	timeout = 10 * time.Second
	var probed context.Context
	stubClaudeProbe(t, func(ctx context.Context) { probed = ctx })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	err := runAuthClaude(cmd, nil)
	if err == nil || !strings.Contains(err.Error(), "not authenticated") {
		t.Fatalf("err = %v, want the failed-probe error", err)
	}
	if probed == nil {
		t.Fatal("the claude probe never ran")
	}
	deadline, ok := probed.Deadline()
	if !ok {
		t.Fatal("--timeout 10s: probe context has no deadline")
	}
	if left := time.Until(deadline); left <= 0 || left > 10*time.Second {
		t.Errorf("--timeout 10s: probe deadline has %s left", left)
	}
}

// ...and with --timeout unset the probe has no deadline.
func TestRunAuthClaude_ProbeHasNoDeadlineWithoutTimeout(t *testing.T) {
	savedTimeout(t)
	timeout = 0
	var probed context.Context
	stubClaudeProbe(t, func(ctx context.Context) { probed = ctx })

	cmd := &cobra.Command{}
	cmd.SetContext(context.Background())
	if err := runAuthClaude(cmd, nil); err == nil {
		t.Fatal("expected the failed-probe error")
	}
	if probed == nil {
		t.Fatal("the claude probe never ran")
	}
	if deadline, ok := probed.Deadline(); ok {
		t.Fatalf("no --timeout: probe context has deadline %v", deadline)
	}
}
