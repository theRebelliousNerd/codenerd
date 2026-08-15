package system

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/tools"
)

// TestInitCoreComponents_ShouldBindTheToolContainmentRoot is the regression
// test for a feature that existed, was tested, and was dead on the path that
// mattered.
//
// The containment boundary for the modular file tools was bound inside
// resolveWorkspaceRoot, which only the GetOrBootCortex wrapper called — the
// one-shot CLI path. Interactive chat and every shard boot through
// BootCortexWithConfig, which inlined its own copy of the workspace-resolution
// cascade and omitted the binding. So `nerd -w <dir> chat` booted Cortex
// against <dir> while its tools still resolved relative paths under the process
// CWD: a glob in that session listed the repo the binary happened to be run
// from, not the workspace the user asked for.
//
// Nothing failed. The tools worked, they just worked on the wrong tree, which
// is why this needs a test that watches the binding rather than the boot.
//
// initCoreComponents is the first boot step and the only place bctx.workspace
// is established, so exercising it directly covers every caller of
// bootCortexWithSteps without standing up a full Cortex.
func TestInitCoreComponents_ShouldBindTheToolContainmentRoot(t *testing.T) {
	// The binding is process-global; put it back so later tests in this package
	// are not run inside a deleted TempDir.
	prevEnv, hadEnv := os.LookupEnv("CODENERD_WORKSPACE_ROOT")
	prevRoot := tools.Global().WorkspaceRoot()
	t.Cleanup(func() {
		if hadEnv {
			_ = os.Setenv("CODENERD_WORKSPACE_ROOT", prevEnv)
		} else {
			_ = os.Unsetenv("CODENERD_WORKSPACE_ROOT")
		}
		tools.SetGlobalWorkspaceRoot(prevRoot)
	})

	ws := t.TempDir()
	// t.TempDir can hand back a symlinked path (/var -> /private/var on macOS);
	// compare against the same normalization the binding applies.
	wantAbs, err := filepath.Abs(ws)
	if err != nil {
		t.Fatalf("abs: %v", err)
	}

	_ = os.Unsetenv("CODENERD_WORKSPACE_ROOT")
	tools.SetGlobalWorkspaceRoot("")

	bctx := &bootContext{cfg: BootConfig{Workspace: ws}}
	if err := initCoreComponents(bctx); err != nil {
		t.Fatalf("initCoreComponents: %v", err)
	}
	if bctx.tracker != nil {
		t.Cleanup(func() { _ = bctx.tracker.Close() })
	}

	if bctx.workspace != wantAbs {
		t.Errorf("bctx.workspace = %q, want the absolute workspace %q", bctx.workspace, wantAbs)
	}
	if got := tools.Global().WorkspaceRoot(); got != wantAbs {
		t.Errorf("tool containment root = %q, want %q — file tools would resolve "+
			"relative paths outside the booted workspace", got, wantAbs)
	}
	if got := os.Getenv("CODENERD_WORKSPACE_ROOT"); got != wantAbs {
		t.Errorf("CODENERD_WORKSPACE_ROOT = %q, want %q — the guard's env fallback "+
			"is what covers tools reached outside a registry", got, wantAbs)
	}
}
