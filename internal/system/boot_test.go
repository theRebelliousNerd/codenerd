package system

import (
	"context"
	"os"
	"runtime"
	"strings"
	"testing"
	"time"
)

// TestBootCortexEndToEnd boots a full Cortex against a temp workspace with no
// API key. This exercises the entire factory assembly path (kernel, stores,
// virtual store, shard manager, session executor/spawner, JIT compiler, prompt
// assembler and all the adapter wiring) without making any LLM network calls.
func TestBootCortexEndToEnd(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()

	ws := t.TempDir()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	if cortex == nil {
		t.Fatal("BootCortex returned nil cortex")
	}

	checks := map[string]bool{
		"RealKernel":      cortex.RealKernel == nil,
		"Kernel":          cortex.Kernel == nil,
		"VirtualStore":    cortex.VirtualStore == nil,
		"ShardManager":    cortex.ShardManager == nil,
		"SessionExecutor": cortex.SessionExecutor == nil,
		"SessionSpawner":  cortex.SessionSpawner == nil,
		"JITCompiler":     cortex.JITCompiler == nil,
		"PromptAssembler": cortex.PromptAssembler == nil,
		"Transducer":      cortex.Transducer == nil,
		"LocalDB":         cortex.LocalDB == nil,
	}
	for name, isNil := range checks {
		if isNil {
			t.Errorf("Cortex.%s should be wired after boot, got nil", name)
		}
	}
	if cortex.Workspace == "" {
		t.Error("Cortex.Workspace should be set after boot")
	}
}

// requireWorkspaceReleased closes the cortex and requires every file it opened
// under the workspace to be released. On Windows an open handle blocks
// removal; a close step that times out (runCloseStep) is abandoned, not
// stopped, so its goroutine can hold a store past Close. That happened once in
// a loaded full suite (2026-09-19) -- surfacing only as TempDir's cleanup
// error on .nerd/knowledge.db with no clue to the holder -- and not in four
// boots beside 32 busy loops. When it happens again the failure names the
// holder: the stacks of every live codenerd goroutine. It then waits for the
// release so the cleanup error does not bury the finding.
func requireWorkspaceReleased(t *testing.T, cortex *Cortex, ws string) {
	t.Helper()
	closeErr := cortex.Close()
	err := os.RemoveAll(ws)
	if err == nil {
		return
	}
	buf := make([]byte, 1<<23)
	n := runtime.Stack(buf, true)
	var live []string
	for _, g := range strings.Split(string(buf[:n]), "\n\n") {
		if strings.Contains(g, "codenerd/") {
			live = append(live, g)
		}
	}
	t.Errorf("workspace still held after Cortex.Close (close error: %v): %v\n%d codenerd goroutine(s) alive:\n%s",
		closeErr, err, len(live), strings.Join(live, "\n\n"))
	start := time.Now()
	for time.Since(start) < time.Minute {
		time.Sleep(500 * time.Millisecond)
		if os.RemoveAll(ws) == nil {
			t.Logf("workspace released %v after Close returned", time.Since(start).Round(time.Millisecond))
			return
		}
	}
}
