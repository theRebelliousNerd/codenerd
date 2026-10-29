//go:build !windows

package tactile

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// ExecInContainer's argv used to carry three "--" before the container ID, so
// docker read the second "--" as the container name and every exec failed.
// The fake docker records its argv; the exec must also be audited.
func TestPersistentDockerExecInvokesDockerExecCorrectlyAndAudits(t *testing.T) {
	dir := t.TempDir()
	argvFile := filepath.Join(dir, "argv")
	fakeDocker := filepath.Join(dir, "docker")
	script := "#!/bin/sh\nfor a in \"$@\"; do printf '%s\\n' \"$a\"; done > " + argvFile + "\necho hello\n"
	if err := os.WriteFile(fakeDocker, []byte(script), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}
	e := &PersistentDockerExecutor{
		dockerPath: fakeDocker,
		available:  true,
		config:     DefaultContainerPoolConfig(),
		containers: map[string]*PersistentContainer{"ctr0001": {ID: "ctr0001"}},
		snapshots:  map[string]*ContainerSnapshot{},
	}
	var events []AuditEventType
	e.SetAuditCallback(func(ev AuditEvent) { events = append(events, ev.Type) })

	result, err := e.ExecInContainer(context.Background(), ContainerExecOptions{
		ContainerID: "ctr0001", Binary: "sh", Arguments: []string{"-c", "echo hi"}, WorkingDir: "/w",
	})
	if err != nil || result.ExitCode != 0 || strings.TrimSpace(result.Stdout) != "hello" {
		t.Fatalf("exec: result=%+v err=%v", result, err)
	}
	raw, err := os.ReadFile(argvFile)
	if err != nil {
		t.Fatalf("read argv: %v", err)
	}
	got := strings.Split(strings.TrimSpace(string(raw)), "\n")
	want := []string{"exec", "-w", "/w", "--", "ctr0001", "sh", "-c", "echo hi"}
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("docker argv = %q, want %q", got, want)
	}
	if len(events) != 2 || events[0] != AuditEventStart || events[1] != AuditEventComplete {
		t.Fatalf("audit events = %v, want [start complete]", events)
	}
}

// IdleTimeout was configured and never enforced (gap G-P2-3): an abandoned
// environment held its container until the process exited. The health pass
// now reaps a container idle past the timeout and leaves a busy one alone.
func TestPersistentDockerReapsIdleContainers(t *testing.T) {
	fakeDocker := filepath.Join(t.TempDir(), "docker")
	if err := os.WriteFile(fakeDocker, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("write fake docker: %v", err)
	}

	now := time.Now()
	cfg := DefaultContainerPoolConfig()
	cfg.IdleTimeout = 10 * time.Minute
	e := &PersistentDockerExecutor{
		dockerPath: fakeDocker,
		available:  true,
		config:     cfg,
		containers: map[string]*PersistentContainer{
			"idlecontainer01": {ID: "idlecontainer01", CreatedAt: now.Add(-2 * time.Hour), LastExecAt: now.Add(-time.Hour)},
			"busycontainer01": {ID: "busycontainer01", CreatedAt: now.Add(-2 * time.Hour), LastExecAt: now.Add(-time.Minute)},
			"neverexecuted01": {ID: "neverexecuted01", CreatedAt: now.Add(-time.Hour)},
		},
		snapshots: map[string]*ContainerSnapshot{},
	}

	e.performHealthChecks()

	e.mu.RLock()
	defer e.mu.RUnlock()
	if _, ok := e.containers["idlecontainer01"]; ok {
		t.Error("container idle for an hour past a 10m IdleTimeout was not reaped")
	}
	if _, ok := e.containers["neverexecuted01"]; ok {
		t.Error("container never used since creation an hour ago was not reaped")
	}
	if _, ok := e.containers["busycontainer01"]; !ok {
		t.Error("container used a minute ago was reaped")
	}
}
