package shards

import (
	"codenerd/internal/core"
	"testing"
)

func TestCompletionIntegrationRequiresLoadedPathAndBehavioralWitness(t *testing.T) {
	dm := buildProductionDerivationMap(t, SharedPredicates())
	path := core.IntegrationPath{Name: "acceptance to completion", Input: "turn_acceptance", InputArity: 3, Output: "turn_done", OutputArity: 1, Witness: "completion_gate"}
	// The witness exercises the loaded production rules, independently of the
	// static map. Removing the acceptance operand must fail the negative case.
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := k.Assert(core.Fact{Predicate: "turn_evidence", Args: []any{core.MangleAtom("/fix"), int64(1), int64(1), int64(0), core.MangleAtom("/false"), core.MangleAtom("/false")}}); err != nil {
		t.Fatal(err)
	}
	if facts, err := k.Query("turn_done"); err != nil || len(facts) != 0 {
		t.Fatalf("missing acceptance derived done: %v %v", facts, err)
	}
	if err := k.Assert(core.Fact{Predicate: "turn_acceptance", Args: []any{core.MangleAtom("/fix"), "contract", "snapshot"}}); err != nil {
		t.Fatal(err)
	}
	if facts, err := k.Query("turn_done"); err != nil || len(facts) != 1 {
		t.Fatalf("accepted turn failed: %v %v", facts, err)
	}
	witnesses := []core.IntegrationWitness{{Name: "completion_gate", Snapshot: "snapshot", Passed: true}}
	if err := dm.CheckIntegrationPath(path, "snapshot", witnesses); err != nil {
		t.Fatal(err)
	}
	if err := dm.CheckIntegrationPath(path, "later-snapshot", witnesses); err == nil {
		t.Fatal("stale witness accepted")
	}
	wrong := path
	wrong.InputArity = 4
	if err := dm.CheckIntegrationPath(wrong, "snapshot", witnesses); err == nil {
		t.Fatal("representation mismatch accepted")
	}
	delete(dm.Arities, "turn_acceptance")
	if err := dm.CheckIntegrationPath(path, "snapshot", witnesses); err == nil {
		t.Fatal("unloaded producer accepted")
	}
}
