package system

import (
	"context"
	"strings"
	"testing"

	"codeberg.org/TauCeti/mangle-go/analysis"

	"codenerd/internal/core"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

// gateStubKernel is a minimal kernel double with no Dreamer backing. Any
// destructive preflight against a VirtualStore using it must fail closed with
// a "dreamer unavailable" verdict, which is exactly the verdict the adapter
// is required to surface.
type gateStubKernel struct {
	asserted []types.Fact
}

func (k *gateStubKernel) LoadFacts([]types.Fact) error { return nil }
func (k *gateStubKernel) Query(string) ([]types.Fact, error) {
	return nil, nil
}
func (k *gateStubKernel) QueryAll() (map[string][]types.Fact, error) { return nil, nil }
func (k *gateStubKernel) Assert(f types.Fact) error {
	k.asserted = append(k.asserted, f)
	return nil
}
func (k *gateStubKernel) AssertBatch(facts []types.Fact) error {
	k.asserted = append(k.asserted, facts...)
	return nil
}
func (k *gateStubKernel) Retract(string) error                     { return nil }
func (k *gateStubKernel) RetractFact(types.Fact) error              { return nil }
func (k *gateStubKernel) UpdateSystemFacts() error                  { return nil }
func (k *gateStubKernel) GetProgramInfo() *analysis.ProgramInfo     { return nil }
func (k *gateStubKernel) Reset()                                    {}
func (k *gateStubKernel) AppendPolicy(string)                       {}
func (k *gateStubKernel) RetractExactFactsBatch([]types.Fact) error { return nil }
func (k *gateStubKernel) RemoveFactsByPredicateSet(map[string]struct{}) error {
	return nil
}

func TestSessionVirtualStoreAdapterGate(t *testing.T) {
	ctx := context.Background()

	// (a) The production adapter must satisfy the executive gate interface.
	// The compile-time assertion in factory_adapters.go already enforces this;
	// the runtime check guards against a future method-signature drift that
	// still compiles but breaks the executor's type assertion.
	vs := core.NewVirtualStoreWithConfig(nil, core.DefaultVirtualStoreConfig())
	vs.SetKernel(&gateStubKernel{})
	adapter := &sessionVirtualStoreAdapter{vs: vs}
	if _, ok := any(adapter).(session.InteractiveExecutiveGate); !ok {
		t.Fatal("sessionVirtualStoreAdapter does not implement session.InteractiveExecutiveGate")
	}

	// (b) Preflight for a destructive tool returns the store's verdict.
	// With a kernel double and no Dreamer, the store fails closed, so the
	// adapter must surface that block rather than allow.
	err := adapter.PreflightDestructiveToolCall(ctx, "gate-test-write", "write_file", map[string]any{
		"path":    "blocked.go",
		"content": "package blocked",
	})
	if err == nil || !strings.Contains(err.Error(), "dreamer unavailable") {
		t.Fatalf("PreflightDestructiveToolCall(write_file) = %v, want fail-closed dreamer error", err)
	}
	// A read tool is non-destructive and must pass through.
	if err := adapter.PreflightDestructiveToolCall(ctx, "gate-test-read", "read_file", map[string]any{
		"path": "safe.go",
	}); err != nil {
		t.Fatalf("PreflightDestructiveToolCall(read_file) = %v, want nil", err)
	}
	// Post-action validation with success=false runs no validators.
	if err := adapter.ValidateInteractiveToolResult(ctx, "gate-test-val", "write_file", map[string]any{
		"path": "blocked.go",
	}, "", false); err != nil {
		t.Fatalf("ValidateInteractiveToolResult(success=false) = %v, want nil", err)
	}

	// (c) A nil store fails closed for a destructive tool and passes for a
	// read tool, matching the core gate's documented fail-closed policy.
	nilAdapter := &sessionVirtualStoreAdapter{vs: nil}
	if err := nilAdapter.PreflightDestructiveToolCall(ctx, "nil-write", "write_file", map[string]any{
		"path": "blocked.go",
	}); err == nil {
		t.Fatal("nil-store PreflightDestructiveToolCall(write_file) = nil, want fail-closed error")
	}
	if err := nilAdapter.PreflightDestructiveToolCall(ctx, "nil-read", "read_file", map[string]any{
		"path": "safe.go",
	}); err != nil {
		t.Fatalf("nil-store PreflightDestructiveToolCall(read_file) = %v, want nil", err)
	}
	if err := nilAdapter.ValidateInteractiveToolResult(ctx, "nil-val", "write_file", map[string]any{
		"path": "blocked.go",
	}, "", true); err != nil {
		t.Fatalf("nil-store ValidateInteractiveToolResult = %v, want nil", err)
	}
	// A nil receiver must behave like a nil store, never panic.
	var nilRecv *sessionVirtualStoreAdapter
	if err := nilRecv.PreflightDestructiveToolCall(ctx, "nil-recv-write", "write_file", map[string]any{
		"path": "blocked.go",
	}); err == nil {
		t.Fatal("nil-receiver PreflightDestructiveToolCall(write_file) = nil, want fail-closed error")
	}
}
