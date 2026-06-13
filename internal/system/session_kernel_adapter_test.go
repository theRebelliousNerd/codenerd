package system

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// TestSessionKernelAdapter drives the session<->kernel adapter against a real
// Mangle kernel, covering the LoadFacts/Assert/AssertBatch/Query/RetractFact/
// Reset delegation surface end to end.
func TestSessionKernelAdapter(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	kernel.SetSchemas("Decl session_fact(Name, N).")
	kernel.SetPolicy("")

	a := &sessionKernelAdapter{kernel: kernel}

	// LoadFacts + Query round trip.
	if err := a.LoadFacts([]types.Fact{{Predicate: "session_fact", Args: []any{"a", 1}}}); err != nil {
		t.Fatalf("LoadFacts: %v", err)
	}
	facts, err := a.Query("session_fact")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("after LoadFacts expected 1 fact, got %d", len(facts))
	}

	// Assert + AssertBatch add more facts.
	if err := a.Assert(types.Fact{Predicate: "session_fact", Args: []any{"b", 2}}); err != nil {
		t.Fatalf("Assert: %v", err)
	}
	if err := a.AssertBatch([]types.Fact{{Predicate: "session_fact", Args: []any{"c", 3}}}); err != nil {
		t.Fatalf("AssertBatch: %v", err)
	}
	facts, _ = a.Query("session_fact")
	if len(facts) != 3 {
		t.Errorf("after asserts expected 3 facts, got %d", len(facts))
	}

	// QueryAll includes the predicate.
	all, err := a.QueryAll()
	if err != nil {
		t.Fatalf("QueryAll: %v", err)
	}
	if len(all["session_fact"]) != 3 {
		t.Errorf("QueryAll should report 3 session_fact rows, got %d", len(all["session_fact"]))
	}

	// RetractFact removes a specific fact.
	if err := a.RetractFact(types.Fact{Predicate: "session_fact", Args: []any{"b", 2}}); err != nil {
		t.Fatalf("RetractFact: %v", err)
	}
	facts, _ = a.Query("session_fact")
	if len(facts) != 2 {
		t.Errorf("after RetractFact expected 2 facts, got %d", len(facts))
	}

	// Reset clears everything.
	a.Reset()
	facts, _ = a.Query("session_fact")
	if len(facts) != 0 {
		t.Errorf("after Reset expected 0 facts, got %d", len(facts))
	}
}
