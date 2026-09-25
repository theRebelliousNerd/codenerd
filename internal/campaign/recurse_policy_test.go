package campaign

import (
	"fmt"
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
)

// newRecursePolicy is policy/recurse.mg over the shipped kernel: the decisions
// under test are rules, so a mock kernel would only test the mock.
func newRecursePolicy(t *testing.T) *recursePolicy {
	t.Helper()
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return &recursePolicy{k: k}
}

func finding(id, node string, kind gates.Kind) gates.Finding {
	return gates.Finding{ID: id, Node: node, Gate: "g:" + string(kind), Kind: kind, Target: node + "/x", Signature: "sig " + id}
}

// The pick: a red build before a red test before a regression before the
// rest, only on the node being visited, lowest ID among equals.
func TestRecursePolicy_PicksByKindThenRegressionOnTheVisitedNode(t *testing.T) {
	p := newRecursePolicy(t)
	open := []gates.Finding{
		finding("lint-a", "store", gates.Lint),
		finding("audit-new", "store", gates.Audit),
		finding("test-b", "store", gates.Test),
		finding("test-a", "store", gates.Test),
		finding("build-elsewhere", "web", gates.Build),
	}
	steps := []struct {
		open        []gates.Finding
		regressions map[string]bool
		want        string
	}{
		{open, nil, "test-a"},
		{open[:2], map[string]bool{"audit-new": true}, "audit-new"},
		{open[:2], nil, "lint-a"},
		{append(open[:1:1], finding("build-z", "store", gates.Build)), nil, "build-z"},
		{open[4:], nil, ""},
	}
	for i, s := range steps {
		if err := p.visit("store", s.open, s.regressions); err != nil {
			t.Fatal(err)
		}
		got, err := p.next()
		if err != nil {
			t.Fatal(err)
		}
		if got != s.want {
			t.Fatalf("step %d: next = %q, want %q", i, got, s.want)
		}
	}
}

// Two failed attempts that ended the same way stall the finding; a kept
// change to the node lifts it; a refusal takes the finding out until the
// owner acts.
func TestRecursePolicy_StallIsARepeatedFailureNotACounter(t *testing.T) {
	p := newRecursePolicy(t)
	open := []gates.Finding{finding("t1", "store", gates.Test), finding("t2", "store", gates.Test)}
	if err := p.visit("store", open, nil); err != nil {
		t.Fatal(err)
	}
	must := func(err error) {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
	}
	must(p.attempt("t1", "store", 1, outcomeReverted, "boom A"))
	must(p.attempt("t1", "store", 2, outcomeReverted, "boom B"))
	if next, _ := p.next(); next != "t1" {
		t.Fatalf("two failures that differ are new evidence, not a stall: next = %q", next)
	}
	must(p.attempt("t1", "store", 3, outcomeReverted, "boom B"))
	if stalled, _ := p.stalled(); !slices.Contains(stalled, "t1") {
		t.Fatalf("two failures ending the same way must stall: %v", stalled)
	}
	if next, _ := p.next(); next != "t2" {
		t.Fatalf("a stalled finding is skipped: next = %q", next)
	}

	must(p.attempt("other", "store", 4, outcomeKept, ""))
	if stalled, _ := p.stalled(); slices.Contains(stalled, "t1") {
		t.Fatalf("a kept change to the node lifts the stall: %v", stalled)
	}

	must(p.attempt("t2", "store", 5, outcomeRefused, "wrote a forbidden path"))
	if refused, _ := p.refused(); !slices.Contains(refused, "t2") {
		t.Fatalf("refused = %v", refused)
	}
	if next, _ := p.next(); next != "t1" {
		t.Fatalf("a refused finding is not retried: next = %q", next)
	}
}

