package prompt

import (
	"fmt"
	"math"
	"reflect"
	"sync"
	"testing"
)

// feed runs n turns. pick(i) returns the atom ids selected on turn i; a nil
// return skips the turn entirely.
func feed(r *CoUseRecorder, n int, pick func(i int) []string) {
	for i := 0; i < n; i++ {
		atoms := pick(i)
		if atoms == nil {
			continue
		}
		id := fmt.Sprintf("turn-%d", i)
		r.Observe(id, atoms)
		r.Settle(id, OutcomeSuccess)
	}
}

func pairFor(t *testing.T, rep CoUseReport, a, b string) PairStat {
	t.Helper()
	for _, p := range rep.Pairs {
		if (p.A == a && p.B == b) || (p.A == b && p.B == a) {
			return p
		}
	}
	t.Fatalf("pair (%s,%s) not reported; got %+v", a, b, rep.Pairs)
	return PairStat{}
}

// ---------------------------------------------------------------------------
// The statistics
// ---------------------------------------------------------------------------

func TestLiftAndJaccardMath(t *testing.T) {
	r := NewCoUseRecorder()
	// 100 turns. A in 50, B in 50, both in 40.
	// P(A)=P(B)=0.5, P(A,B)=0.4  ->  lift = 0.4/0.25 = 1.6
	// union = 50+50-40 = 60      ->  jaccard = 40/60 = 0.6667
	feed(r, 100, func(i int) []string {
		switch {
		case i < 40:
			return []string{"A", "B"}
		case i < 50:
			return []string{"A", "filler"}
		case i < 60:
			return []string{"B", "filler"}
		default:
			return []string{"filler"}
		}
	})

	rep := r.Report(DefaultCoUseParams(), nil)
	if rep.SuccessSelections != 100 {
		t.Fatalf("success turns = %d, want 100", rep.SuccessSelections)
	}

	p := pairFor(t, rep, "A", "B")
	if p.Joint != 40 {
		t.Fatalf("joint = %d, want 40", p.Joint)
	}
	if math.Abs(p.Lift-1.6) > 1e-9 {
		t.Fatalf("lift = %v, want 1.6", p.Lift)
	}
	if math.Abs(p.Jaccard-40.0/60.0) > 1e-9 {
		t.Fatalf("jaccard = %v, want %v", p.Jaccard, 40.0/60.0)
	}
}

func TestLiftScoresIndependenceAtOne(t *testing.T) {
	r := NewCoUseRecorder()
	// A in exactly half the turns, B in exactly half, independently: the joint
	// is a quarter. Lift must be 1, and the pair must therefore not survive the
	// MinLift filter. If independent atoms formed edges, every cluster in the
	// report would be an artifact of frequency rather than of association.
	feed(r, 200, func(i int) []string {
		var atoms []string
		if i%2 == 0 {
			atoms = append(atoms, "A")
		}
		if (i/2)%2 == 0 {
			atoms = append(atoms, "B")
		}
		if len(atoms) == 0 {
			return []string{"filler"}
		}
		return atoms
	})

	params := DefaultCoUseParams()
	params.MinLift = 0 // report everything so the value itself can be checked
	rep := r.Report(params, nil)

	p := pairFor(t, rep, "A", "B")
	if math.Abs(p.Lift-1.0) > 0.05 {
		t.Fatalf("lift for independent atoms = %v, want ~1", p.Lift)
	}

	// And with the real threshold, the pair is gone.
	strict := r.Report(DefaultCoUseParams(), nil)
	for _, q := range strict.Pairs {
		if (q.A == "A" && q.B == "B") || (q.A == "B" && q.B == "A") {
			t.Fatalf("independent pair survived the lift threshold: %+v", q)
		}
	}
}

