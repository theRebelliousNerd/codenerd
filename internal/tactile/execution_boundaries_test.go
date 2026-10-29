package tactile

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Brutal execution-boundary probes: timeout kill, output cap, and fail-closed routing.

// A command that outlives its timeout must be killed promptly, not awaited.
func TestDirectTimeoutKill(t *testing.T) {
	ex := NewDirectExecutor()
	ctx := context.Background()
	res, err := ex.Execute(ctx, Command{
		Binary:    "sleep",
		Arguments: []string{"30"},
		Limits:    &ResourceLimits{TimeoutMs: 200},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Killed {
		t.Fatal("expected Killed=true for timed-out command")
	}
	if !strings.Contains(res.KillReason, "timeout") {
		t.Fatalf("expected timeout kill reason, got %q", res.KillReason)
	}
	if res.Duration > 10*time.Second {
		t.Fatalf("kill took too long: %v (process was awaited, not killed)", res.Duration)
	}
	if !res.Success {
		t.Fatal("Success should be true: infrastructure worked, command was killed")
	}
}

// Output beyond the cap must be truncated with an exact byte accounting.
func TestDirectOutputTruncationCap(t *testing.T) {
	ex := NewDirectExecutorWithConfig(ExecutorConfig{MaxOutputBytes: 64, DefaultTimeout: 30 * time.Second})
	ctx := context.Background()
	// sh -c 'head -c 4096 /dev/zero | tr "\0" "x"' avoids shell-glob portability issues.
	res, err := ex.Execute(ctx, Command{
		Binary:    "sh",
		Arguments: []string{"-c", `head -c 4096 /dev/zero | tr '\0' 'x'`},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Truncated {
		t.Fatal("expected Truncated=true for 4096-byte output with 64-byte cap")
	}
	if int64(len(res.Stdout)) != 64 {
		t.Fatalf("expected exactly 64 captured bytes, got %d", len(res.Stdout))
	}
	if res.TruncatedBytes != 4096-64 {
		t.Fatalf("expected 4032 discarded bytes, got %d", res.TruncatedBytes)
	}
}

// Explicit isolation is a security contract: a sandbox mode with no
// registered backend must fail closed, never silently execute on the host.
// Firejail is never auto-registered by the factory, so it is a deterministic
// missing-backend probe on every platform.
func TestCompositeFailClosedOnMissingSandboxBackend(t *testing.T) {
	ce := NewCompositeExecutorWithConfig(DefaultExecutorConfig())
	executed := false
	ce.SetAuditCallback(func(ev AuditEvent) {
		if ev.Type == AuditEventStart {
			executed = true
		}
	})
	ctx := context.Background()
	_, err := ce.Execute(ctx, Command{
		Binary:    "sh",
		Arguments: []string{"-c", "exit 0"},
		Sandbox:   &SandboxConfig{Mode: SandboxFirejail},
	})
	if err == nil {
		t.Fatal("expected error for unregistered firejail backend, got nil (fail-open!)")
	}
	if !strings.Contains(err.Error(), "firejail") {
		t.Fatalf("error should name the missing backend, got: %v", err)
	}
	if executed {
		t.Fatal("host executor ran a firejail-mode command (fail-open routing!)")
	}
}

// Non-zero exits are successful executions with a recorded code, not errors.
func TestDirectNonZeroExitShape(t *testing.T) {
	ex := NewDirectExecutor()
	res, err := ex.Execute(context.Background(), Command{
		Binary:    "sh",
		Arguments: []string{"-c", "exit 7"},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if !res.Success {
		t.Fatal("Success should be true for a ran-but-nonzero command")
	}
	if res.ExitCode != 7 {
		t.Fatalf("expected exit code 7, got %d", res.ExitCode)
	}
	if res.Killed {
		t.Fatal("Killed must be false for a normal non-zero exit")
	}
}
