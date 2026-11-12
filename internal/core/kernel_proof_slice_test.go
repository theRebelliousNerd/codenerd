package core

import (
	"bytes"
	"errors"
	"fmt"
	"sort"
	"testing"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codeberg.org/TauCeti/mangle-go/engine"
	"codeberg.org/TauCeti/mangle-go/provenance"
	"github.com/stretchr/testify/require"
)

// proofSlicePolicy exercises every dependency a slice must follow: recursion,
// negation, an aggregation and a let transform, beside a large predicate the
// goals never read.
const proofSlicePolicy = `
Decl ps_edge(A, B) bound [/name, /name].
Decl ps_node(A) bound [/name].
Decl ps_noise(A, B) bound [/name, /number].
Decl ps_reach(A, B) bound [/name, /name].
Decl ps_isolated(A) bound [/name].
Decl ps_reaches_any(A) bound [/name].
Decl ps_degree(A, N) bound [/name, /number].
Decl ps_degree_plus(A, N) bound [/name, /number].
Decl ps_noise_seen(A) bound [/name].
ps_reach(A, B) :- ps_edge(A, B).
ps_reach(A, C) :- ps_edge(A, B), ps_reach(B, C).
ps_reaches_any(A) :- ps_reach(A, _).
ps_isolated(A) :- ps_node(A), !ps_reaches_any(A).
ps_degree(A, N) :- ps_edge(A, _) |> do fn:group_by(A), let N = fn:count().
ps_degree_plus(A, M) :- ps_degree(A, N) |> let M = fn:plus(N, 1).
ps_noise_seen(A) :- ps_noise(A, _).
`

func proofSliceKernel(t *testing.T) *RealKernel {
	t.Helper()
	k := setupMockKernel(t)
	k.AppendPolicy(proofSlicePolicy)
	var facts []Fact
	for _, e := range [][2]string{{"/a", "/b"}, {"/b", "/c"}, {"/c", "/d"}, {"/a", "/c"}} {
		facts = append(facts, Fact{Predicate: "ps_edge", Args: []any{MangleAtom(e[0]), MangleAtom(e[1])}})
	}
	for _, n := range []string{"/a", "/b", "/c", "/d", "/z"} {
		facts = append(facts, Fact{Predicate: "ps_node", Args: []any{MangleAtom(n)}})
	}
	for i := 0; i < 2000; i++ {
		facts = append(facts, Fact{Predicate: "ps_noise", Args: []any{MangleAtom(fmt.Sprintf("/n%d", i)), int64(i)}})
	}
	require.NoError(t, k.LoadFacts(facts))
	require.NoError(t, k.Evaluate())
	k.EnableProvenance()
	return k
}

// fullRecordedProofs is the reference: the whole program evaluated with a
// recorder over a fresh store of every EDB row, the way evaluate() recorded
// before proofs were sliced.
func fullRecordedProofs(t *testing.T, k *RealKernel, goals []string) map[string]string {
	t.Helper()
	k.mu.Lock()
	defer k.mu.Unlock()
	all := make(map[string]struct{})
	for f := range k.facts.all() {
		all[f.Predicate] = struct{}{}
	}
	atoms, err := k.edbAtomsOfLocked(all)
	require.NoError(t, err)
	store := newEvaluationFactStore(len(atoms))
	for _, a := range atoms {
		store.Add(a)
	}
	rec := provenance.NewMemoryRecorder()
	opts := append([]engine.EvalOption{engine.WithCreatedFactLimit(k.effectiveDerivedFactLimitLocked()), engine.WithDerivationRecorder(rec)},
		k.externalPredicateOptionsLocked()...)
	_, err = engine.EvalStratifiedProgramWithStats(k.programInfo, k.strata, k.predToStratum, store, opts...)
	require.NoError(t, err)
	out := make(map[string]string, len(goals))
	for _, g := range goals {
		unit, err := parseUnit(bytes.NewReader([]byte(g + ".")))
		require.NoError(t, err)
		proofs, err := provenance.BuildFromRecording(rec, store, unit.Clauses[0].Head, provenance.Options{})
		if errors.Is(err, provenance.ErrNoProof) {
			out[g] = "no proof"
			continue
		}
		require.NoError(t, err)
		out[g] = printProofs(t, proofs)
	}
	return out
}

func printProofs(t *testing.T, proofs []*provenance.ProofNode) string {
	t.Helper()
	var b bytes.Buffer
	require.NoError(t, provenance.Print(&b, proofs))
	return b.String()
}

// productionGoals picks ground goals the production corpus derives at boot,
// a few rows of each of several derived predicates, deterministically.
func productionGoals(t *testing.T, k *RealKernel, perPred int) []string {
	t.Helper()
	k.mu.RLock()
	var heads []string
	for sym := range k.programInfo.IdbPredicates {
		heads = append(heads, sym.Symbol)
	}
	k.mu.RUnlock()
	sort.Strings(heads)
	var goals []string
	for _, pred := range heads {
		rows, err := k.Query(pred)
		require.NoError(t, err)
		var picked []string
		for _, f := range rows {
			atom, err := f.ToAtom()
			if err != nil {
				continue
			}
			picked = append(picked, atom.String())
		}
		sort.Strings(picked)
		if len(picked) > perPred {
			picked = picked[:perPred]
		}
		goals = append(goals, picked...)
	}
	return goals
}

