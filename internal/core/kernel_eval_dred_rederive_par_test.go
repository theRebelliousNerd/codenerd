package core

import (
	"fmt"
	"sort"
	"testing"

	"codeberg.org/TauCeti/mangle-go/factstore"
)

// rederiveWidePolicy is the shape of the coder context rules: a hub's
// dependents are impacted while the hub is a target OR was changed, and what
// is impacted is included.
const rederiveWidePolicy = `
Decl rd_target(T) bound [/string].
Decl rd_changed(T) bound [/string].
Decl rd_link(X, Y) bound [/string, /string].
Decl rd_impacted(X) bound [/string].
Decl rd_include(X) bound [/string].
rd_impacted(X) :- rd_target(Y), rd_link(X, Y).
rd_impacted(X) :- rd_changed(Y), rd_link(X, Y).
rd_include(X) :- rd_impacted(X).
`

func rederiveWideKernel(tb testing.TB, perHub int) *RealKernel {
	tb.Helper()
	k := newRealKernelBase()
	k.AppendPolicy(rederiveWidePolicy)
	if err := k.Evaluate(); err != nil {
		tb.Fatal(err)
	}
	facts := make([]Fact, 0, 2*perHub+4)
	for i := range perHub {
		// kept: also supported by rd_changed; lost: supported only by the target
		facts = append(facts,
			Fact{Predicate: "rd_link", Args: []any{fmt.Sprintf("kept%05d.go", i), "hub_kept.go"}},
			Fact{Predicate: "rd_link", Args: []any{fmt.Sprintf("lost%05d.go", i), "hub_lost.go"}})
	}
	facts = append(facts,
		Fact{Predicate: "rd_target", Args: []any{"hub_kept.go"}},
		Fact{Predicate: "rd_target", Args: []any{"hub_lost.go"}},
		Fact{Predicate: "rd_changed", Args: []any{"hub_kept.go"}})
	if err := k.LoadFacts(facts); err != nil {
		tb.Fatal(err)
	}
	if _, err := k.Query("rd_include"); err != nil {
		tb.Fatal(err)
	}
	return k
}

func rederiveRows(tb testing.TB, k *RealKernel, pred string) []string {
	tb.Helper()
	rows, err := k.Query(pred)
	if err != nil {
		tb.Fatal(err)
	}
	out := make([]string, 0, len(rows))
	for _, r := range rows {
		out = append(out, r.String())
	}
	sort.Strings(out)
	return out
}

// An over-delete large enough to test its facts for support on every core
// leaves exactly the facts a fresh evaluation of the new state derives: what
// has another support comes back, what does not stays gone.
func TestDRedRederive_AWideOverDeleteRestoresExactlyWhatIsStillSupported(t *testing.T) {
	const perHub = 3000 // well past dredParallelMinRows
	k := rederiveWideKernel(t, perHub)
	for _, hub := range []string{"hub_kept.go", "hub_lost.go"} {
		if err := k.RetractExactFact(Fact{Predicate: "rd_target", Args: []any{hub}}); err != nil {
			t.Fatal(err)
		}
	}
	if err := k.EvaluatePending(); err != nil {
		t.Fatal(err)
	}
	ev := k.LastEvaluation()
	if ev.Mode != "dred" || ev.OverDeleted < 2*perHub {
		t.Fatalf("evaluated as %q over-deleting %d (fallback %q); want dred over-deleting at least %d", ev.Mode, ev.OverDeleted, ev.ConeFallback, 2*perHub)
	}
	if ev.Rederived < perHub {
		t.Fatalf("re-derived %d facts; the %d dependents of the hub that is still changed must come back", ev.Rederived, perHub)
	}

	// The oracle: the new state evaluated from nothing.
	fresh := newPolicyKernel(t, newRealKernelBase(), rederiveWidePolicy)
	var facts []Fact
	for _, pred := range []string{"rd_link", "rd_target", "rd_changed"} {
		rows, err := k.Query(pred)
		if err != nil {
			t.Fatal(err)
		}
		facts = append(facts, rows...)
	}
	if err := fresh.LoadFacts(facts); err != nil {
		t.Fatal(err)
	}
	for _, pred := range []string{"rd_impacted", "rd_include"} {
		got, want := rederiveRows(t, k, pred), rederiveRows(t, fresh, pred)
		if len(want) != perHub {
			t.Fatalf("%s: the oracle derives %d rows, want the %d kept dependents", pred, len(want), perHub)
		}
		if len(got) != len(want) {
			t.Fatalf("%s: %d rows after the incremental pass, a fresh evaluation derives %d", pred, len(got), len(want))
		}
		for i := range want {
			if got[i] != want[i] {
				t.Fatalf("%s: row %d is %s, a fresh evaluation has %s", pred, i, got[i], want[i])
			}
		}
	}
}

func BenchmarkDRedRederiveWideOverDelete(b *testing.B) {
	const perHub = 1700 // about the 3,300 facts a turn boundary over-deleted live
	k := rederiveWideKernel(b, perHub)
	target := Fact{Predicate: "rd_target", Args: []any{"hub_kept.go"}}
	b.ReportAllocs()
	for b.Loop() {
		if err := k.RetractExactFact(target); err != nil {
			b.Fatal(err)
		}
		if err := k.EvaluatePending(); err != nil {
			b.Fatal(err)
		}
		b.StopTimer()
		if err := k.Assert(target); err != nil {
			b.Fatal(err)
		}
		if err := k.EvaluatePending(); err != nil {
			b.Fatal(err)
		}
		b.StartTimer()
	}
}

// A kernel that boots small and grows by incremental passes ends up on an
// indexed store, not on the small one its first evaluation chose.
func TestEvaluate_AGrownKernelIsMovedToAnIndexedStore(t *testing.T) {
	k := rederiveWideKernel(t, indexedStoreMinFacts)
	if err := k.Assert(Fact{Predicate: "rd_changed", Args: []any{"one_more.go"}}); err != nil {
		t.Fatal(err)
	}
	if err := k.EvaluatePending(); err != nil {
		t.Fatal(err)
	}
	if mode := k.LastEvaluation().Mode; mode == "full" {
		t.Fatalf("the write evaluated as %q; the test needs an incremental pass to mean anything", mode)
	}
	k.mu.RLock()
	_, small := k.store.(factstore.SimpleInMemoryStore)
	held := k.store.EstimateFactCount()
	k.mu.RUnlock()
	if small {
		t.Fatalf("a kernel holding %d facts is still on the unindexed store", held)
	}
	// Both hubs are still targets: every dependent of each is included.
	if got, want := len(rederiveRows(t, k, "rd_include")), 2*indexedStoreMinFacts; got != want {
		t.Fatalf("rd_include holds %d rows after the move, want %d", got, want)
	}
}
