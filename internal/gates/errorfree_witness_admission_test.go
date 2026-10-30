package gates

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/lintreport"
)

// A compile diagnostic is a category, not an analyzer: there is no
// `go vet -compile`. FindingGroups made one anyway, so a recurse attempt that
// fixed the compile error was judged by a witness stock vet rejects as a
// usage error, and reverted (audit compile-diagnostic-witness-admission).
// Only a check the gate can select gets a witness.
func TestFindingGroups_ACompileDiagnosticGetsNoWitness(t *testing.T) {
	t.Parallel()
	snap := Snapshot{ID: "s", Gates: []Measurement{
		{Gate: "vet", Label: "G2", Status: MeasureFail, Findings: []lintreport.Finding{
			f("a/a.go", lintreport.CompileCheck, "undefined: missing"),
			f("a/b.go", "printf", "bad verb"),
		}},
		{Gate: "staticcheck", Label: "G4", Status: MeasureFail, Findings: []lintreport.Finding{
			f("b/b.go", lintreport.CompileCheck, "could not type-check"),
			f("b/c.go", "S1002", "omit comparison"),
		}},
		{Gate: "golangci_lint", Label: "G5", Status: MeasureFail, Findings: []lintreport.Finding{
			f("c/c.go", "typecheck", "could not import"),
			f("c/d.go", "errcheck", "unchecked error"),
		}},
	}}
	var checks []string
	for _, g := range FindingGroups(snap) {
		checks = append(checks, g.Gate+":"+g.Check)
		for _, arg := range g.Witness {
			if arg == "-compile" || arg == "compile" || arg == "typecheck" {
				t.Errorf("group %s has a witness selecting a compile diagnostic: %v", g.ID, g.Witness)
			}
		}
	}
	got := strings.Join(checks, ",")
	for _, want := range []string{"vet:printf", "staticcheck:S1002", "golangci_lint:errcheck"} {
		if !strings.Contains(got, want) {
			t.Errorf("groups %s lack %s", got, want)
		}
	}
	if len(checks) != 3 {
		t.Fatalf("groups = %s, want exactly the three analyzer groups", got)
	}
}

// A selector that is not the shape of the gate's check names is refused, so
// no witness smuggles a flag or a category into a command line.
func TestWitnessArgv_AdmitsOnlySelectableChecks(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		gate, check string
		ok          bool
	}{
		{"vet", "printf", true},
		{"vet", "compile", false},
		{"vet", "vettool=/bin/sh", false},
		{"vet", "Printf", false},
		{"staticcheck", "SA4006", true},
		{"staticcheck", "ST1003", true},
		{"staticcheck", "compile", false},
		{"staticcheck", "all", false},
		{"golangci_lint", "errcheck", true},
		{"golangci_lint", "typecheck", false},
		{"golangci_lint", "--fix", false},
		{"gofmt", "gofmt", false},
	} {
		if _, ok := WitnessArgv(c.gate, c.check, []string{"./a"}); ok != c.ok {
			t.Errorf("WitnessArgv(%s, %s) admitted = %v, want %v", c.gate, c.check, ok, c.ok)
		}
	}
}

// A witness that fails says why: its output comes back with the verdict, so
// a failure is attributable rather than a bare exit status.
func TestRunWitness_ReturnsTheOutputOfAFailure(t *testing.T) {
	root := t.TempDir()
	writeModuleFile(t, root, "go.mod", "module witnessprobe\n\ngo 1.22\n")
	writeModuleFile(t, root, "a/a.go", "package a\n\nimport \"fmt\"\n\n// Say prints n.\nfunc Say(n string) { fmt.Printf(\"%d\\n\", n) }\n")
	argv, ok := WitnessArgv("vet", "printf", []string{"./a"})
	if !ok {
		t.Fatal("vet printf has no witness")
	}
	res, err := RunWitness(context.Background(), root, argv)
	if err != nil {
		t.Fatal(err)
	}
	if res.Passed || !strings.Contains(res.Output, "a/a.go") {
		t.Fatalf("witness = %+v, want a failure naming a/a.go", res)
	}
}
