package gates

import (
	"context"
	"slices"
	"testing"

	"codenerd/internal/lintreport"
)

func f(file, check, msg string) lintreport.Finding {
	return lintreport.Finding{File: file, Line: 1, Check: check, Message: msg}
}

// A scoped after-measure is compared with the baseline restricted to the same
// packages, and a kept one is laid over the baseline.
func TestSnapshot_RestrictToAndOverlay(t *testing.T) {
	t.Parallel()
	base := Snapshot{ID: "base", Gates: []Measurement{
		{Gate: "staticcheck", Label: "G4", Status: MeasureFail, Count: 3, Findings: []lintreport.Finding{
			f("a/a.go", "S1002", "x"), f("b/b.go", "S1002", "y"), f("b/sub/c.go", "SA4006", "z"),
		}},
		{Gate: "gofmt", Label: "G10", Status: MeasurePass, Findings: []lintreport.Finding{}},
	}}
	after := Snapshot{ID: "after", Gates: []Measurement{
		{Gate: "staticcheck", Label: "G4", Status: MeasurePass, Count: 0, Findings: []lintreport.Finding{}, Scope: []string{"./a"}},
		{Gate: "gofmt", Label: "G10", Status: MeasurePass, Findings: []lintreport.Finding{}},
	}}
	before := base.RestrictTo(after)
	sc, _ := before.Gate("staticcheck")
	if sc.Count != 1 || sc.Findings[0].File != "a/a.go" || sc.Status != MeasureFail {
		t.Fatalf("restricted baseline = %+v, want a/a.go only", sc)
	}
	merged := base.Overlay(after)
	sc, _ = merged.Gate("staticcheck")
	if merged.ID != "after" || sc.Count != 2 || sc.Findings[0].File != "b/b.go" || len(sc.Scope) != 0 {
		t.Fatalf("overlay = %+v, want b's two findings kept and a's gone, unscoped", sc)
	}
	if !InScope(f("b/sub/c.go", "", ""), []string{"./b/..."}) || InScope(f("b/sub/c.go", "", ""), []string{"./b"}) || !InScope(f("main.go", "", ""), []string{"."}) {
		t.Fatal("InScope disagrees with Go package patterns")
	}
}

// A lint gate scoped to no package does not run and passes over an empty
// scope, so a cycle that touched no Go file compares nothing it could not
// have changed.
func TestMeasure_ScopedToNoPackage(t *testing.T) {
	t.Parallel()
	snap := MeasureErrorFree(context.Background(), t.TempDir(), MeasureOptions{Only: []string{"vet"}, ScopedPackages: []string{}})
	vet, _ := snap.Gate("vet")
	if vet.Status != MeasurePass || !slices.Equal(vet.Scope, []string{NoPackageScope}) || vet.ExitCode != nil {
		t.Fatalf("vet scoped to no package = %+v", vet)
	}
	base := Snapshot{ID: "b", Gates: []Measurement{{Gate: "vet", Status: MeasureFail, Count: 1, Findings: []lintreport.Finding{f("a/a.go", "printf", "m")}}}}
	if r, _ := base.RestrictTo(snap).Gate("vet"); r.Count != 0 || r.Status != MeasurePass {
		t.Fatalf("baseline restricted to no package = %+v", r)
	}
}

// Groups are one check from one gate in one package, each with a witness
// that runs the gate over the package with only that check; gates with no
// per-check exit form no group.
func TestFindingGroups(t *testing.T) {
	t.Parallel()
	s := Snapshot{Gates: []Measurement{
		{Gate: "staticcheck", Label: "G4", Findings: []lintreport.Finding{f("a/x.go", "ST1005", "1"), f("a/y.go", "ST1005", "2"), f("a/x.go", "S1002", "3")}},
		{Gate: "vet", Label: "G2", Findings: []lintreport.Finding{f("main.go", "printf", "4")}},
		{Gate: "gofmt", Label: "G10", Findings: []lintreport.Finding{f("a/x.go", "gofmt", "not gofmt-formatted")}},
	}}
	groups := FindingGroups(s)
	if len(groups) != 3 {
		t.Fatalf("groups = %+v, want ST1005@a, S1002@a, printf@.", groups)
	}
	byCheck := map[string]FindingGroup{}
	for _, g := range groups {
		byCheck[g.Check] = g
	}
	if g := byCheck["ST1005"]; g.Files != 2 || len(g.Findings) != 2 || !slices.Equal(g.Witness, []string{"staticcheck", "-checks", "ST1005", "./a"}) {
		t.Fatalf("ST1005 group = %+v", g)
	}
	if g := byCheck["printf"]; g.Package != "." || !slices.Equal(g.Witness, []string{"go", "vet", "-printf", "."}) {
		t.Fatalf("printf group = %+v", g)
	}
	if again := FindingGroups(s); again[0].ID != groups[0].ID {
		t.Fatal("group IDs are not stable")
	}
}
