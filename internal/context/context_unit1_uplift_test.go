package context

import (
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// The budget is shared across the compressor's turn/context/meter paths, so it
// must be safe under concurrent use. Run the gates with -race: this test fails
// there if the lock ever regresses to caller discipline.
func TestTokenBudget_ConcurrentUse(t *testing.T) {
	tb := NewTokenBudget(DefaultConfig())
	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(w int) {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = tb.Allocate("atoms", 10)
				_ = tb.Allocate("working", 5)
				_ = tb.TotalUsed()
				_ = tb.Utilization()
				_ = tb.GetUsage()
				_ = tb.ShouldCompress()
				tb.Release("atoms", 3)
				if i%10 == 0 {
					tb.SetUsage(100*w, 200, 50, 25, 10)
				}
			}
		}(w)
	}
	wg.Wait()
	if got := tb.TotalUsed(); got < 0 {
		t.Errorf("TotalUsed = %d after concurrent use, want >= 0", got)
	}
}

// A zero budget used to report NaN utilization, which compares false against
// every threshold and silently disabled compression. It reports full now.
func TestTokenBudget_ZeroBudgetReportsFull(t *testing.T) {
	tb := NewTokenBudget(CompressorConfig{})
	if got := tb.Utilization(); got != 1.0 {
		t.Errorf("Utilization with zero budget = %v, want 1.0", got)
	}
	if !tb.ShouldCompress() {
		t.Error("ShouldCompress with zero budget = false, want true")
	}
}

// Negative amounts are caller bugs, not quiet refunds.
func TestTokenBudget_NegativeAmountsRejected(t *testing.T) {
	tb := NewTokenBudget(DefaultConfig())
	if tb.Allocate("atoms", -100) {
		t.Error("Allocate(negative) = true, want false")
	}
	if got := tb.TotalUsed(); got != 0 {
		t.Errorf("TotalUsed after negative allocate = %d, want 0", got)
	}
	tb.Release("atoms", -50)
	if got := tb.TotalUsed(); got != 0 {
		t.Errorf("TotalUsed after negative release = %d, want 0", got)
	}
}

func TestTokenBudget_SetUsageReplacesAtomically(t *testing.T) {
	tb := NewTokenBudget(DefaultConfig())
	tb.SetUsage(1, 2, 3, 4, 5)
	if got := tb.TotalUsed(); got != 15 {
		t.Errorf("TotalUsed = %d, want 15", got)
	}
	tb.SetUsage(-10, 0, 0, 0, 7)
	u := tb.GetUsage()
	if u.Core != 0 || u.Total != 7 {
		t.Errorf("usage after clamp = %+v, want core 0 total 7", u)
	}
}

// A MangleAtom is a distinct type from string: without its own case every
// atom — often a long path — cost a flat 3 tokens whatever its length.
func TestCountFact_MangleAtomScalesWithText(t *testing.T) {
	tc := NewTokenCounter()
	short := tc.CountFact(core.Fact{Predicate: "p", Args: []any{types.MangleAtom("/a")}})
	long := tc.CountFact(core.Fact{Predicate: "p", Args: []any{types.MangleAtom("/" + strings.Repeat("long-symbol-name-", 20))}})
	if long <= short {
		t.Errorf("long atom = %d, short atom = %d; want long > short", long, short)
	}
	if short <= 3 {
		t.Errorf("short atom = %d; want more than the old flat default", short)
	}
	if got := tc.CountFact(core.Fact{Predicate: "p", Args: []any{int32(1), uint64(2), float32(3)}}); got <= 0 {
		t.Errorf("numeric widening count = %d, want > 0", got)
	}
}

func TestWorkingStore_SaveRejectsEmptyID(t *testing.T) {
	w, err := NewWorkingSet(nil, t.TempDir(), "ids")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	if err := w.Save(t.Context(), WorkingRecord{Entity: "a.go", Body: "x"}); err == nil {
		t.Error("Save with empty ID: expected an error, got nil")
	}
}

func TestWorkingStore_CandidatesRejectsBadLimit(t *testing.T) {
	w, err := NewWorkingSet(nil, t.TempDir(), "limits")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	if _, err := w.store.Candidates(t.Context(), []string{"a.go"}, 0); err == nil {
		t.Error("Candidates limit 0: expected an error, got nil")
	}
	if _, err := w.store.Candidates(t.Context(), []string{"a.go"}, -1); err == nil {
		t.Error("Candidates limit -1: expected an error, got nil (SQLite reads that as unlimited)")
	}
	if got, err := w.store.Candidates(t.Context(), nil, 10); err != nil || got != nil {
		t.Errorf("Candidates with no entities = %v, %v; want nil, nil", got, err)
	}
}

// Two stop reasons derived at once must resolve to the same decision every
// time: the harness reports the reason, and a flapping reason is a lying one.
func TestWorkingSetContinue_MultiStopIsDeterministic(t *testing.T) {
	w, err := NewWorkingSet(nil, t.TempDir(), "stops")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	p := WorkingProgress{Cycle: true, FailedRounds: 5, WriteIntent: true, Rounds: 30, SinceWrite: 30, SinceVerify: 30}
	first, err := w.Continue(t.Context(), p)
	if err != nil {
		t.Fatal(err)
	}
	if first.Stop == "" {
		t.Fatal("expected a stop, got none")
	}
	for i := 0; i < 5; i++ {
		got, err := w.Continue(t.Context(), p)
		if err != nil {
			t.Fatal(err)
		}
		if got != first {
			t.Fatalf("decision drifted: first=%+v now=%+v", first, got)
		}
	}
	switch first.Stop {
	case "repeated_cycle", "tool_failures", "read_only_stall":
	default:
		t.Errorf("stop = %q, want one of the derived reasons", first.Stop)
	}
}

// stubWorld feeds the working scope a fixed dependency neighbourhood so Select
// exercises the canonical C1/C4 path: user_intent + focus_resolution +
// dependency_link derive should_include_context, which resolves world facts
// into the annotation section.
type stubWorld struct{ facts map[string][]core.Fact }

func (s stubWorld) Query(q string) ([]core.Fact, error) {
	for prefix, facts := range s.facts {
		if strings.HasPrefix(q, prefix) {
			return facts, nil
		}
	}
	return nil, nil
}

func TestWorkingSetSelect_WorldFactsAnnotate(t *testing.T) {
	world := stubWorld{facts: map[string][]core.Fact{
		"dependency_link": {{Predicate: "dependency_link", Args: []any{"a.go", "b.go", "import"}}},
		"code_defines":    {{Predicate: "code_defines", Args: []any{"a.go", "Main", "/func", int64(1), int64(10)}}},
	}}
	w, err := NewWorkingSet(world, t.TempDir(), "world")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = w.Close() })
	sel, err := w.Select(t.Context(), "a.go", nil, nil, 8000)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(sel.Text, "code_defines") || !strings.Contains(sel.Text, "Main") {
		t.Errorf("annotation section missing the world fact; text:\n%s", sel.Text)
	}
}
