package tools

import (
	"context"
	"slices"
	"testing"
)

func noopTool(name string) *Tool {
	return &Tool{
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
	if Global() != Global() {
		t.Error("Global() should return the same instance each call")
	}
	MustRegisterGlobal(noopTool("global_probe_tool"))
	if !Global().Has("global_probe_tool") {
		t.Error("MustRegisterGlobal should register into the global registry")
	}
}
