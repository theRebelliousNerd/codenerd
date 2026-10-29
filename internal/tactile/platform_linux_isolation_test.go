//go:build linux

package tactile

import (
	"context"
	"strings"
	"sync"
	"testing"
)

// The composite registers exactly the isolation backends the host probe
// found. A backend the host provides serves an explicit request with real
// isolation; one it lacks stays unregistered and the request fails closed.
// Before registration existed, every explicit namespace/firejail request
// failed closed even on hosts that could isolate it.
func TestCompositeServesProbedLinuxIsolationAndFailsClosedOtherwise(t *testing.T) {
	probe := probeLinuxIsolation()
	t.Logf("host isolation probe: firejail=%q namespaces=%v cgroups=v%d", probe.firejailPath, probe.namespaces, probe.cgroupVersion)
	ce := NewCompositeExecutorWithConfig(DefaultExecutorConfig())

	nsCmd := Command{
		Binary:    "/bin/sh",
		Arguments: []string{"-c", "echo $$"},
		Sandbox:   &SandboxConfig{Mode: SandboxNamespace},
	}
	result, err := ce.Execute(context.Background(), nsCmd)
	if probe.namespaces {
		if err != nil {
			t.Fatalf("namespace isolation probed usable but request failed: %v", err)
		}
		if result.SandboxUsed != SandboxNamespace {
			t.Fatalf("SandboxUsed = %q, want namespace", result.SandboxUsed)
		}
		// A new PID namespace makes the shell PID 1: proof the command did
		// not run in the host's PID namespace.
		if got := strings.TrimSpace(result.Stdout); got != "1" || result.ExitCode != 0 {
			t.Fatalf("namespaced shell pid = %q (exit %d, stderr %q), want 1", got, result.ExitCode, result.Stderr)
		}
	} else if err == nil {
		t.Fatalf("namespace isolation unavailable on this host, but the request ran (sandbox %q)", result.SandboxUsed)
	}

	fjCmd := Command{Binary: "/bin/true", Sandbox: &SandboxConfig{Mode: SandboxFirejail}}
	if _, err := ce.Execute(context.Background(), fjCmd); probe.firejailPath == "" && err == nil {
		t.Fatal("firejail unavailable on this host, but the request ran")
	}
}

// A command that asks for a limit the direct executor cannot enforce goes to
// the cgroup-limited executor when the host has a writable cgroup hierarchy;
// a command that asks for none keeps the direct path.
func TestCompositeRoutesEnforcedLimitsToTheCgroupExecutor(t *testing.T) {
	probe := probeLinuxIsolation()
	ce := NewCompositeExecutorWithConfig(DefaultExecutorConfig())

	var mu sync.Mutex
	var names []string
	ce.SetAuditCallback(func(ev AuditEvent) {
		if ev.Type == AuditEventComplete {
			mu.Lock()
			names = append(names, ev.ExecutorName)
			mu.Unlock()
		}
	})

	limited := Command{Binary: "/bin/true", Limits: &ResourceLimits{MaxMemoryBytes: 256 << 20, TimeoutMs: 10000}}
	plain := Command{Binary: "/bin/true", Limits: &ResourceLimits{TimeoutMs: 10000}}
	for _, cmd := range []Command{limited, plain} {
		if _, err := ce.Execute(context.Background(), cmd); err != nil {
			t.Fatalf("execute %v: %v", cmd.Limits, err)
		}
	}

	mu.Lock()
	defer mu.Unlock()
	if len(names) != 2 {
		t.Fatalf("expected two completion events, got %v", names)
	}
	wantLimited := "direct"
	if probe.cgroupVersion != 0 {
		wantLimited = "limited-linux"
	}
	if names[0] != wantLimited {
		t.Fatalf("memory-limited command ran on %q, want %q (cgroups v%d)", names[0], wantLimited, probe.cgroupVersion)
	}
	if names[1] != "direct" {
		t.Fatalf("unlimited command ran on %q, want direct", names[1])
	}
}
