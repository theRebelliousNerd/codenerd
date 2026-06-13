package system

import (
	"testing"

	"codenerd/internal/core"
)

// TestMCPKernelAdapter_AssertQueryRetract drives the MCP kernel adapter against
// a real Mangle kernel: a string fact is parsed and loaded, queried back through
// the pattern matcher, then retracted — a full cross-boundary round trip.
func TestMCPKernelAdapter_AssertQueryRetract(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	kernel.SetSchemas("Decl mcp_tool(Name, Domain).")
	kernel.SetPolicy("")

	a := newMCPKernelAdapter(kernel)

	if err := a.Assert(`mcp_tool("echo", "util")`); err != nil {
		t.Fatalf("Assert: %v", err)
	}

	results, err := a.Query(`mcp_tool("echo", "util")`)
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(results) != 1 {
		t.Fatalf("expected 1 result after Assert, got %d: %v", len(results), results)
	}

	if err := a.Retract(`mcp_tool("echo", "util")`); err != nil {
		t.Fatalf("Retract: %v", err)
	}
	after, err := a.Query(`mcp_tool("echo", "util")`)
	if err != nil {
		t.Fatalf("Query after retract: %v", err)
	}
	if len(after) != 0 {
		t.Errorf("expected 0 results after Retract, got %d: %v", len(after), after)
	}
}

// TestMCPKernelAdapter_AssertInvalidFact ensures a malformed fact string surfaces
// a parse error rather than silently succeeding.
func TestMCPKernelAdapter_AssertInvalidFact(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	a := newMCPKernelAdapter(kernel)
	if err := a.Assert(`this is not a fact (((`); err == nil {
		t.Error("expected a parse error for a malformed fact string")
	}
}
