package system

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
)

// Through the production adapter and the real kernel: a repeated context
// over an unchanged kernel reuses the dynamic-snapshot preflight and hits the
// cache; a live-kernel write that feeds the injected context changes the
// identity, so the next compile re-snapshots and serves the new evidence.
func TestKernelAdapter_PreflightReuseFollowsTheKernelGeneration(t *testing.T) {
	live, adapter := newPromptScopeTestKernel(t)
	compiler, err := prompt.NewJITPromptCompiler(
		prompt.WithEmbeddedCorpus(promptScopeTestCorpus()),
		prompt.WithKernel(adapter),
	)
	if err != nil {
		t.Fatalf("NewJITPromptCompiler: %v", err)
	}
	t.Cleanup(func() { _ = compiler.Close() })

	identity, ok := adapter.SnapshotIdentity()
	if !ok || !identity.Reusable {
		t.Fatalf("the default kernel program must yield a reusable identity, got %+v (ok=%v)", identity, ok)
	}

	newContext := func() *prompt.CompilationContext {
		return prompt.NewCompilationContext().WithLanguage("/go").WithShard("/coder", "", "")
	}
	ctx := context.Background()
	if _, err := compiler.Compile(ctx, newContext()); err != nil {
		t.Fatalf("first compile: %v", err)
	}
	second, err := compiler.Compile(ctx, newContext())
	if err != nil {
		t.Fatalf("second compile: %v", err)
	}
	if !second.Stats.CacheHit {
		t.Fatal("unchanged kernel and context must hit the cache")
	}
	if got := compiler.GetStats().PreflightReuses; got != 1 {
		t.Fatalf("PreflightReuses = %d, want 1 (the hit must not clone the kernel)", got)
	}

	if err := live.Assert(core.Fact{Predicate: "injectable_context", Args: []any{"/_all", "FRESH_KERNEL_EVIDENCE"}}); err != nil {
		t.Fatalf("Assert: %v", err)
	}
	third, err := compiler.Compile(ctx, newContext())
	if err != nil {
		t.Fatalf("third compile: %v", err)
	}
	if third.Stats.CacheHit || third.Stats.PreflightReused {
		t.Fatalf("a kernel write must invalidate both the memo and the cache (hit=%v reused=%v)",
			third.Stats.CacheHit, third.Stats.PreflightReused)
	}
	if !strings.Contains(third.Prompt, "FRESH_KERNEL_EVIDENCE") {
		t.Fatalf("new kernel evidence missing from the recompiled prompt:\n%s", third.Prompt)
	}
	assertNoLivePromptFacts(t, live)
}