func TestUbiquitousAtomScoresIndependenceAgainstEverything(t *testing.T) {
	r := NewCoUseRecorder()
	// U is in every turn. This is the trap raw co-occurrence counts fall into:
	// U co-occurs with everything more than any other pair does, so a
	// count-ranked report would put the least informative atom at the top of
	// every list.
	feed(r, 100, func(i int) []string {
		if i < 50 {
			return []string{"U", "A"}
		}
		return []string{"U", "B"}
	})

	params := DefaultCoUseParams()
	params.MinLift = 0
	params.UbiquityThreshold = 2 // disable the exclusion to isolate the lift math
	rep := r.Report(params, nil)

	p := pairFor(t, rep, "U", "A")
	if math.Abs(p.Lift-1.0) > 1e-9 {
		t.Fatalf("lift(U,A) = %v, want exactly 1 — a universal atom is independent of everything", p.Lift)
	}
	if p.Joint != 50 {
		t.Fatalf("joint(U,A) = %d, want 50 — the raw count is high, which is the point", p.Joint)
	}
}

func TestUbiquitousAtomsAreExcludedFromTheGraph(t *testing.T) {
	r := NewCoUseRecorder()
	feed(r, 100, func(i int) []string {
		if i < 50 {
			return []string{"skeleton", "A", "B"}
		}
		return []string{"skeleton", "C", "D"}
	})

	rep := r.Report(DefaultCoUseParams(), nil)

	if len(rep.Ubiquitous) != 1 || rep.Ubiquitous[0] != "skeleton" {
		t.Fatalf("ubiquitous = %v, want [skeleton]", rep.Ubiquitous)
	}
	for _, p := range rep.Pairs {
		if p.A == "skeleton" || p.B == "skeleton" {
			t.Fatalf("a ubiquitous atom formed an edge: %+v", p)
		}
	}

	// A/B and C/D are perfectly associated inside their halves and never meet.
	// They must stay two clusters.
	if len(rep.Clusters) != 2 {
		t.Fatalf("clusters = %d, want 2: %+v", len(rep.Clusters), rep.Clusters)
	}
	for _, cl := range rep.Clusters {
		if len(cl.Atoms) != 2 {
			t.Fatalf("cluster %v has %d atoms, want 2", cl.Atoms, len(cl.Atoms))
		}
	}
}

func TestMinSupportFiltersNoise(t *testing.T) {
	r := NewCoUseRecorder()
	// X and Y appear together twice out of a hundred turns and never apart.
	// Their lift is enormous — 50 — and the evidence is two turns. Reporting it
	// is how a taxonomy gets built out of coincidence.
	feed(r, 100, func(i int) []string {
		if i < 2 {
			return []string{"X", "Y"}
		}
		return []string{"filler"}
	})

	rep := r.Report(DefaultCoUseParams(), nil)
	for _, p := range rep.Pairs {
		if p.A == "X" || p.B == "X" {
			t.Fatalf("a 2-turn pair survived MinSupport=%d: %+v", DefaultMinSupport, p)
		}
	}
	if len(rep.Clusters) != 0 {
		t.Fatalf("clusters = %+v, want none", rep.Clusters)
	}
}

// ---------------------------------------------------------------------------
// Clustering and taxonomy alignment
// ---------------------------------------------------------------------------

func TestClustersSplitOnRealBoundaries(t *testing.T) {
	r := NewCoUseRecorder()
	feed(r, 120, func(i int) []string {
		switch i % 3 {
		case 0:
			return []string{"a1", "a2", "a3"}
		case 1:
			return []string{"b1", "b2"}
		default:
			return []string{"c1", "c2"}
		}
	})

	rep := r.Report(DefaultCoUseParams(), nil)
	if len(rep.Clusters) != 3 {
		t.Fatalf("clusters = %d, want 3: %+v", len(rep.Clusters), rep.Clusters)
	}
	// Largest first.
	if len(rep.Clusters[0].Atoms) != 3 {
		t.Fatalf("largest cluster = %v, want the 3-atom group", rep.Clusters[0].Atoms)
	}
	if rep.ClusteredAtoms != 7 {
		t.Fatalf("clustered atoms = %d, want 7", rep.ClusteredAtoms)
	}
}

