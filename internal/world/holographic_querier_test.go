package world

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/core"
)

// stubQuerier is the whole kernel dependency a holographic provider needs,
// which is the point of narrowing it: context can now be exercised without
// booting a kernel.
type stubQuerier struct{ facts map[string][]core.Fact }

func (s *stubQuerier) Query(pred string) ([]core.Fact, error) { return s.facts[pred], nil }

// TestHolographicProvider_WhenGivenStubQuerier_ShouldUseIt proves the provider
// depends only on Query, not on *core.RealKernel.
func TestHolographicProvider_WhenGivenStubQuerier_ShouldUseIt(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "svc.go")
	if err := os.WriteFile(target, []byte("package svc\n\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	q := &stubQuerier{facts: map[string][]core.Fact{
		"code_defines": {{Predicate: "code_defines", Args: []any{target, "svc.Run", "/function", int64(3), int64(3)}}},
		"code_calls":   {{Predicate: "code_calls", Args: []any{"svc.Run", "svc.Helper"}}},
	}}
	h := NewHolographicProvider(q, dir)
	ctx, err := h.GetContext(target)
	if err != nil {
		t.Fatalf("GetContext: %v", err)
	}
	if len(ctx.CallGraph) == 0 {
		t.Errorf("provider did not consult the querier: call graph is empty")
	}
}

// TestHolographicProvider_WhenKernelIsTypedNil_ShouldDegradeNotPanic — callers
// pass a *core.RealKernel that can be nil. Stored in an interface field, a nil
// pointer is a non-nil interface, so the existing `h.kernel == nil` guards would
// stop guarding and the first query would panic.
func TestHolographicProvider_WhenKernelIsTypedNil_ShouldDegradeNotPanic(t *testing.T) {
	dir := t.TempDir()
	target := filepath.Join(dir, "svc.go")
	if err := os.WriteFile(target, []byte("package svc\n\nfunc Run() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	var typedNil *core.RealKernel
	h := NewHolographicProvider(typedNil, dir)
	if h.kernel != nil {
		t.Fatal("typed-nil kernel was not flattened to a nil interface")
	}
	if _, err := h.GetContext(target); err != nil {
		t.Fatalf("GetContext with typed-nil kernel: %v", err)
	}
	if _, err := h.BuildWithImpactPriorities(t.Context(), target); err != nil {
		t.Fatalf("BuildWithImpactPriorities with typed-nil kernel: %v", err)
	}
}
