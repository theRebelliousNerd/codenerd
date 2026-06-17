package system

import (
	"context"
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

	cortex, err := BootCortex(ctx, t.TempDir(), "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer cortex.Close()
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