func TestCategoryAlignmentIsHighWhenClustersRespectCategories(t *testing.T) {
	r := NewCoUseRecorder()
	feed(r, 100, func(i int) []string {
		if i%2 == 0 {
			return []string{"arch1", "arch2"}
		}
		return []string{"impl1", "impl2"}
	})

	lookup := func(id string) string {
		if len(id) >= 4 && id[:4] == "arch" {
			return "architecture"
		}
		return "implementation"
	}

	rep := r.Report(DefaultCoUseParams(), lookup)
	if math.Abs(rep.CategoryAlignment-1.0) > 1e-9 {
		t.Fatalf("alignment = %v, want 1 — every cluster is single-category", rep.CategoryAlignment)
	}
	if rep.CrossCategoryPairs != 0 {
		t.Fatalf("cross-category pairs = %d, want 0", rep.CrossCategoryPairs)
	}
}

func TestCategoryAlignmentIsLowWhenClustersCutAcrossCategories(t *testing.T) {
	r := NewCoUseRecorder()
	// Two atoms from different categories that are always used together. This
	// is the shape that answers Q2 in the negative: information clusters across
	// the taxonomy, not along it.
	feed(r, 100, func(i int) []string {
		if i%2 == 0 {
			return []string{"arch1", "impl1"}
		}
		return []string{"filler"}
	})

	lookup := func(id string) string {
		switch id {
		case "arch1":
			return "architecture"
		case "impl1":
			return "implementation"
		}
		return "other"
	}

	rep := r.Report(DefaultCoUseParams(), lookup)
	if len(rep.Clusters) != 1 {
		t.Fatalf("clusters = %d, want 1: %+v", len(rep.Clusters), rep.Clusters)
	}
	// One of two atoms carries the modal category, so purity is 0.5.
	if math.Abs(rep.Clusters[0].Purity-0.5) > 1e-9 {
		t.Fatalf("purity = %v, want 0.5", rep.Clusters[0].Purity)
	}
	if rep.CrossCategoryPairs != 1 {
		t.Fatalf("cross-category pairs = %d, want 1", rep.CrossCategoryPairs)
	}
}

func TestPurityCountsUncategorizedAtomsAgainstTheTaxonomy(t *testing.T) {
	r := NewCoUseRecorder()
	feed(r, 100, func(i int) []string {
		if i%2 == 0 {
			return []string{"known", "unknown"}
		}
		return []string{"filler"}
	})

	lookup := func(id string) string {
		if id == "known" {
			return "architecture"
		}
		return "" // deliberately uncategorized
	}

	rep := r.Report(DefaultCoUseParams(), lookup)
	if len(rep.Clusters) != 1 {
		t.Fatalf("clusters = %+v", rep.Clusters)
	}
	// An atom the taxonomy does not cover is evidence against the taxonomy.
	// Dividing it out of the denominator would score that gap as agreement.
	if math.Abs(rep.Clusters[0].Purity-0.5) > 1e-9 {
		t.Fatalf("purity = %v, want 0.5", rep.Clusters[0].Purity)
	}
}

func TestNilLookupLeavesAlignmentUnclaimed(t *testing.T) {
	r := NewCoUseRecorder()
	feed(r, 100, func(i int) []string { return []string{"a", "b"} })

	rep := r.Report(DefaultCoUseParams(), nil)
	if rep.CategoryAlignment != 0 {
		t.Fatalf("alignment = %v with no category source, want 0", rep.CategoryAlignment)
	}
}

