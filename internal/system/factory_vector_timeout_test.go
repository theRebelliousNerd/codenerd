package system

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// jit.vector_search_timeout_ms reaches the booted JIT compiler; before it was
// a config key, every compile ran on a bound no one could set.
func TestBootJITCompilerTakesTheConfiguredVectorSearchTimeout(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	nerdDir := filepath.Join(ws, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), []byte(`{"jit":{"vector_search_timeout_ms":2345}}`), 0o600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)
	if cortex.JITCompiler == nil {
		t.Fatal("boot built no JIT compiler")
	}
	if got := cortex.JITCompiler.GetConfig().VectorSearchTimeout; got != 2345*time.Millisecond {
		t.Fatalf("compiler vector search timeout = %v, want the configured 2.345s", got)
	}
}
