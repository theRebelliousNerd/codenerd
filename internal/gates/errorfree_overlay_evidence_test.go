package gates

import (
	"context"
	"testing"

	"codenerd/internal/lintreport"
)

// Observing nothing changes nothing. A cycle that touched no Go file scopes
// the lint gates to no package; they pass over that empty domain without
// running. Laid over a baseline whose staticcheck was never available, that
// empty pass used to become a whole-module pass, and the next cycle that did
// touch Go -- staticcheck still missing -- read pass -> unavailable as a
// regression (audit scoped-gate-overlay-evidence-conservation).
func TestOverlay_AnEmptyScopeIsTheIdentity(t *testing.T) {
	t.Parallel()
	base := Snapshot{ID: "full", Gates: []Measurement{
		{Gate: "staticcheck", Label: "G4", Status: MeasureUnavailable, Findings: []lintreport.Finding{}, Reason: "staticcheck is not installed"},
		{Gate: "vet", Label: "G2", Status: MeasureFail, Count: 1, Findings: []lintreport.Finding{f("a/a.go", "printf", "m")}},
	}}
	// The real empty-scope early return, not a hand-built pass row.
	empty := MeasureErrorFree(context.Background(), t.TempDir(), MeasureOptions{Only: []string{"staticcheck", "vet"}, ScopedPackages: []string{}})
	for _, g := range empty.Gates {
		if g.Status != MeasurePass {
			t.Fatalf("precondition: %s scoped to no package = %+v, want the empty-domain pass", g.Gate, g)
		}
	}
	merged := base.Overlay(empty)
	for _, want := range base.Gates {
		got, _ := merged.Gate(want.Gate)
		if got.Status != want.Status || got.Count != want.Count || got.Reason != want.Reason {
			t.Fatalf("%s after an empty-scope overlay = %+v, want the baseline row %+v unchanged", want.Gate, got, want)
		}
	}

	// The next cycle touches Go with staticcheck still missing: unavailable
	// against the baseline's unavailable is no regression.
	next := Snapshot{ID: "s2", Root: empty.Root, Gates: []Measurement{
		{Gate: "staticcheck", Label: "G4", Status: MeasureUnavailable, Findings: []lintreport.Finding{}, Scope: []string{"./a"}, Packages: []string{"./a"}, Reason: "staticcheck is not installed"},
		{Gate: "vet", Label: "G2", Status: MeasureFail, Count: 1, Findings: []lintreport.Finding{f("a/a.go", "printf", "m")}, Scope: []string{"./a"}, Packages: []string{"./a"}},
	}}
	c, err := Compare(admissionKernel(t), merged.RestrictTo(next), next)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Keepable {
		t.Fatalf("the next cycle was vetoed: %v", c.NotKeepableBecause())
	}
}

// A clean measurement of one package does not certify the rest of a gate
// that never gave a verdict: the composed row keeps the baseline's
// unverified status, with the findings the scoped run did observe.
func TestOverlay_APartialPassDoesNotCompleteAnUnknownGate(t *testing.T) {
	t.Parallel()
	base := Snapshot{ID: "full", Gates: []Measurement{
		{Gate: "golangci_lint", Label: "G5", Status: MeasureUnverified, Findings: []lintreport.Finding{}, Reason: "stopped: context deadline exceeded"},
	}}
	clean := Snapshot{ID: "s1", Gates: []Measurement{
		{Gate: "golangci_lint", Label: "G5", Status: MeasurePass, Findings: []lintreport.Finding{}, Scope: []string{"./a"}, Packages: []string{"./a"}},
	}}
	g, _ := base.Overlay(clean).Gate("golangci_lint")
	if g.Status != MeasureUnverified {
		t.Fatalf("composed = %+v: one clean package certified a gate that never finished", g)
	}
	found := Snapshot{ID: "s2", Gates: []Measurement{
		{Gate: "golangci_lint", Label: "G5", Status: MeasureFail, Count: 1, Findings: []lintreport.Finding{f("a/a.go", "errcheck", "x")}, Scope: []string{"./a"}, Packages: []string{"./a"}},
	}}
	g, _ = base.Overlay(found).Gate("golangci_lint")
	if g.Status != MeasureUnverified || g.Count != 1 || g.Findings[0].File != "a/a.go" {
		t.Fatalf("composed = %+v, want unverified holding the observed a/a.go finding", g)
	}
}