func TestIsolatedAtomsCounted(t *testing.T) {
	r := NewCoUseRecorder()
	// "lonely" appears often but never with a consistent partner.
	feed(r, 100, func(i int) []string {
		return []string{"lonely", fmt.Sprintf("partner-%d", i)}
	})

	rep := r.Report(DefaultCoUseParams(), nil)
	// lonely is in 100 of 100 turns, which makes it ubiquitous rather than
	// isolated. Its partners each appear once, below MinSupport, so they are
	// too rare to have had a chance to cluster and must not be blamed for it.
	if rep.IsolatedAtoms != 0 {
		t.Fatalf("isolated = %d, want 0 (rare atoms are a sample-size problem, not a finding)", rep.IsolatedAtoms)
	}
	if len(rep.Ubiquitous) != 1 || rep.Ubiquitous[0] != "lonely" {
		t.Fatalf("ubiquitous = %v, want [lonely]", rep.Ubiquitous)
	}
}

// ---------------------------------------------------------------------------
// Recorder mechanics
// ---------------------------------------------------------------------------

func TestPairKeyIsUnordered(t *testing.T) {
	if makePairKey("b", "a") != makePairKey("a", "b") {
		t.Fatal("(a,b) and (b,a) are different cells; every pair would be half-counted")
	}
}

func TestDuplicateAtomsInOneSelectionCollapse(t *testing.T) {
	r := NewCoUseRecorder()
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("t%d", i)
		r.Observe(id, []string{"A", "A", "B", "A"})
		r.Settle(id, OutcomeSuccess)
	}

	params := DefaultCoUseParams()
	params.MinLift = 0
	params.UbiquityThreshold = 2
	rep := r.Report(params, nil)

	for _, a := range rep.Atoms {
		if a.Success != 10 {
			t.Fatalf("atom %s counted %d times over 10 turns; a duplication bug would look like a correlation",
				a.ID, a.Success)
		}
	}
	p := pairFor(t, rep, "A", "B")
	if p.Joint != 10 {
		t.Fatalf("joint = %d, want 10", p.Joint)
	}
}

func TestSettleTwiceIsIgnored(t *testing.T) {
	r := NewCoUseRecorder()
	r.Observe("t1", []string{"A", "B"})
	r.Settle("t1", OutcomeSuccess)
	r.Settle("t1", OutcomeSuccess)

	params := DefaultCoUseParams()
	params.MinSupport = 1
	params.MinLift = 0
	params.UbiquityThreshold = 2
	rep := r.Report(params, nil)

	if rep.SuccessSelections != 1 {
		t.Fatalf("success turns = %d, want 1", rep.SuccessSelections)
	}
	if got := pairFor(t, rep, "A", "B").Joint; got != 1 {
		t.Fatalf("joint = %d, want 1", got)
	}
}

func TestSettleWithoutObserveIsIgnored(t *testing.T) {
	r := NewCoUseRecorder()
	r.Settle("never-seen", OutcomeSuccess)
	rep := r.Report(DefaultCoUseParams(), nil)
	if rep.SuccessSelections != 0 {
		t.Fatalf("success turns = %d, want 0", rep.SuccessSelections)
	}
}

func TestUnsettledTurnsAreDisclosedNotDiscarded(t *testing.T) {
	r := NewCoUseRecorder()
	// One more than the pending cap, none settled.
	for i := 0; i < maxPendingTurns+7; i++ {
		r.Observe(fmt.Sprintf("t%d", i), []string{"A", "B"})
	}

	waiting, dropped := r.Pending()
	if waiting != maxPendingTurns {
		t.Fatalf("waiting = %d, want %d", waiting, maxPendingTurns)
	}
	if dropped != 7 {
		t.Fatalf("dropped = %d, want 7", dropped)
	}

	rep := r.Report(DefaultCoUseParams(), nil)
	if rep.PendingTurns != maxPendingTurns || rep.DroppedSelections != 7 {
		t.Fatalf("report hides its own gaps: pending=%d dropped=%d", rep.PendingTurns, rep.DroppedSelections)
	}
}

