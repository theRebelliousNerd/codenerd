//go:build !windows

package tactile

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

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
