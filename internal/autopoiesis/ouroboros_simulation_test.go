package autopoiesis

import (
	"context"
	"testing"

	"codenerd/internal/mangle"
)

// The Phase 3 gate must be about THIS proposal. It used to run through
// DifferentialEngine.Query, which never filtered by the query constant — it
// emitted every valid_transition row in the store — so a transition that had
// nothing to do with the tool under simulation satisfied the gate, and the
// query's own argument was spelled as a /name against a /string-bound
// predicate, so it could not have matched even if it had been checked.
//
// This pins both halves: a valid_transition for an unrelated step is not an
// answer for this one, and the simulation's own facts stay off the loop's real
// state machine. See Docs/journeys/impl/S23-differential-path.md.
func TestSimulateTransition_GateIsScopedToThisProposalAndIsolated(t *testing.T) {
	loop := NewOuroborosLoop(&MockLLMClient{}, DefaultOuroborosConfig(t.TempDir()))
	ctx := context.Background()

	// A perfectly valid transition belonging to some OTHER step, planted on
	// the loop's own engine. Nothing about it says anything about /step_probe.
	if err := loop.engine.AddFacts([]mangle.Fact{
		{Predicate: "state", Args: []any{"/step_other", stabilityScore(0.10), "1"}},
		{Predicate: "base_stability", Args: []any{"/step_other", stabilityScore(0.10)}},
		{Predicate: "state", Args: []any{"/step_other_next", stabilityScore(0.99), "2"}},
		{Predicate: "proposed", Args: []any{"/step_other_next"}},
		{Predicate: "base_stability", Args: []any{"/step_other_next", stabilityScore(0.99)}},
	}); err != nil {
		t.Fatalf("seed unrelated transition: %v", err)
	}

	need := &ToolNeed{Name: "probe", Confidence: 0.90}
	tool := &GeneratedTool{Name: "probe", Code: "package tools\n\nfunc Probe() {}\n"}

	result := &LoopResult{}
	if ok := loop.simulateTransition(ctx, "/step_probe", need, tool, result); !ok {
		t.Fatalf("simulation rejected a monotonic 0.00 -> 0.90 transition: %s", result.Error)
	}

	// Isolation: the hypothetical facts must not have reached loop.engine.
	rows, err := loop.engine.Query(ctx, `state(X, S, L)`)
	if err != nil {
		t.Fatalf("Query state: %v", err)
	}
	for _, row := range rows.Bindings {
		if got := row["X"]; got == "/step_probe" || got == "/step_probe_next" {
			t.Fatalf("simulation fact %v leaked into the loop's real state machine", got)
		}
	}
}
