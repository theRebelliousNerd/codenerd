package mangle

import (
	"context"
	"testing"
)

func newControlProbeEngine(t *testing.T) *Engine {
	t.Helper()
	cfg := DefaultConfig()
	cfg.AutoEval = true
	e, err := NewEngine(cfg, nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	schema := `Decl ctl(Flag, N) bound [/name, /number].
Decl ctl_stop(Reason) bound [/name].
ctl_stop(/failures) :- ctl(_, N), N >= 3.
ctl_stop(/cycle) :- ctl(/yes, _).`
	if err := e.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	t.Cleanup(func() { _ = e.Close() })
	return e
}

func controlStops(t *testing.T, e *Engine) []string {
	t.Helper()
	res, err := e.Query(context.Background(), "ctl_stop(R)")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return bindingValues(res)
}

// A control fact is replaced, not accumulated, and what was derived from the
// old fact is gone once the fact is. ReplaceFactsForFile keys facts by their
// first string argument, so ctl(/no, 3) was never removed by it; and
// evaluation is monotone, so ctl_stop(/failures) stayed derived after the
// count went back to zero.
func TestReplaceControlFactsRetractsWhatNoLongerHolds(t *testing.T) {
	e := newControlProbeEngine(t)

	if err := e.ReplaceControlFacts([]Fact{{Predicate: "ctl", Args: []any{"/no", int64(3)}}}, "ctl"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := controlStops(t, e); len(got) != 1 || got[0] != "/failures" {
		t.Fatalf("stops after three failures = %v, want [/failures]", got)
	}

	if err := e.ReplaceControlFacts([]Fact{{Predicate: "ctl", Args: []any{"/no", int64(0)}}}, "ctl"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := controlStops(t, e); len(got) != 0 {
		t.Fatalf("stops after the count went back to zero = %v, want none: the old derivation survived", got)
	}
	facts, err := e.GetFacts("ctl")
	if err != nil {
		t.Fatalf("GetFacts: %v", err)
	}
	if len(facts) != 1 {
		t.Fatalf("ctl facts = %d, want 1: the old control fact accumulated instead of being replaced", len(facts))
	}

	// A name argument passed as a "/"-prefixed string is a name, so the
	// atom-matching rule fires.
	if err := e.ReplaceControlFacts([]Fact{{Predicate: "ctl", Args: []any{"/yes", int64(0)}}}, "ctl"); err != nil {
		t.Fatalf("replace: %v", err)
	}
	if got := controlStops(t, e); len(got) != 1 || got[0] != "/cycle" {
		t.Fatalf("stops for a cycle = %v, want [/cycle]", got)
	}
}
