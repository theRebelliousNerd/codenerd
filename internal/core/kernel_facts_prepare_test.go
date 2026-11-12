package core

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/require"
)

// prepareTestFacts is a load big enough for the parallel prepare: duplicate
// facts, a value the Decl coerces (a float in a /number slot), one the Decl
// refuses (a fractional float there), and a malformed predicate name.
func prepareTestFacts() []Fact {
	var facts []Fact
	for i := 0; i < 3*parallelPrepareMinFacts; i++ {
		facts = append(facts, Fact{Predicate: "pp_edge", Args: []any{MangleAtom(fmt.Sprintf("/n%d", i%5000)), MangleAtom(fmt.Sprintf("/n%d", (i+1)%5000))}})
		if i%7 == 0 {
			facts = append(facts, Fact{Predicate: "pp_weight", Args: []any{fmt.Sprintf("w%d", i), float64(i)}})
		}
		if i%997 == 0 {
			facts = append(facts, Fact{Predicate: "pp_weight", Args: []any{fmt.Sprintf("frac%d", i), 0.5}})
			facts = append(facts, Fact{Predicate: "Bad-Name", Args: []any{"x"}})
		}
	}
	return facts
}

const preparePolicy = `
Decl pp_edge(A, B) bound [/name, /name].
Decl pp_weight(K, N) bound [/string, /number].
Decl pp_reach(A, B) bound [/name, /name].
pp_reach(A, B) :- pp_edge(A, B).
`

func prepareKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := setupMockKernel(t)
	k.AppendPolicy(preparePolicy)
	require.NoError(t, k.Evaluate())
	return k
}

// A load prepared on every core leaves exactly the EDB that inserting the
// same facts one at a time leaves: the same facts, in the same order, the
// same refusals, and the same derivations.
func TestLoadFacts_ParallelPrepareMatchesOneAtATime(t *testing.T) {
	facts := prepareTestFacts()
	require.GreaterOrEqual(t, len(facts), parallelPrepareMinFacts)

	parallel := prepareKernel(t)
	require.NoError(t, parallel.LoadFacts(facts))

	serial := prepareKernel(t)
	serial.mu.Lock()
	for _, f := range facts {
		serial.addFactIfNewLocked(sanitizeFactForNumericPredicates(f))
	}
	serial.markDirtyLocked()
	serial.mu.Unlock()

	edb := func(k *RealKernel) []string {
		k.mu.RLock()
		defer k.mu.RUnlock()
		var out []string
		for f, key := range k.facts.each() {
			out = append(out, f.Predicate+"|"+key)
		}
		return out
	}
	got, want := edb(parallel), edb(serial)
	require.Equal(t, len(want), len(got))
	require.Equal(t, want, got, "the parallel load holds different facts or a different order")

	for _, pred := range []string{"pp_reach", "pp_weight"} {
		a, err := parallel.Query(pred)
		require.NoError(t, err)
		b, err := serial.Query(pred)
		require.NoError(t, err)
		require.Equal(t, len(b), len(a), pred)
	}
	w, err := parallel.Query("pp_weight")
	require.NoError(t, err)
	for _, f := range w {
		require.NotContains(t, f.Args[0], "frac", "a fractional /number was admitted")
	}
	bad, err := parallel.Query("Bad-Name")
	require.True(t, err != nil || len(bad) == 0, "a malformed predicate was admitted")
}
