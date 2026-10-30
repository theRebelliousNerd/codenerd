package mangle

import (
	"os"
	"strings"
	"testing"
	"time"

	"codeberg.org/TauCeti/mangle-go/analysis"
	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/factstore"
)

// Reproduction for upstream (codeberg.org/TauCeti/mangle-go v0.7.0): a merge
// lattice whose recursion reaches its own key through a cost-increasing cycle
// never finishes, and the created-fact limit does not stop it.
//
// Minimal program, one self-loop edge:
//
//	Decl cheapest(A, B, Cost)
//	  descr [fundep([A, B], [Cost]), merge([Cost], "lower")] bound [/name, /name, /number].
//	Decl lower(X, Y, Best) descr [mode('+', '+', '-'), deferred()] bound [/number, /number, /number].
//	lower(X, Y, X) :- X < Y.
//	lower(X, Y, Y) :- Y <= X.
//	edge(/a, /a, 1).
//	cheapest(A, B, C) :- edge(A, B, C).
//	cheapest(A, C, T) :- cheapest(A, B, X), edge(B, C, Y), T = fn:plus(X, Y).
//
// Expected: cheapest(/a, /a, 1) and an end. Observed: EvalStratifiedProgramWithStats
// does not return (CPU bound, memory flat, WithCreatedFactLimit(1000) never
// trips). The same lattice over the three-cycle a->b (4), b->c (1), c->a (1)
// finishes at once, and so does the unweighted hop-count version over a cycle;
// a three-cycle plus a redundant direct edge a->c (9) also hangs. Hypothesis,
// not yet confirmed in the engine source: mergeDelta mutates the incoming fact
// to the stored winner and leaves it in the delta store, so a rule that derives
// a differently valued candidate from it (cost 1 + a self-loop of 1 = 2, merged
// back to 1) re-derives that candidate every round; the cycle that finishes is
// the one whose re-derived candidates equal what is already stored.
//
// Not a codeNERD defect: no kernel rule has a recursive weighted lattice. It is
// recorded here so the atoms can say "bound every recursive lattice" (atoms/
// mangle/advanced.yaml) with the program that justifies it.
//
// Skipped unless MANGLE_LATTICE_HANG_REPRO=1, and bounded when it runs: the
// evaluation is abandoned after the timeout (its goroutine keeps spinning until
// the test binary exits).
const latticeReproPreamble = `
Decl edge(A, B, Cost) bound [/name, /name, /number].
Decl cheapest(A, B, Cost)
  descr [fundep([A, B], [Cost]), merge([Cost], "lower")]
  bound [/name, /name, /number].
Decl lower(X, Y, Best) descr [mode('+', '+', '-'), deferred()] bound [/number, /number, /number].
lower(X, Y, X) :- X < Y.
lower(X, Y, Y) :- Y <= X.
cheapest(A, B, C) :- edge(A, B, C).
cheapest(A, C, T) :- cheapest(A, B, X), edge(B, C, Y), T = fn:plus(X, Y).
`

// evalLatticeWithin runs the program and reports its cheapest facts, or that it
// did not finish within the timeout.
func evalLatticeWithin(edges string, timeout time.Duration) (facts string, finished bool, err error) {
	type result struct {
		facts string
		err   error
	}
	done := make(chan result, 1)
	go func() {
		unit, perr := ParseUnit(strings.NewReader(latticeReproPreamble + edges))
		if perr != nil {
			done <- result{err: perr}
			return
		}
		info, aerr := analysis.AnalyzeOneUnit(unit, nil)
		if aerr != nil {
			done <- result{err: aerr}
			return
		}
		strata, predToStratum, serr := Stratify(info)
		if serr != nil {
			done <- result{err: serr}
			return
		}
		store := factstore.NewSimpleInMemoryStore()
		if _, eerr := engine.EvalStratifiedProgramWithStats(info, strata, predToStratum, store, engine.WithCreatedFactLimit(1000)); eerr != nil {
			done <- result{err: eerr}
			return
		}
		var out []string
		for _, pred := range store.ListPredicates() {
			if pred.Symbol != "cheapest" {
				continue
			}
			_ = store.GetFacts(ast.NewQuery(pred), func(a ast.Atom) error {
				out = append(out, a.String())
				return nil
			})
		}
		done <- result{facts: strings.Join(out, " ")}
	}()
	select {
	case r := <-done:
		return r.facts, true, r.err
	case <-time.After(timeout):
		return "", false, nil
	}
}

func TestLatticeReproduction_CostIncreasingCycleDoesNotTerminate(t *testing.T) {
	if os.Getenv("MANGLE_LATTICE_HANG_REPRO") != "1" {
		t.Skip("set MANGLE_LATTICE_HANG_REPRO=1 to run the upstream reproduction (it abandons a spinning goroutine)")
	}
	t.Run("control: a three-cycle settles", func(t *testing.T) {
		facts, finished, err := evalLatticeWithin("edge(/a, /b, 4).\nedge(/b, /c, 1).\nedge(/c, /a, 1).\n", 10*time.Second)
		if err != nil || !finished {
			t.Fatalf("the control must finish: finished=%v err=%v", finished, err)
		}
		if !strings.Contains(facts, "cheapest(/a,/c,5)") {
			t.Errorf("control facts = %s", facts)
		}
	})
	t.Run("a self-loop never settles", func(t *testing.T) {
		facts, finished, err := evalLatticeWithin("edge(/a, /a, 1).\n", 10*time.Second)
		if err != nil {
			t.Fatalf("eval error: %v", err)
		}
		if !finished {
			t.Errorf("upstream mangle-go: a one-edge self-loop over a merge lattice did not finish within 10s (see this file's header)")
			return
		}
		if want := "cheapest(/a,/a,1)"; !strings.Contains(facts, want) {
			t.Errorf("facts = %s, want %s", facts, want)
		}
	})
}