// The slice's proofs are the proofs a recorded full pass gives, for the test
// program's recursion, negation, aggregation and let goals and for every
// derived predicate of the production corpus.
func TestProofSlice_MatchesTheFullRecordedPass(t *testing.T) {
	k := proofSliceKernel(t)
	goals := []string{
		"ps_reach(/a, /d)", "ps_reach(/b, /d)", "ps_isolated(/z)", "ps_isolated(/d)",
		"ps_degree(/a, 2)", "ps_degree_plus(/a, 3)", "ps_reach(/d, /a)",
	}
	goals = append(goals, productionGoals(t, k, 2)...)
	require.Greater(t, len(goals), 50, "the production corpus derives too little at boot to compare")

	want := fullRecordedProofs(t, k, goals)
	derived := 0
	for _, g := range goals {
		proofs, err := k.Explain(g, ExplainOptions{})
		got := "no proof"
		if err == nil {
			got = printProofs(t, proofs)
			derived++
		} else if !errors.Is(err, provenance.ErrNoProof) {
			t.Fatalf("Explain(%s): %v", g, err)
		}
		require.Equal(t, want[g], got, "proofs of %s differ from the full recorded pass", g)
	}
	require.Greater(t, derived, 40)
}

// A slice records only what the goal reads: the noise rows and their rule
// never enter the recording of ps_reach.
func TestProofSlice_RecordsOnlyTheGoalsUpstream(t *testing.T) {
	k := proofSliceKernel(t)
	_, err := k.Explain("ps_reach(/a, /d)", ExplainOptions{})
	require.NoError(t, err)
	k.mu.RLock()
	defer k.mu.RUnlock()
	s := k.proofs.byPred["ps_reach"]
	require.NotNil(t, s)
	require.Equal(t, 2, s.rules, "ps_reach's slice is its own two rules")
	require.Equal(t, 4, s.edbRows, "ps_reach's slice holds the four edges, no noise")
	for _, ev := range s.recorder.Events() {
		require.Equal(t, "ps_reach", ev.Output.Predicate.Symbol)
	}
	var noise int
	require.NoError(t, s.store.GetFacts(ast.NewQuery(ast.PredicateSym{Symbol: "ps_noise", Arity: 2}), func(ast.Atom) error {
		noise++
		return nil
	}))
	require.Zero(t, noise)
}

// A recording is of one evaluation. A write, a retraction or a program change
// drops it, and the next explanation describes the new state; asking twice
// about the same state reuses the recording.
func TestProofSlice_InvalidatedByEveryChange(t *testing.T) {
	k := proofSliceKernel(t)
	_, err := k.Explain("ps_reach(/a, /d)", ExplainOptions{})
	require.NoError(t, err)
	k.mu.RLock()
	first := k.proofs.byPred["ps_reach"]
	k.mu.RUnlock()
	_, err = k.Explain("ps_reach(/b, /d)", ExplainOptions{})
	require.NoError(t, err)
	k.mu.RLock()
	require.Same(t, first, k.proofs.byPred["ps_reach"], "same evaluation, same recording")
	k.mu.RUnlock()

	// A new edge: the new reachability is explained.
	_, err = k.Explain("ps_reach(/d, /e)", ExplainOptions{})
	require.ErrorIs(t, err, provenance.ErrNoProof)
	require.NoError(t, k.Assert(Fact{Predicate: "ps_edge", Args: []any{MangleAtom("/d"), MangleAtom("/e")}}))
	proofs, err := k.Explain("ps_reach(/d, /e)", ExplainOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, proofs)
	k.mu.RLock()
	require.NotSame(t, first, k.proofs.byPred["ps_reach"], "a write drops the recording")
	k.mu.RUnlock()

	// Retracting it: the proof is gone, not served from the old recording.
	require.NoError(t, k.RetractExactFact(Fact{Predicate: "ps_edge", Args: []any{MangleAtom("/d"), MangleAtom("/e")}}))
	_, err = k.Explain("ps_reach(/d, /e)", ExplainOptions{})
	require.ErrorIs(t, err, provenance.ErrNoProof)

	// A program change: a new rule's derivation is explained.
	k.AppendPolicy(`ps_reach(A, A) :- ps_node(A).`)
	proofs, err = k.Explain("ps_reach(/z, /z)", ExplainOptions{})
	require.NoError(t, err)
	require.NotEmpty(t, proofs)

	k.DisableProvenance()
	_, err = k.Explain("ps_reach(/a, /d)", ExplainOptions{})
	require.ErrorIs(t, err, ErrProvenanceDisabled)
}
