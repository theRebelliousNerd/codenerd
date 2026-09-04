package mangle

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"
)

func newClearProbeEngine(t *testing.T) *Engine {
	t.Helper()
	e, err := NewEngine(DefaultConfig(), nil)
	if err != nil {
		t.Fatalf("NewEngine: %v", err)
	}
	// A rule is required, not incidental: Engine.Query evaluates top-down, so a
	// query against a pure EDB predicate never reaches the fact store. Routing
	// through a derived predicate forces the evaluator to resolve its premise
	// via queryContext.Store — the exact reference Clear() used to leave stale.
	schema := `Decl clearprobe(Value) bound [/string].
Decl clearprobe_derived(Value) bound [/string].
clearprobe_derived(V) :- clearprobe(V).`
	if err := e.LoadSchemaString(schema); err != nil {
		t.Fatalf("LoadSchemaString: %v", err)
	}
	return e
}

// queryDerived asks through the derived predicate so the fact store is consulted.
func queryDerived(t *testing.T, e *Engine) []string {
	t.Helper()
	res, err := e.Query(context.Background(), "clearprobe_derived(X)")
	if err != nil {
		t.Fatalf("Query: %v", err)
	}
	return bindingValues(res)
}

// bindingValues flattens a QueryResult into the string forms of its bound values.
func bindingValues(res *QueryResult) []string {
	if res == nil {
		return nil
	}
	out := make([]string, 0, len(res.Bindings))
	for _, row := range res.Bindings {
		for _, v := range row {
			out = append(out, strings.Trim(fmt.Sprintf("%v", v), `"`))
		}
	}
	return out
}

// Clear() swaps the fact store, but the query evaluator holds its own copy of
// the store reference on QueryContext (Store is a value field). Before the fix,
// Query kept answering from the discarded pre-Clear store while
// GetFacts/AddFacts/GetStats saw the new one — two read paths disagreeing
// permanently, for the lifetime of the engine.
//
// The pre-existing TestEngineClear asserts only GetFacts, which is precisely
// why this went unnoticed. This test drives the Query path.
func TestClear_QueryPathSeesClearedStore(t *testing.T) {
	e := newClearProbeEngine(t)

	if err := e.AddFact("clearprobe", "old"); err != nil {
		t.Fatalf("AddFact(old): %v", err)
	}

	if got := queryDerived(t, e); len(got) != 1 {
		t.Fatalf("expected 1 binding before Clear, got %d (%v)", len(got), got)
	}

	e.Clear()

	if got := queryDerived(t, e); len(got) != 0 {
		t.Fatalf("Query returned %d stale binding(s) after Clear: %v", len(got), got)
	}

	if err := e.AddFact("clearprobe", "new"); err != nil {
		t.Fatalf("AddFact(new): %v", err)
	}

	got := queryDerived(t, e)

	// Seeing "old" is the original bug's signature: the evaluator still reading
	// the store that Clear() discarded.
	for _, v := range got {
		if v == "old" {
			t.Fatalf("Query returned the pre-Clear fact %q — evaluator is bound to the discarded store (got %v)", v, got)
		}
	}
	if len(got) != 1 || got[0] != "new" {
		t.Errorf("post-Clear bindings = %v, want exactly [new]", got)
	}
}

// Clear() keeps schemas loaded, so the query path must remain usable. Nilling
// queryContext (the way Reset does) would make Query fail with errNoSchemas.
func TestClear_KeepsSchemasQueryable(t *testing.T) {
	e := newClearProbeEngine(t)

	e.Clear()

	if _, err := e.Query(context.Background(), "clearprobe_derived(X)"); err != nil {
		t.Fatalf("Query after Clear must still work (schemas are retained): %v", err)
	}
	if err := e.AddFact("clearprobe", "x"); err != nil {
		t.Fatalf("AddFact after Clear: %v", err)
	}
}

// Repeated Clear cycles must not accumulate or resurrect facts.
func TestClear_RepeatedCyclesStayConsistent(t *testing.T) {
	e := newClearProbeEngine(t)

	for i := range 3 {
		val := fmt.Sprintf("gen%d", i)
		if err := e.AddFact("clearprobe", val); err != nil {
			t.Fatalf("AddFact(%s): %v", val, err)
		}

		got := queryDerived(t, e)
		if len(got) != 1 || got[0] != val {
			t.Fatalf("cycle %d: bindings = %v, want exactly [%s]", i, got, val)
		}

		e.Clear()
	}
}

// AddFactsContext accepted a context and ignored it entirely, so a caller's
// deadline or cancellation was silently meaningless.
func TestAddFactsContext_HonorsCancellation(t *testing.T) {
	e := newClearProbeEngine(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	facts := make([]Fact, 1000)
	for i := range facts {
		facts[i] = Fact{Predicate: "clearprobe", Args: []any{"f"}}
	}

	err := e.AddFactsContext(ctx, facts)
	if err == nil {
		t.Fatal("AddFactsContext ignored an already-cancelled context")
	}
	if err != context.Canceled {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func TestAddFactsContext_HonorsDeadline(t *testing.T) {
	e := newClearProbeEngine(t)

	// Wait for the deadline to actually fire: a fixed sleep is not enough on
	// Windows, where the timer that expires the context has ~15ms resolution.
	ctx, cancel := context.WithTimeout(context.Background(), time.Nanosecond)
	defer cancel()
	<-ctx.Done()

	if err := e.AddFactsContext(ctx, []Fact{{Predicate: "clearprobe", Args: []any{"x"}}}); err == nil {
		t.Fatal("AddFactsContext ignored an expired deadline")
	}
}

// An empty batch is a no-op and must not fail even on a dead context, so a
// caller draining an empty queue during shutdown sees no spurious error.
func TestAddFactsContext_EmptyBatchIsNoop(t *testing.T) {
	e := newClearProbeEngine(t)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	if err := e.AddFactsContext(ctx, nil); err != nil {
		t.Errorf("empty batch should be a no-op, got %v", err)
	}
}

// AddFacts must keep working now that it delegates to AddFactsContext.
func TestAddFacts_StillInsertsViaDelegation(t *testing.T) {
	e := newClearProbeEngine(t)

	if err := e.AddFacts([]Fact{
		{Predicate: "clearprobe", Args: []any{"a"}},
		{Predicate: "clearprobe", Args: []any{"b"}},
	}); err != nil {
		t.Fatalf("AddFacts: %v", err)
	}

	if got := queryDerived(t, e); len(got) != 2 {
		t.Errorf("bindings = %v, want 2", got)
	}
}
