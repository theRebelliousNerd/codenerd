package tools

import (
	"context"
	"slices"
	"testing"
)

func noopTool(name string) *Tool {
	return &Tool{
		Effect:  EffectRead,
		Name:    name,
		Execute: func(ctx context.Context, args map[string]any) (string, error) { return "", nil },
	}
}

func TestRegistryHasGetMultipleNames(t *testing.T) {
	r := NewRegistry()
	for _, n := range []string{"beta", "alpha", "gamma"} {
		if err := r.Register(noopTool(n)); err != nil {
			t.Fatalf("Register(%s): %v", n, err)
		}
	}

	if !r.Has("alpha") || r.Has("missing") {
		t.Error("Has should report registered tools and only those")
	}

	got := r.GetMultiple([]string{"alpha", "gamma", "missing"})
	if len(got) != 2 {
		t.Fatalf("GetMultiple returned %d tools, want 2 (missing skipped)", len(got))
	}

	names := r.Names()
	if !slices.Equal(names, []string{"alpha", "beta", "gamma"}) {
		t.Errorf("Names()=%v, want sorted [alpha beta gamma]", names)
	}
}

func TestGlobalRegistrySingleton(t *testing.T) {
	if Global() == nil {
		t.Fatal("Global() should return a non-nil registry")
	}
	// Global() is a stable singleton.
	// Global() must return one shared registry; two calls yielding different
	// instances would silently give each caller its own tool set.
	firstGlobal, secondGlobal := Global(), Global()
	if firstGlobal != secondGlobal {
		t.Error("Global() should return the same instance each call")
	}
	// Unregister on cleanup, or this test only works the first time it runs in
	// a process: MustRegisterGlobal panics on a duplicate name, so the second
	// run of the suite dies here. Same defect as the 25 session tests, same
	// fix.
	const probe = "global_probe_tool"
	t.Cleanup(func() { Global().Unregister(probe) })

	MustRegisterGlobal(noopTool(probe))
	if !Global().Has(probe) {
		t.Error("MustRegisterGlobal should register into the global registry")
	}
}