func TestFailedTurnsAreTalliedSeparately(t *testing.T) {
	r := NewCoUseRecorder()
	for i := 0; i < 10; i++ {
		id := fmt.Sprintf("s%d", i)
		r.Observe(id, []string{"A", "B"})
		r.Settle(id, OutcomeSuccess)
	}
	for i := 0; i < 4; i++ {
		id := fmt.Sprintf("f%d", i)
		r.Observe(id, []string{"A", "C"})
		r.Settle(id, OutcomeFailure)
	}

	params := DefaultCoUseParams()
	params.MinSupport = 1
	params.MinLift = 0
	params.UbiquityThreshold = 2
	rep := r.Report(params, nil)

	if rep.SuccessSelections != 10 || rep.FailureSelections != 4 {
		t.Fatalf("turns = %d success / %d failure, want 10/4", rep.SuccessSelections, rep.FailureSelections)
	}
	// The C atom only ever appeared in failures, so it has no successful
	// co-use. Folding failures into the numerator would recommend the atoms
	// present when things went wrong.
	for _, p := range rep.Pairs {
		if p.A == "C" || p.B == "C" {
			t.Fatalf("a failure-only pair appears in the success statistics: %+v", p)
		}
	}
}

func TestResetClears(t *testing.T) {
	r := NewCoUseRecorder()
	feed(r, 20, func(i int) []string { return []string{"A", "B"} })
	r.Observe("pending", []string{"C"})
	r.Reset()

	rep := r.Report(DefaultCoUseParams(), nil)
	if rep.SuccessSelections != 0 || rep.DistinctAtoms != 0 || len(rep.Pairs) != 0 {
		t.Fatalf("reset left state behind: %+v", rep)
	}
	if waiting, dropped := r.Pending(); waiting != 0 || dropped != 0 {
		t.Fatalf("reset left %d pending / %d dropped", waiting, dropped)
	}
}

func TestNilRecorderIsInert(t *testing.T) {
	// The recorder is optional wiring; a nil one must be safe to call so that
	// disabling measurement never becomes a crash.
	var r *CoUseRecorder
	r.Observe("t", []string{"A"})
	r.Settle("t", OutcomeSuccess)
	r.Reset()
	if w, d := r.Pending(); w != 0 || d != 0 {
		t.Fatalf("nil recorder reported %d/%d", w, d)
	}
	if rep := r.Report(DefaultCoUseParams(), nil); rep.SuccessSelections != 0 {
		t.Fatalf("nil recorder produced a report: %+v", rep)
	}
}

// ---------------------------------------------------------------------------
// Determinism and concurrency
// ---------------------------------------------------------------------------

func TestReportIsDeterministic(t *testing.T) {
	r := NewCoUseRecorder()
	feed(r, 150, func(i int) []string {
		switch i % 4 {
		case 0:
			return []string{"a1", "a2", "a3"}
		case 1:
			return []string{"b1", "b2", "a1"}
		case 2:
			return []string{"c1", "c2"}
		default:
			return []string{"a2", "b1"}
		}
	})

	lookup := func(id string) string { return id[:1] }

	first := r.Report(DefaultCoUseParams(), lookup)
	// Go randomizes map iteration, so an implementation that walked the pair
	// or atom maps without sorting would produce a differently ordered report
	// on every call and make two runs impossible to diff.
	for i := 0; i < 50; i++ {
		if got := r.Report(DefaultCoUseParams(), lookup); !reflect.DeepEqual(first, got) {
			t.Fatalf("report %d differs from the first", i)
		}
	}
}

func TestRecorderIsSafeUnderConcurrency(t *testing.T) {
	r := NewCoUseRecorder()

	var wg sync.WaitGroup
	for w := 0; w < 8; w++ {
		wg.Add(1)
		go func(worker int) {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				id := fmt.Sprintf("w%d-t%d", worker, i)
				r.Observe(id, []string{"A", "B", fmt.Sprintf("x%d", i%5)})
				r.Settle(id, OutcomeSuccess)
			}
		}(w)
	}
	// Reporting concurrently with recording is the realistic case: a readout
	// command runs while turns are still happening.
	for w := 0; w < 2; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 50; i++ {
				_ = r.Report(DefaultCoUseParams(), nil)
				_, _ = r.Pending()
			}
		}()
	}
	wg.Wait()

	rep := r.Report(DefaultCoUseParams(), nil)
	if rep.SuccessSelections != 800 {
		t.Fatalf("success turns = %d, want 800", rep.SuccessSelections)
	}
}

