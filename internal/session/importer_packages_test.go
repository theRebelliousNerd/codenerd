package session

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// A module where b imports a, and b's test pins what a returns.
func importerModule(t *testing.T, aSrc string) string {
	t.Helper()
	return writeBaselineModule(t, map[string]string{
		"go.mod":      "module impprobe\n\ngo 1.21\n",
		"a/a.go":      aSrc,
		"a/a_test.go": "package a\n\nimport \"testing\"\n\nfunc TestNameIsNotEmpty(t *testing.T) {\n\tif Name() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n",
		"b/b.go":      "package b\n\nimport \"impprobe/a\"\n\nfunc Label() string { return a.Name() }\n",
		"b/b_test.go": "package b\n\nimport \"testing\"\n\nfunc TestLabelIsTheOldName(t *testing.T) {\n\tif Label() != \"widget\" {\n\t\tt.Fatalf(\"Label = %q, want widget\", Label())\n\t}\n}\n",
	})
}

const (
	aBefore = "package a\n\nfunc Name() string { return \"widget\" }\n"
	aAfter  = "package a\n\nfunc Name() string { return \"a.widget\" }\n"
)

// N25: the turn's own package is green and the package that imports it is
// not. Until this the gate saw only the first and the turn ended /done.
func TestGateTests_RunsThePackagesThatImportWhatTheTurnWrote(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := importerModule(t, aAfter)
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}

	own, _ := gateOwnTests(context.Background(), ws, result, false)
	if own.Verdict() != VerifyPassed {
		t.Fatalf("the turn's own package = %s (%s), want passed: this is what the gate used to see", own.Verdict(), own.Output)
	}

	v, _ := gateTests(context.Background(), ws, result, false)
	if v.Verdict() != VerifyPassed {
		t.Fatalf("own run = %s (%s), want passed: the gate's return is the turn's own package", v.Verdict(), v.Output)
	}
	imp := result.ImporterCheck
	if imp.Verdict() != VerifyFailed {
		t.Fatalf("importer run = %s (%s), want failed: b's test pins what a returns", imp.Verdict(), imp.Reason)
	}
	if !strings.Contains(imp.Output, "TestLabelIsTheOldName") {
		t.Errorf("the importer run does not name the failing test:\n%s", imp.Output)
	}
	if !strings.Contains(imp.Reason, "import what this turn changed") {
		t.Errorf("the importer run does not say whose tests failed: %q", imp.Reason)
	}
	result.TestCheck = v
	e := newObligationExec(t)
	e.syncTestGateFacts(testTurn, result)
	if got := derivedVerify(t, e, testTurn, "/test"); got != VerifyFailed {
		t.Fatalf("test gate = %s, want failing: TestLabelIsTheOldName did not fail before the turn\n%s", got, imp.Output)
	}
}

// A turn that changes nothing the importer can see leaves it green, and the
// gate passes.
func TestGateTests_AnImporterThatStillHoldsIsNoFailure(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := importerModule(t, "package a\n\nfunc Name() string { return \"widget\" }\n\nfunc Extra() int { return 1 }\n")
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}

	v, _ := gateTests(context.Background(), ws, result, false)
	if v.Verdict() != VerifyPassed {
		t.Fatalf("own run = %s (%s):\n%s\nwant passed", v.Verdict(), v.Reason, v.Output)
	}
	if result.ImporterCheck.Verdict() != VerifyPassed {
		t.Fatalf("importer run = %s (%s):\n%s\nwant passed", result.ImporterCheck.Verdict(), result.ImporterCheck.Reason, result.ImporterCheck.Output)
	}
	result.TestCheck = v
	e := newObligationExec(t)
	e.syncTestGateFacts(testTurn, result)
	if got := derivedVerify(t, e, testTurn, "/test"); got != VerifyPassed {
		t.Fatalf("test gate = %s, want passing", got)
	}
}

