//go:build !windows

package shell

import (
	"context"
	"errors"
	"os/exec"
	"strings"
	"syscall"
	"testing"
	"time"
)

// A runner stopped by a signal while the tool's own deadline is live was
// stopped from outside the tool: the error says so and that no verdict was
// reached, instead of exec's bare "signal: terminated", which the session
// read as the suite's failure (problems log 2026-10-28). The tool's own
// deadline stays the context's error.
func TestVerificationRunError_AnOutsideKillIsNotAVerdict(t *testing.T) {
	cmd := exec.Command("sleep", "30")
	if err := cmd.Start(); err != nil {
		t.Skipf("no sleep binary: %v", err)
	}
	if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
		t.Fatal(err)
	}
	runErr := cmd.Wait()
	code := cmd.ProcessState.ExitCode()
	if code >= 0 {
		t.Fatalf("a signaled process reported exit %d", code)
	}

	err := verificationRunError(context.Background(), runErr, code, "go", 600)
	if err == nil || !strings.Contains(err.Error(), "stopped from outside this tool") ||
		!strings.Contains(err.Error(), "signal: terminated") || !strings.Contains(err.Error(), "no verdict") {
		t.Fatalf("err = %v, want an outside stop naming the signal and the missing verdict", err)
	}

	expired, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-expired.Done()
	if err := verificationRunError(expired, runErr, code, "go", 1); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("the tool's own deadline must stay the error, got %v", err)
	}

	exitOne := errors.New("exit status 1")
	if err := verificationRunError(context.Background(), exitOne, 1, "go", 600); err != exitOne {
		t.Fatalf("a run that exited keeps its own error, got %v", err)
	}
}
