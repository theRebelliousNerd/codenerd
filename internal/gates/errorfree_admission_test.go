package gates

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/lintreport"
)

// A comparison says "no gate got worse" only about gates both snapshots
// measured over the same domain, with counts that enumerate their findings
// (audit gate-comparison-domain-admission). Each case below used to be
// keepable.

func admissionKernel(t *testing.T) *core.RealKernel {
	t.Helper()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	return k
}

func unadmittedReasons(c Comparison) map[string]string {
	out := map[string]string{}
	for _, u := range c.Unadmitted {
		out[u.Gate] = u.Reason
	}
	return out
}

// G1: a gate measured before and not after left the comparison silently.
func TestCompare_AGateMissingAfterIsNotKeepable(t *testing.T) {
	k := admissionKernel(t)
	before := Snapshot{ID: "b", Gates: []Measurement{
		{Gate: "vet", Label: "G2", Status: MeasurePass, Findings: []lintreport.Finding{}},
		{Gate: "test", Label: "G3", Status: MeasureFail, Count: 1, Findings: []lintreport.Finding{{File: "a/a_test.go", Check: "TestA", Message: "fail"}}},
	}}
	after := Snapshot{ID: "a", Gates: []Measurement{
		{Gate: "vet", Label: "G2", Status: MeasurePass, Findings: []lintreport.Finding{}},
	}}
	c, err := Compare(k, before, after)
	if err != nil {
		t.Fatal(err)
	}
	if c.Keepable {
		t.Fatal("dropping the test gate from the after snapshot was keepable")
	}
	if got := unadmittedReasons(c)["test"]; got != "missing_after" {
		t.Fatalf("unadmitted = %+v, want test missing_after", c.Unadmitted)
	}

	// The other direction too: an after-only gate has no baseline.
	c, err = Compare(k, after, before)
	if err != nil {
		t.Fatal(err)
	}
	if c.Keepable || unadmittedReasons(c)["test"] != "missing_before" {
		t.Fatalf("after-only gate: keepable=%v unadmitted=%+v", c.Keepable, c.Unadmitted)
	}
}

// Two empty snapshots compare nothing, so they cannot vouch for a cycle.
func TestCompare_AnEmptyComparisonIsNotKeepable(t *testing.T) {
	c, err := Compare(admissionKernel(t), Snapshot{ID: "b"}, Snapshot{ID: "a"})
	if err != nil {
		t.Fatal(err)
	}
	if c.Keepable || !c.Empty {
		t.Fatalf("empty comparison: keepable=%v empty=%v", c.Keepable, c.Empty)
	}
}

// G3: a failure that named no finding is not a smaller count. N named
// findings becoming one opaque failure is worse: the gate can no longer
// show it did not get worse. The reverse is not an improvement either.
func TestCompare_AnOpaqueFailureIsNotFewerFindings(t *testing.T) {
	k := admissionKernel(t)
	named := Measurement{Gate: "build", Label: "G1", Status: MeasureFail, Count: 2, Findings: []lintreport.Finding{
		{File: "a.go", Check: "compile", Message: "x"}, {File: "b.go", Check: "compile", Message: "y"},
	}}
	opaque := Measurement{Gate: "build", Label: "G1", Status: MeasureFail, Count: 0, Findings: []lintreport.Finding{}, Reason: "go exited 1 and named no finding"}
	c, err := Compare(k, Snapshot{ID: "b", Gates: []Measurement{named}}, Snapshot{ID: "a", Gates: []Measurement{opaque}})
	if err != nil {
		t.Fatal(err)
	}
	if c.Keepable || len(c.Gates) != 1 || c.Gates[0].Change != "worse" {
		t.Fatalf("named -> opaque failure: keepable=%v gates=%+v, want worse", c.Keepable, c.Gates)
	}
	c, err = Compare(k, Snapshot{ID: "b", Gates: []Measurement{opaque}}, Snapshot{ID: "a", Gates: []Measurement{opaque}})
	if err != nil {
		t.Fatal(err)
	}
	if len(c.Gates) != 1 || c.Gates[0].Change != "same" {
		t.Fatalf("opaque -> opaque: %+v, want same", c.Gates)
	}
}

// G2, on real measurements: the test gate over the whole module fails; the
// same gate over one passing package is a different experiment, not an
// improvement.
func TestCompare_ANarrowerTestScopeIsNotAnImprovement(t *testing.T) {
	root := t.TempDir()
	writeModuleFile(t, root, "go.mod", "module scopeprobe\n\ngo 1.22\n")
	writeModuleFile(t, root, "good/good.go", "package good\n\n// One is 1.\nfunc One() int { return 1 }\n")
	writeModuleFile(t, root, "good/good_test.go", "package good\n\nimport \"testing\"\n\nfunc TestOne(t *testing.T) {\n\tif One() != 1 {\n\t\tt.Fatal(\"one\")\n\t}\n}\n")
	writeModuleFile(t, root, "bad/bad.go", "package bad\n\n// Two is 2.\nfunc Two() int { return 3 }\n")
	writeModuleFile(t, root, "bad/bad_test.go", "package bad\n\nimport \"testing\"\n\nfunc TestTwo(t *testing.T) {\n\tif Two() != 2 {\n\t\tt.Fatal(\"two\")\n\t}\n}\n")
	ctx := context.Background()

	whole := MeasureErrorFree(ctx, root, MeasureOptions{Only: []string{"test"}})
	narrow := MeasureErrorFree(ctx, root, MeasureOptions{Only: []string{"test"}, TestPackages: []string{"./good"}})
	if w, _ := whole.Gate("test"); w.Status != MeasureFail {
		t.Fatalf("precondition: the whole-module test gate = %+v, want fail", w)
	}
	if n, _ := narrow.Gate("test"); n.Status != MeasurePass {
		t.Fatalf("precondition: the ./good test gate = %+v, want pass", n)
	}
	c, err := Compare(admissionKernel(t), whole, narrow)
	if err != nil {
		t.Fatal(err)
	}
	if c.Keepable {
		t.Fatal("narrowing the test gate to a passing package was keepable")
	}
	if got := unadmittedReasons(c)["test"]; got != "domain_changed" {
		t.Fatalf("unadmitted = %+v, want test domain_changed", c.Unadmitted)
	}

	// The same scope measured twice is comparable.
	again := MeasureErrorFree(ctx, root, MeasureOptions{Only: []string{"test"}, TestPackages: []string{"./good"}})
	c, err = Compare(admissionKernel(t), narrow, again)
	if err != nil {
		t.Fatal(err)
	}
	if !c.Keepable || len(c.Unadmitted) != 0 {
		t.Fatalf("one scope measured twice: keepable=%v unadmitted=%+v", c.Keepable, c.Unadmitted)
	}
}

// A requested gate that names no gate is an error, not an empty selection.
func TestUnknownGates(t *testing.T) {
	got := UnknownGates(MeasureOptions{Only: []string{"vet", "G4", "nope", "  "}})
	if strings.Join(got, ",") != "nope" {
		t.Fatalf("UnknownGates = %v, want [nope]", got)
	}
}
