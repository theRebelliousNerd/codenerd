package campaign

import (
	"testing"
	"time"

	"codenerd/internal/core"
)

// The rolling-wave refresh routes refresh_scope with the workspace directory
// as its target and no payload. The constitution permits a safe action only as
// a pending_action with that exact target and payload, so this pins the fact
// triggerRollingWave now asserts: with it, a real kernel permits the refresh;
// without it, it does not (every refresh was denied before 2026-09-21, so no
// campaign refreshed its world model between phases).
func TestRollingWaveScopeRefreshIsPermittedOnlyAsAPendingAction(t *testing.T) {
	workspace := t.TempDir()
	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatalf("new kernel: %v", err)
	}
	vs := core.NewVirtualStore(nil)
	vs.SetKernel(kernel)

	if vs.CheckKernelPermitted("/refresh_scope", workspace, nil) {
		t.Fatal("refresh_scope was permitted with no pending_action: the default-deny premise this test rests on is gone")
	}

	pending := core.Fact{
		Predicate: "pending_action",
		Args:      []any{"rolling-wave-test", core.MangleAtom("/refresh_scope"), workspace, "{}", time.Now().Unix()},
	}
	if err := kernel.Assert(pending); err != nil {
		t.Fatalf("assert pending_action: %v", err)
	}
	if !vs.CheckKernelPermitted("/refresh_scope", workspace, nil) {
		t.Fatal("refresh_scope on the workspace directory is not permitted even as a pending_action: the rolling-wave refresh cannot run")
	}
}
