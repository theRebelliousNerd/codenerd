package core

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The reference relation best is a numeric-maximum lattice. Its merge
// comparator, higher, is called by the Decl, not by a rule body, so the
// inclusion closure never reached it: the projected program held
// best :- candidate without higher's rules, the merge found every pair
// incomparable and kept both rows, and the losing claim passed (audit
// mangle-inclusion-projection-closure).
func TestInclusion_LatticeReferenceKeepsItsComparator(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl incl_candidate(K, V) bound [/name, /number].
Decl incl_best(K, V)
    descr [fundep([K], [V]), merge([V], "incl_higher")]
    bound [/name, /number].
Decl incl_higher(A, B, Best)
    descr [mode('+', '+', '-'), deferred()]
    bound [/number, /number, /number].
Decl incl_claimed(K, V) bound [/name, /number] inclusion [incl_best(K, V)].
incl_best(K, V) :- incl_candidate(K, V).
incl_higher(A, B, A) :- A >= B.
incl_higher(A, B, B) :- B > A.`)

	for _, order := range [][]int64{{1, 2}, {2, 1}} {
		batch := []Fact{
			{Predicate: "incl_candidate", Args: []any{MangleAtom("/k"), order[0]}},
			{Predicate: "incl_candidate", Args: []any{MangleAtom("/k"), order[1]}},
		}
		loser := append(append([]Fact{}, batch...), Fact{Predicate: "incl_claimed", Args: []any{MangleAtom("/k"), int64(1)}})
		report, err := k.InclusionReport("probe", loser)
		require.NoError(t, err)
		require.Empty(t, report.Unresolved, "the comparator reads nothing outside the batch")
		require.Len(t, report.Violations, 1, "claiming the lattice's losing value 1 (candidates in order %v) must violate inclusion", order)

		winner := append(append([]Fact{}, batch...), Fact{Predicate: "incl_claimed", Args: []any{MangleAtom("/k"), int64(2)}})
		require.NoError(t, k.CheckBatchInclusion("probe", winner), "the winning value 2 is a member")
	}
}

// A comparator that reads a relation the batch does not hold leaves the
// lattice's domain open: unresolved, never borrowed from live state.
func TestInclusion_ComparatorOverAnOpenDomainIsUnresolved(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl inclo_candidate(K, O) bound [/name, /name].
Decl inclo_rank(O, R) bound [/name, /number].
Decl inclo_best(K, O)
    descr [fundep([K], [O]), merge([O], "inclo_better")]
    bound [/name, /name].
Decl inclo_better(A, B, Best)
    descr [mode('+', '+', '-'), deferred()]
    bound [/name, /name, /name].
Decl inclo_claimed(K, O) bound [/name, /name] inclusion [inclo_best(K, O)].
inclo_best(K, O) :- inclo_candidate(K, O).
inclo_better(A, B, A) :- inclo_rank(A, RA), inclo_rank(B, RB), RA <= RB.
inclo_better(A, B, B) :- inclo_rank(A, RA), inclo_rank(B, RB), RB < RA.`)
	batch := []Fact{
		{Predicate: "inclo_candidate", Args: []any{MangleAtom("/k"), MangleAtom("/a")}},
		{Predicate: "inclo_claimed", Args: []any{MangleAtom("/k"), MangleAtom("/a")}},
	}
	report, err := k.InclusionReport("probe", batch)
	require.NoError(t, err)
	require.Empty(t, report.Violations)
	require.Len(t, report.Unresolved, 1)
	require.Contains(t, report.Unresolved[0].Reason, "inclo_rank")
}

// A program fact of a derived reference relation survives the projection,
// and its time entry must travel with it: the engine reads
// InitialFactTimes[i] for every initial fact it admits, and the projection
// dropped that slice, so any retained program fact was an index panic.
func TestInclusion_RetainedProgramFactKeepsItsTimeMetadata(t *testing.T) {
	t.Parallel()
	k := admissionKernel(t, `Decl inclf_source(X) bound [/name].
Decl inclf_reference(X) bound [/name].
Decl inclf_claim(X) bound [/name] inclusion [inclf_reference(X)].
inclf_reference(X) :- inclf_source(X).
inclf_reference(/builtin).`)
	batch := []Fact{
		{Predicate: "inclf_source", Args: []any{MangleAtom("/ordinary")}},
		{Predicate: "inclf_claim", Args: []any{MangleAtom("/builtin")}},
		{Predicate: "inclf_claim", Args: []any{MangleAtom("/ordinary")}},
	}
	var report InclusionReport
	var err error
	require.NotPanics(t, func() { report, err = k.InclusionReport("probe", batch) })
	require.NoError(t, err)
	require.Empty(t, report.Unresolved)
	require.Empty(t, report.Violations, "the program fact inclusion reference(/builtin) is part of the domain")
	require.Equal(t, 2, report.Checked)

	stray := append(append([]Fact{}, batch...), Fact{Predicate: "inclf_claim", Args: []any{MangleAtom("/stray")}})
	require.ErrorIs(t, k.CheckBatchInclusion("probe", stray), ErrInclusionViolation)
}