func TestUnionFindGroupsAreStable(t *testing.T) {
	uf := newUnionFind()
	uf.union("b", "a")
	uf.union("c", "b")
	uf.union("e", "d")

	groups := uf.groups()
	if len(groups) != 2 {
		t.Fatalf("groups = %d, want 2", len(groups))
	}
	// Union by lexical order makes the root deterministic, which is what keeps
	// grouped output stable across runs.
	if _, ok := groups["a"]; !ok {
		t.Fatalf("expected 'a' to be the root of its group, got roots %v", keysOf(groups))
	}
	if _, ok := groups["d"]; !ok {
		t.Fatalf("expected 'd' to be the root of its group, got roots %v", keysOf(groups))
	}
}

func keysOf(m map[string][]string) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}

// ---------------------------------------------------------------------------
// The sample unit
// ---------------------------------------------------------------------------

func TestSelectionsWithinOneTurnStaySeparateSamples(t *testing.T) {
	r := NewCoUseRecorder()
	// One turn, two compilations: the turn's own prompt and a shard's. A and B
	// were never in the same prompt; C and D were never in the same prompt as
	// A or B. Merging the selections into a turn-level set would report all
	// four as mutually co-used, which is exactly the false conclusion this
	// analysis exists to avoid.
	for i := 0; i < 20; i++ {
		id := fmt.Sprintf("t%d", i)
		r.Observe(id, []string{"A", "B"})
		r.Observe(id, []string{"C", "D"})
		r.Settle(id, OutcomeSuccess)
	}

	params := DefaultCoUseParams()
	params.MinLift = 0
	params.UbiquityThreshold = 2
	rep := r.Report(params, nil)

	if rep.SuccessSelections != 40 {
		t.Fatalf("selections = %d, want 40 (two compilations per turn over 20 turns)", rep.SuccessSelections)
	}
	for _, p := range rep.Pairs {
		crossed := (p.A == "A" || p.A == "B") && (p.B == "C" || p.B == "D")
		crossed = crossed || ((p.B == "A" || p.B == "B") && (p.A == "C" || p.A == "D"))
		if crossed {
			t.Fatalf("atoms from two different prompts were reported as co-used: %+v", p)
		}
	}

	// Each atom appears in half the selections, and its true partner in the
	// same half: lift 2, not 1.
	if got := pairFor(t, rep, "A", "B"); math.Abs(got.Lift-2.0) > 1e-9 {
		t.Fatalf("lift(A,B) = %v, want 2", got.Lift)
	}
}

func TestOneTurnCannotDominateTheSample(t *testing.T) {
	r := NewCoUseRecorder()
	// A turn compiling in a loop would otherwise let a single outcome supply
	// unbounded evidence.
	for i := 0; i < maxSelectionsPerTurn+10; i++ {
		r.Observe("runaway", []string{"A", "B"})
	}
	r.Settle("runaway", OutcomeSuccess)

	rep := r.Report(DefaultCoUseParams(), nil)
	if rep.SuccessSelections != maxSelectionsPerTurn {
		t.Fatalf("selections = %d, want the cap %d", rep.SuccessSelections, maxSelectionsPerTurn)
	}
	if rep.DroppedSelections != 10 {
		t.Fatalf("dropped = %d, want 10 — over-cap selections must be disclosed, not silently discarded",
			rep.DroppedSelections)
	}
}

// swapCoUse installs a recorder as the process-wide one and returns a restore
// function. Tests that drive the real compiler need their own recorder so they
// do not measure, or be measured by, whatever else the suite is doing.
func swapCoUse(r *CoUseRecorder) func() {
	prev := coUse
	coUse = r
	return func() { coUse = prev }
}