// An importer that was already failing before the turn is not the turn's:
// the same baseline attribution the turn's own packages get.
func TestGateTests_AnImporterFailingBeforeTheTurnIsNotCharged(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := importerModule(t, aBefore)
	writeWorkspaceFile(t, ws, "b/b_test.go",
		"package b\n\nimport \"testing\"\n\nfunc TestAlreadyRed(t *testing.T) { t.Fatal(\"red before the turn\") }\n")
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	// The turn added a function; b's failure has nothing to do with it.
	writeWorkspaceFile(t, ws, "a/a.go", aBefore+"\nfunc Added() int { return 2 }\n")
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}

	v, _ := gateTests(context.Background(), ws, result, false)
	if v.Verdict() != VerifyPassed {
		t.Fatalf("own run = %s (%s), want passed: the turn added a function and a's test still holds\n%s", v.Verdict(), v.Reason, v.Output)
	}
	imp := result.ImporterCheck
	if imp.Verdict() != VerifyFailed {
		t.Fatalf("importer process = %s (%s), want the suite's own failure left in place", imp.Verdict(), imp.Reason)
	}
	if !strings.Contains(imp.Output, "TestAlreadyRed") {
		t.Fatalf("the text does not name the pre-existing failure:\n%s", imp.Output)
	}
	result.TestCheck = v
	e := newObligationExec(t)
	e.syncTestGateFacts(testTurn, result)
	if got := derivedVerify(t, e, testTurn, "/test"); got != VerifyPassed {
		t.Fatalf("test gate = %v, want passing: TestAlreadyRed failed before the turn\n%s", got, imp.Output)
	}
}

// An importer run that does not finish is not a pass. verifyImporters on a
// canceled context never lists packages and comes back skipped, which must
// not block; the unfinished case is the test run after a successful list.
// verifyTests returns that cancel as itself. Indeterminate is the same
// /unfinished atom (importer_gate_test.go); a hang is not this test.
func TestGateTests_AnUnfinishedImporterRunDoesNotPass(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := importerModule(t, "package a\n\nfunc Name() string { return \"widget\" }\n\nfunc Extra() int { return 1 }\n")
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}

	own, _ := gateOwnTests(context.Background(), ws, result, false)
	if own.Verdict() != VerifyPassed {
		t.Fatalf("own run = %s (%s), want passed", own.Verdict(), own.Output)
	}
	result.TestCheck = own
	patterns, _ := splitTagGatedPackages(ws, packagesForPaths(result.WrittenPaths))
	importers := importerPackages(context.Background(), ws, patterns)
	if len(importers) == 0 {
		t.Fatal("the fixture has no importer; the cancel would not be a measurement")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	imp := verifyTests(ctx, ws, importers)
	if imp.Verdict() != VerifyCanceled {
		t.Fatalf("importer run = %s (%s), want canceled before it started", imp.Verdict(), imp.Reason)
	}
	result.ImporterCheck = imp
	e := newObligationExec(t)
	e.syncTestGateFacts(testTurn, result)
	if got := derivedVerify(t, e, testTurn, "/test"); got != VerifySkipped {
		t.Fatalf("test gate = %s, want neither passing nor failing: the importer run did not finish", got)
	}
}

// The importers are the packages that import the turn's, through code or
// tests, and never the turn's own.
func TestImporterPackages_OneHopThroughCodeAndTests(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":      "module impprobe\n\ngo 1.21\n",
		"a/a.go":      aBefore,
		"b/b.go":      "package b\n\nimport \"impprobe/a\"\n\nfunc Label() string { return a.Name() }\n",
		"c/c.go":      "package c\n\nfunc Unrelated() int { return 0 }\n",
		"c/c_test.go": "package c\n\nimport (\n\t\"testing\"\n\n\t\"impprobe/a\"\n)\n\nfunc TestUsesA(t *testing.T) {\n\tif a.Name() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n",
		"d/d.go":      "package d\n\nimport \"impprobe/b\"\n\nfunc Deep() string { return b.Label() }\n",
		// An external test package imports the package it tests, which would
		// make a an importer of itself and run its suite twice.
		"a/a_x_test.go": "package a_test\n\nimport (\n\t\"testing\"\n\n\t\"impprobe/a\"\n)\n\nfunc TestFromOutside(t *testing.T) {\n\tif a.Name() == \"\" {\n\t\tt.Fatal(\"empty\")\n\t}\n}\n",
	})

	got := importerPackages(context.Background(), ws, []string{"./a"})
	slices.Sort(got)
	if !slices.Equal(got, []string{"impprobe/b", "impprobe/c"}) {
		t.Fatalf("importers = %v, want [impprobe/b impprobe/c]: b imports a in code, c only in its test, d is a hop further out", got)
	}
}
