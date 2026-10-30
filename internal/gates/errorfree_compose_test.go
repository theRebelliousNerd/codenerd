package gates

import (
	"testing"

	"codenerd/internal/lintreport"
)

// A failure that named no finding cannot be located, so narrowing it to a
// scope or laying a scoped result over it keeps the failure. Both used to
// recount it from its findings -- none -- and call it a pass, and a kept
// cycle's baseline then forgot a whole-module failure the cycle never
// re-measured (audit gate-scoped-recurse-current-main-addendum).
func TestRestrictAndOverlay_KeepAFailureNoFindingAccountsFor(t *testing.T) {
	t.Parallel()
	base := Snapshot{ID: "base", Gates: []Measurement{
		{Gate: "vet", Label: "G2", Status: MeasureFail, Count: 0, Findings: []lintreport.Finding{}, Reason: "go exited 1 and named no finding"},
	}}
	after := Snapshot{ID: "after", Gates: []Measurement{
		{Gate: "vet", Label: "G2", Status: MeasurePass, Findings: []lintreport.Finding{}, Scope: []string{"./a"}, Packages: []string{"./a"}},
	}}

	before, _ := base.RestrictTo(after).Gate("vet")
	if before.Status != MeasureFail {
		t.Fatalf("the restricted baseline = %+v: an unlocated failure became a pass", before)
	}
	merged := base.Overlay(after)
	vet, _ := merged.Gate("vet")
	if vet.Status != MeasureFail || !vet.Unenumerated {
		t.Fatalf("the composed baseline = %+v: an unlocated failure was dropped by a clean scoped result", vet)
	}

	// With findings beside it the failure stays unenumerated, so the count
	// is never read as complete.
	after.Gates[0] = Measurement{Gate: "vet", Label: "G2", Status: MeasureFail, Count: 1, Findings: []lintreport.Finding{f("a/a.go", "printf", "m")}, Scope: []string{"./a"}, Packages: []string{"./a"}}
	vet, _ = base.Overlay(after).Gate("vet")
	if vet.Status != MeasureFail || vet.Count != 1 || !vet.Unenumerated {
		t.Fatalf("composed with a finding = %+v, want fail, one finding, still unenumerated", vet)
	}
}

// A composed baseline is not a measurement: it does not take the ID of the
// scoped snapshot whose rows it partly holds, and it says what it was
// composed of, across successive kept cycles.
func TestOverlay_IsComposedNotMeasured(t *testing.T) {
	t.Parallel()
	base := Snapshot{ID: "full", Gates: []Measurement{{Gate: "gofmt", Label: "G10", Status: MeasurePass, Findings: []lintreport.Finding{}}}}
	one := Snapshot{ID: "s1", Gates: []Measurement{{Gate: "gofmt", Label: "G10", Status: MeasurePass, Findings: []lintreport.Finding{}}}}
	two := Snapshot{ID: "s2", Gates: []Measurement{{Gate: "gofmt", Label: "G10", Status: MeasurePass, Findings: []lintreport.Finding{}}}}
	m1 := base.Overlay(one)
	if m1.ID == one.ID {
		t.Fatalf("the composed baseline took the scoped snapshot's ID %s", m1.ID)
	}
	m2 := m1.Overlay(two)
	if got := m2.ComposedOf; m2.ID == two.ID || len(got) != 3 || got[0] != "full" || got[1] != "s1" || got[2] != "s2" {
		t.Fatalf("second composition = %s composed of %v, want its own id over [full s1 s2]", m2.ID, got)
	}
}

// The flag reaches policy: one finding beside an unenumerated failure is not
// fewer than two named findings.
func TestCompare_AnUnenumeratedCountIsNotAnImprovement(t *testing.T) {
	named := Measurement{Gate: "vet", Label: "G2", Status: MeasureFail, Count: 2, Findings: []lintreport.Finding{f("a/a.go", "printf", "x"), f("a/b.go", "printf", "y")}}
	partial := Measurement{Gate: "vet", Label: "G2", Status: MeasureFail, Count: 1, Findings: []lintreport.Finding{f("a/a.go", "printf", "x")}, Unenumerated: true}
	c, err := Compare(admissionKernel(t), Snapshot{ID: "b", Gates: []Measurement{named}}, Snapshot{ID: "a", Gates: []Measurement{partial}})
	if err != nil {
		t.Fatal(err)
	}
	if c.Keepable || len(c.Gates) != 1 || c.Gates[0].Change != "worse" {
		t.Fatalf("named(2) -> unenumerated(1): keepable=%v gates=%+v, want worse", c.Keepable, c.Gates)
	}
}