func TestRecursePolicy_RatchetKeepsOnlyAResolvedTargetWithNothingWorse(t *testing.T) {
	p := newRecursePolicy(t)
	gate := func(name, before, after string, bc, ac int) ratchetGate {
		return ratchetGate{Gate: name, Before: before, After: after, BeforeCount: bc, AfterCount: ac}
	}
	cases := []struct {
		name string
		in   ratchetInput
		want string
	}{
		{"fixed, nothing worse", ratchetInput{TargetOK: true, Changed: true, Gates: []ratchetGate{
			gate("test", verdictFail, verdictPass, 1, 0), gate("build", verdictPass, verdictPass, 0, 0)}}, ratchetKeep},
		{"fixed, one error uncovered the next", ratchetInput{TargetOK: true, Changed: true, Gates: []ratchetGate{
			gate("build", verdictFail, verdictFail, 2, 2)}}, ratchetKeep},
		{"fixed its target, broke another gate", ratchetInput{TargetOK: true, Changed: true, Gates: []ratchetGate{
			gate("test", verdictFail, verdictPass, 1, 0), gate("vet", verdictPass, verdictFail, 0, 1)}}, ratchetRevert},
		{"fixed, more findings", ratchetInput{TargetOK: true, Changed: true, Gates: []ratchetGate{
			gate("lint", verdictFail, verdictFail, 2, 3)}}, ratchetRevert},
		{"fixed, a gate lost its verdict", ratchetInput{TargetOK: true, Changed: true, Gates: []ratchetGate{
			gate("build", verdictPass, verdictUnverified, 0, 0)}}, ratchetRevert},
		{"target still open", ratchetInput{TargetOK: false, Changed: true, Gates: []ratchetGate{
			gate("test", verdictFail, verdictFail, 1, 1)}}, ratchetRevert},
		{"changed nothing", ratchetInput{TargetOK: false, Changed: false}, ratchetRevert},
		{"wrote a forbidden path", ratchetInput{TargetOK: true, Changed: true, Forbidden: []string{"internal/core/defaults/policy/constitution.mg"}}, ratchetRefuse},
	}
	for i, tc := range cases {
		tc.in.Cycle = 100 + i
		tc.in.TargetID = "target"
		got, err := p.ratchet(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: verdict %q, want %q", tc.name, got, tc.want)
		}
	}
}

func TestRecursePolicy_RatchetKinds(t *testing.T) {
	kinds, err := newRecursePolicy(t).ratchetKinds()
	if err != nil {
		t.Fatal(err)
	}
	if !kinds[gates.Build] || !kinds[gates.Lint] || kinds[gates.Test] || kinds[gates.Audit] {
		t.Fatalf("every attempt re-runs workspace build and lint; tests and audits run per pass: %v", kinds)
	}
}

// Each pass leads with one angle, in rotation, on nodes where its metric can
// be measured.
func TestRecursePolicy_ImprovementAngleRotatesByPass(t *testing.T) {
	p := newRecursePolicy(t)
	measurable := map[string]int{gates.MetricTests: 10, gates.MetricLines: 300}
	var got []string
	for pass := range 5 {
		angle, err := p.improveAngle("store", pass, measurable)
		if err != nil {
			t.Fatal(err)
		}
		got = append(got, angle)
	}
	if want := []string{"stabilize", "harden", "simplify", "extend", "stabilize"}; !slices.Equal(got, want) {
		t.Fatalf("angles by pass = %v, want %v", got, want)
	}
	// simplify needs the node's lines; a node where they cannot be measured
	// gets no improvement step that pass.
	if angle, err := p.improveAngle("store", 2, map[string]int{gates.MetricTests: 10}); err != nil || angle != "" {
		t.Fatalf("simplify without a lines metric = %q, %v; want none", angle, err)
	}
}

// An improvement is kept only if its angle's metric moved the right way and
// no guard metric moved the wrong way; a fix that deletes tests is reverted
// even though its finding is gone.
func TestRecursePolicy_ImprovementKeptOnlyOnAMeasuredMove(t *testing.T) {
	p := newRecursePolicy(t)
	m := func(kv ...int) map[string]int {
		return map[string]int{gates.MetricTests: kv[0], gates.MetricLines: kv[1], gates.MetricCoverage: kv[2]}
	}
	pass := []ratchetGate{{Gate: "test", Before: verdictPass, After: verdictPass}}
	cases := []struct {
		name string
		in   ratchetInput
		want string
	}{
		{"stabilize adds tests", ratchetInput{Improve: "stabilize", Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(12, 300, 5200)}, ratchetKeep},
		{"stabilize moves nothing", ratchetInput{Improve: "stabilize", Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(10, 310, 5000)}, ratchetRevert},
		{"stabilize adds tests but drops coverage", ratchetInput{Improve: "stabilize", Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(11, 300, 4900)}, ratchetRevert},
		{"harden raises coverage", ratchetInput{Improve: "harden", Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(10, 305, 5400)}, ratchetKeep},
		{"simplify drops lines", ratchetInput{Improve: "simplify", Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(10, 250, 5000)}, ratchetKeep},
		{"simplify drops lines by deleting tests", ratchetInput{Improve: "simplify", Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(8, 250, 5000)}, ratchetRevert},
		{"extend adds a tested capability", ratchetInput{Improve: "extend", Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(11, 340, 5000)}, ratchetKeep},
		{"extend that breaks a gate", ratchetInput{Improve: "extend", Changed: true, Gates: []ratchetGate{{Gate: "build", Before: verdictPass, After: verdictFail, AfterCount: 1}}, Before: m(10, 300, 5000), After: m(11, 340, 5000)}, ratchetRevert},
		{"a fix that deletes the failing test", ratchetInput{TargetID: "t", TargetOK: true, Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(9, 300, 5000)}, ratchetRevert},
		{"a fix that keeps the tests", ratchetInput{TargetID: "t", TargetOK: true, Changed: true, Gates: pass, Before: m(10, 300, 5000), After: m(10, 301, 4900)}, ratchetKeep},
	}
	for i, tc := range cases {
		tc.in.Cycle = 200 + i
		tc.in.Tested = true
		got, err := p.ratchet(tc.in)
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		if got != tc.want {
			t.Fatalf("%s: verdict %q, want %q", tc.name, got, tc.want)
		}
	}
}

