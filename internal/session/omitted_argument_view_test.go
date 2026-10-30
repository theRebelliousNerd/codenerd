package session

import (
	"context"
	"slices"
	"strings"
	"testing"

	"codenerd/internal/tools"
	"codenerd/internal/types"
)

// The registry leaves out an optional non-string argument a model sent as
// "null"; the session's write guards read the raw map first and refused it
// (a delete_element with paths "null": "paths must be an array of strings")
// before the registry could read it as omitted. Every layer now reads the
// registry's view.
func TestExecuteAndRecordToolCall_GuardsReadTheRegistrysArgumentView(t *testing.T) {
	reg := tools.NewRegistry()
	var executed map[string]any
	if err := reg.Register(&tools.Tool{
		Name: "delete_element", Description: "delete an element", Effect: tools.EffectWrite, Capability: tools.CapabilityOverlapping,
		Schema: tools.ToolSchema{
			Required: []string{"path", "ref"},
			Properties: map[string]tools.Property{
				"path":  {Type: "string", Description: "file"},
				"ref":   {Type: "string", Description: "element"},
				"paths": {Type: "array", Description: "use paths", Items: &tools.PropertyItems{Type: "string"}},
			},
		},
		Execute: func(_ context.Context, args map[string]any) (string, error) {
			executed = args
			return "deleted", nil
		},
	}); err != nil {
		t.Fatal(err)
	}
	e := newObligationExec(t)
	e.config.EnableSafetyGate = false
	e.config.WorkspaceRoot = t.TempDir()
	e.SetModularTools(reg)

	var guarded []string
	ctx := WithWriteGuard(context.Background(), func(_ context.Context, paths []string) error {
		guarded = paths
		return nil
	})
	// The executive gate is not wired in this fixture, so the call stops
	// there; what matters is that it got past the guards that read the
	// arguments, which refused it before.
	_, err := e.executeAndRecordToolCall(ctx, types.ToolCall{ID: "1", Name: "delete_element",
		Input: map[string]any{"path": "example.go", "ref": "unused", "paths": "null"}},
		validCapabilityTestConfig("delete_element"), &ExecutionResult{})
	if err != nil && (strings.Contains(err.Error(), "could not be named") || strings.Contains(err.Error(), "array")) {
		t.Fatalf("a call the registry accepts was refused by a guard reading the raw arguments: %v", err)
	}
	_ = executed
	if !slices.ContainsFunc(guarded, func(p string) bool { return strings.HasSuffix(p, "example.go") }) {
		t.Fatalf("the write guard saw targets %v, want example.go", guarded)
	}
}
