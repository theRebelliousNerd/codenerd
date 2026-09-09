package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// seedImpactChain asserts the EDB the impact rules join on:
//
//	modified_function(Func, File) + code_calls(Caller, Func) -> impact_caller/2
//	impact_caller -> impact_graph(Target, Caller, 1)
//	impact_graph + code_defines -> context_priority_file(File, Caller, 3)
//
// (internal/core/defaults/policy/impact.mg:29-78)
func seedImpactChain(t *testing.T, k *core.RealKernel, callerFile string) {
	t.Helper()
	facts := []core.Fact{
		{Predicate: "modified_function", Args: []any{"Target", "target.go"}},
		{Predicate: "code_calls", Args: []any{"DirectCaller", "Target"}},
		{Predicate: "code_calls", Args: []any{"GrandCaller", "DirectCaller"}},
		{Predicate: "code_defines", Args: []any{callerFile, "DirectCaller", "/function", int64(1), int64(9)}},
		{Predicate: "code_defines", Args: []any{callerFile, "GrandCaller", "/function", int64(11), int64(19)}},
	}
	for _, f := range facts {
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
}

// TestImpactChain_ReachesPromptSection is the end-to-end regression test for a
// feature that was built, tested in isolation, and never ran.
//
// Every link existed: impact.mg derived the chain, BuildWithImpactPriorities
// consumed it, PromptSection rendered it. Three links were cut — nothing
// produced modified_function, BuildWithImpactPriorities had no production
// caller, and so PromptSection's prioritized-callers branch was unreachable.
// This test drives the real kernel rules through the real renderer, so any one
// of those cuts reappearing fails here.
func TestImpactChain_ReachesPromptSection(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package p\n\n// Target is the edited symbol.\nfunc Target() error { return nil }\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	seedImpactChain(t, kernel, filepath.Join(dir, "callers.go"))

	// The kernel must actually derive the chain, or the rest of the test is
	// asserting against an empty relation and would pass vacuously.
	derived, err := kernel.Query("context_priority_file")
	if err != nil {
		t.Fatalf("query context_priority_file: %v", err)
	}
	if len(derived) == 0 {
		t.Fatal("impact.mg derived no context_priority_file facts from a seeded modified_function + code_calls; the chain is broken upstream of the renderer")
	}

	h := NewHolographicProvider(kernel, dir)
	hc, err := h.GetContextWithContext(context.Background(), target)
	if err != nil {
		t.Fatalf("GetContextWithContext: %v", err)
	}
	if len(hc.PrioritizedCallers) == 0 {
		t.Fatal("holographic context has no PrioritizedCallers; applyImpactPriorities is not wired into the context path")
	}

	// A direct caller is depth 1, which impact.mg encodes as priority 3 and
	// impactPriorityToScale maps to 100. Before that conversion existed every
	// caller rendered as MINIMAL.
	var direct *PrioritizedCaller
	for i := range hc.PrioritizedCallers {
		if hc.PrioritizedCallers[i].Name == "DirectCaller" {
			direct = &hc.PrioritizedCallers[i]
		}
	}
	if direct == nil {
		t.Fatalf("DirectCaller missing from prioritized callers: %+v", hc.PrioritizedCallers)
	}
	if direct.Priority != 100 || direct.Depth != 1 {
		t.Errorf("direct caller priority/depth = %d/%d, want 100/1 (impact.mg emits 3 for depth 1)", direct.Priority, direct.Depth)
	}
	if hc.ImpactPriority != 100 {
		t.Errorf("ImpactPriority = %d, want 100", hc.ImpactPriority)
	}

	// Ranking must put the direct caller ahead of the grandcaller.
	if hc.PrioritizedCallers[0].Name != "DirectCaller" {
		t.Errorf("ranking put %q first; the direct caller must outrank the grandcaller", hc.PrioritizedCallers[0].Name)
	}

	section := h.PromptSection(context.Background(), target)
	if !strings.Contains(section, "Callers (impact-prioritized)") {
		t.Fatalf("PromptSection fell through to the unranked branch:\n%s", section)
	}
	if !strings.Contains(section, "DirectCaller") {
		t.Errorf("rendered section omits the direct caller:\n%s", section)
	}
	if !strings.Contains(section, "priority 100") {
		t.Errorf("rendered section omits the converted priority:\n%s", section)
	}
}

// TestImpactChain_QuietWithoutModifications is the other half of the contract.
// The chain must produce nothing until something has actually been modified —
// otherwise every turn would carry an impact block built from stale facts.
func TestImpactChain_QuietWithoutModifications(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "target.go")
	if err := os.WriteFile(target, []byte("package p\n\nfunc Target() error { return nil }\n"), 0o644); err != nil {
		t.Fatalf("write target: %v", err)
	}

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("kernel: %v", err)
	}
	// code_calls exists, but nothing was modified.
	if err := kernel.Assert(core.Fact{Predicate: "code_calls", Args: []any{"DirectCaller", "Target"}}); err != nil {
		t.Fatalf("assert: %v", err)
	}

	h := NewHolographicProvider(kernel, dir)
	hc, err := h.GetContextWithContext(context.Background(), target)
	if err != nil {
		t.Fatalf("GetContextWithContext: %v", err)
	}
	if len(hc.PrioritizedCallers) != 0 {
		t.Fatalf("impact ranking fired with no modified_function: %+v", hc.PrioritizedCallers)
	}
	if hc.ImpactPriority != 0 {
		t.Errorf("ImpactPriority = %d with nothing modified, want 0", hc.ImpactPriority)
	}
}

