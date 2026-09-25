package campaign

import (
	"slices"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
)

// newRecursePolicy is policy/recurse.mg over the shipped kernel: the decisions
// under test are rules, so a mock kernel would only test the mock.
func newRecursePolicy(t *testing.T) recursePolicy {
	t.Helper()
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	return recursePolicy{k: k}
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