// A forever run leaves a bounded number of attempt facts: per finding its last
// two, and none from before the node's last kept change. Refusals stay.
func TestRecursePolicy_AttemptMemoryIsBounded(t *testing.T) {
	p := newRecursePolicy(t)
	count := func() int {
		rows, err := p.k.Query("recurse_attempt")
		if err != nil {
			t.Fatal(err)
		}
		return len(rows)
	}
	for c := 1; c <= 200; c++ {
		// Every attempt fails differently: new evidence each time, so no
		// stall -- and still only the last two are kept.
		if err := p.attempt("g", "store", c, outcomeReverted, fmt.Sprintf("failure %d-%c", c, 'a'+c%26)); err != nil {
			t.Fatal(err)
		}
	}
	if stalled, _ := p.stalled(); slices.Contains(stalled, "g") {
		t.Fatalf("failures that differ are not a stall: %v", stalled)
	}
	for c := 201; c <= 400; c++ {
		if err := p.attempt("f", "store", c, outcomeReverted, "same failure"); err != nil {
			t.Fatal(err)
		}
	}
	if got := count(); got != 4 {
		t.Fatalf("400 failures over two findings leave %d attempt facts, want 4", got)
	}
	if stalled, _ := p.stalled(); !slices.Contains(stalled, "f") {
		t.Fatalf("pruning must not lose the stall: %v", stalled)
	}
	if err := p.attempt("r", "store", 201, outcomeRefused, "forbidden"); err != nil {
		t.Fatal(err)
	}
	if err := p.attempt("x", "store", 202, outcomeKept, ""); err != nil {
		t.Fatal(err)
	}
	if got := count(); got != 1 {
		t.Fatalf("a kept change retires the node's attempts but not its refusal: %d facts", got)
	}
	if refused, _ := p.refused(); !slices.Contains(refused, "r") {
		t.Fatalf("refusal lost: %v", refused)
	}
	kept, err := p.k.Query("recurse_node_kept")
	if err != nil {
		t.Fatal(err)
	}
	if err := p.attempt("y", "store", 203, outcomeKept, ""); err != nil {
		t.Fatal(err)
	}
	kept2, err := p.k.Query("recurse_node_kept")
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || len(kept2) != 1 {
		t.Fatalf("only a node's latest kept change is kept: %d then %d", len(kept), len(kept2))
	}
}

// An improvement no test gate ran on is not kept, whatever its metrics say:
// counting test functions says tests were added, not that they pass.
func TestRecursePolicy_AnImprovementNeedsATestVerdict(t *testing.T) {
	p := newRecursePolicy(t)
	in := ratchetInput{
		Cycle: 300, Improve: "stabilize", Changed: true,
		Before: map[string]int{gates.MetricTests: 10}, After: map[string]int{gates.MetricTests: 12},
	}
	if got, err := p.ratchet(in); err != nil || got != ratchetRevert {
		t.Fatalf("untested improvement = %q, %v; want revert", got, err)
	}
	in.Cycle, in.Tested = 301, true
	if got, err := p.ratchet(in); err != nil || got != ratchetKeep {
		t.Fatalf("tested improvement = %q, %v; want keep", got, err)
	}
}