// TestImpactPriorityToScale pins the conversion that made every Mangle-derived
// caller render as MINIMAL: impact.mg emits 4-Depth (3, 2, 1) while every Go
// renderer buckets on 80/50/25.
func TestImpactPriorityToScale(t *testing.T) {
	cases := []struct {
		raw            int
		wantPrio       int
		wantDepth      int
		wantAtLeastMed bool
	}{
		{raw: 3, wantPrio: 100, wantDepth: 1, wantAtLeastMed: true},
		{raw: 2, wantPrio: 70, wantDepth: 2, wantAtLeastMed: true},
		{raw: 1, wantPrio: 40, wantDepth: 3},
		{raw: 0, wantPrio: 50, wantDepth: 1},
		// Producers already speaking the 0-100 scale pass through unchanged.
		{raw: 80, wantPrio: 80, wantDepth: 1, wantAtLeastMed: true},
	}
	for _, tc := range cases {
		gotPrio, gotDepth := impactPriorityToScale(tc.raw)
		if gotPrio != tc.wantPrio || gotDepth != tc.wantDepth {
			t.Errorf("impactPriorityToScale(%d) = %d/%d, want %d/%d", tc.raw, gotPrio, gotDepth, tc.wantPrio, tc.wantDepth)
		}
		if tc.wantAtLeastMed && priorityLevelString(gotPrio) == "MINIMAL" {
			t.Errorf("priority %d (from raw %d) still renders as MINIMAL", gotPrio, tc.raw)
		}
	}
}

// TestRankPrioritizedCallers_StableAndCapped pins two properties the prompt
// path depends on: a deterministic order (a section that shuffles between turns
// throws away prompt-cache hits) and a hard cap on how many callers can reach
// the model.
func TestRankPrioritizedCallers_StableAndCapped(t *testing.T) {
	var callers []PrioritizedCaller
	for i := 0; i < maxPrioritizedCallers*3; i++ {
		callers = append(callers, PrioritizedCaller{Name: string(rune('a'+i%26)) + "fn", Priority: 50, Depth: 2})
	}
	callers = append(callers, PrioritizedCaller{Name: "zzTop", Priority: 100, Depth: 1})

	first := rankPrioritizedCallers(append([]PrioritizedCaller(nil), callers...))
	if len(first) != maxPrioritizedCallers {
		t.Fatalf("ranked list length = %d, want the %d cap", len(first), maxPrioritizedCallers)
	}
	if first[0].Name != "zzTop" {
		t.Errorf("highest priority did not sort first, got %q", first[0].Name)
	}

	second := rankPrioritizedCallers(append([]PrioritizedCaller(nil), callers...))
	for i := range first {
		if first[i].Name != second[i].Name {
			t.Fatalf("ranking is not deterministic at index %d: %q vs %q", i, first[i].Name, second[i].Name)
		}
	}
}
