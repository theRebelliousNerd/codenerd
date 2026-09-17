package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// F-VERIFY-1: failures that already fail before the turn must not be charged
// to the turn. These tests pin attributeTestFailures against real throwaway
// modules, reusing the temp-module pattern from test_verify_test.go.

func writeBaselineModule(t *testing.T, files map[string]string) string {
	t.Helper()
	ws := t.TempDir()
	for name, content := range files {
		p := filepath.Join(ws, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatalf("write %s: %v", name, err)
		}
	}
	return ws
}

func writeWorkspaceFile(t *testing.T, ws, name, content string) {
	t.Helper()
	p := filepath.Join(ws, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(p), err)
	}
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
}

// TestAttributeTestFailures_AllPreExistingPasses covers case (a): the turn
// adds an unrelated func while TestAlwaysFails fails identically before and
// after. The gate must pass and name the pre-existing failure.
func TestAttributeTestFailures_AllPreExistingPasses(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	origCalc := "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n"
	testFile := "package verifyprobe\n\nimport \"testing\"\n\n" +
		"func TestAlwaysFails(t *testing.T) { t.Fatal(\"always fails\") }\n" +
		"func TestOK(t *testing.T) { if Add(2, 3) != 5 { t.Fatal(\"bad\") } }\n"
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":      "module verifyprobe\n\ngo 1.21\n",
		"calc.go":     origCalc,
		"calc_test.go": testFile,
	})
	preWrite := map[string]string{"calc.go": origCalc}

	// Turn edit: add an unrelated exported func, tests untouched.
	writeWorkspaceFile(t, ws, "calc.go", origCalc+"\nfunc Unrelated() int { return 42 }\n")

	head := verifyTests(context.Background(), ws, []string{"."})
	if head.Outcome != VerifyFailed {
		t.Fatalf("head should fail on TestAlwaysFails, got Outcome=%v OK=%v output=%q", head.Outcome, head.OK, head.Output)
	}

	got := attributeTestFailures(context.Background(), ws, []string{"."}, preWrite, head)
	if got.Outcome != VerifyPassed {
		t.Fatalf("all-pre-existing gate should pass, got Outcome=%v output=%q", got.Outcome, got.Output)
	}
	if !got.OK || !got.Ran {
		t.Errorf("all-pre-existing gate should report Ran=true OK=true, got Ran=%v OK=%v", got.Ran, got.OK)
	}
	if len(got.PreExistingFailures) != 1 || got.PreExistingFailures[0] != "TestAlwaysFails" {
		t.Errorf("PreExistingFailures = %v; want [TestAlwaysFails]", got.PreExistingFailures)
	}
}

// TestAttributeTestFailures_MixedNewAndPreExisting covers case (b): the
// turn breaks TestOK while TestAlwaysFails was already failing. The gate
// must stay failed, name the pre-existing failure, and prefix the output
// with the "Pre-existing failures" line while keeping TestOK's failure.
func TestAttributeTestFailures_MixedNewAndPreExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	origCalc := "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n"
	testFile := "package verifyprobe\n\nimport \"testing\"\n\n" +
		"func TestAlwaysFails(t *testing.T) { t.Fatal(\"always fails\") }\n" +
		"func TestOK(t *testing.T) { if Add(2, 3) != 5 { t.Fatalf(\"Add broken\") } }\n"
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":      "module verifyprobe\n\ngo 1.21\n",
		"calc.go":     origCalc,
		"calc_test.go": testFile,
	})
	preWrite := map[string]string{"calc.go": origCalc}

	// Turn edit breaks Add, so TestOK is newly failing.
	writeWorkspaceFile(t, ws, "calc.go", "package verifyprobe\n\nfunc Add(a, b int) int { return a + b + 1 }\n")

	head := verifyTests(context.Background(), ws, []string{"."})
	if head.Outcome != VerifyFailed {
		t.Fatalf("head should fail, got Outcome=%v output=%q", head.Outcome, head.Output)
	}

	got := attributeTestFailures(context.Background(), ws, []string{"."}, preWrite, head)
	if got.Outcome != VerifyFailed {
		t.Fatalf("mixed gate should stay failed, got Outcome=%v output=%q", got.Outcome, got.Output)
	}
	if len(got.PreExistingFailures) != 1 || got.PreExistingFailures[0] != "TestAlwaysFails" {
		t.Fatalf("PreExistingFailures = %v; want [TestAlwaysFails]", got.PreExistingFailures)
	}
	const prefix = "Pre-existing failures (also fail without this turn's edits; not yours to fix): TestAlwaysFails"
	if !strings.HasPrefix(got.Output, prefix) {
		t.Errorf("output should begin with %q, got %q", prefix, got.Output)
	}
	if !strings.Contains(got.Output, "TestOK") {
		t.Errorf("output should still contain TestOK's failure, got %q", got.Output)
	}
}

// TestAttributeTestFailures_TurnCreatedFileIsNew covers case (c): a file
// created by the turn (preWrite value "") introduces the failure. The
// baseline overlay deletes it, the test passes at baseline, so the gate
// must stay failed with no pre-existing failures.
func TestAttributeTestFailures_TurnCreatedFileIsNew(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":  "module verifyprobe\n\ngo 1.21\n",
		"calc.go": "package verifyprobe\n\nvar Extra = \"\"\n",
		"calc_test.go": "package verifyprobe\n\nimport \"testing\"\n\n" +
			"func TestExtra(t *testing.T) { if Extra != \"\" { t.Fatalf(\"broken by new file: %q\", Extra) } }\n",
	})

	// Turn creates a new file that flips Extra via init.
	writeWorkspaceFile(t, ws, "extra.go", "package verifyprobe\n\nfunc init() { Extra = \"broken\" }\n")
	preWrite := map[string]string{"extra.go": ""}

	head := verifyTests(context.Background(), ws, []string{"."})
	if head.Outcome != VerifyFailed {
		t.Fatalf("head should fail on TestExtra, got Outcome=%v output=%q", head.Outcome, head.Output)
	}

	got := attributeTestFailures(context.Background(), ws, []string{"."}, preWrite, head)
	if got.Outcome != VerifyFailed {
		t.Fatalf("turn-created failure should stay failed, got Outcome=%v output=%q", got.Outcome, got.Output)
	}
	if len(got.PreExistingFailures) != 0 {
		t.Errorf("PreExistingFailures = %v; want empty (baseline passes without the new file)", got.PreExistingFailures)
	}
}

// TestAttributeTestFailures_BuildFailureUnchanged covers case (d): a head
// with "[build failed]" and no named test failures is always the turn's and
// must be returned unchanged.
func TestAttributeTestFailures_BuildFailureUnchanged(t *testing.T) {
	head := TestVerification{
		Ran:     true,
		OK:      false,
		Outcome: VerifyFailed,
		Output:  "package verifyprobe\ncalc.go:3: undefined: Foo\n[build failed]",
		Command: []string{"go", "test", "."},
	}
	preWrite := map[string]string{"calc.go": "package verifyprobe\n"}

	got := attributeTestFailures(context.Background(), t.TempDir(), []string{"."}, preWrite, head)
	if got.Outcome != VerifyFailed {
		t.Errorf("build failure should stay failed, got Outcome=%v", got.Outcome)
	}
	if got.Output != head.Output {
		t.Errorf("build failure output should be unchanged, got %q want %q", got.Output, head.Output)
	}
	if len(got.PreExistingFailures) != 0 {
		t.Errorf("PreExistingFailures = %v; want empty for a build failure", got.PreExistingFailures)
	}
}

// TestAttributeTestFailures_MissingSnapshotSkipsAttribution covers the F-VERIFY-1
// guard: when any written path lacks a pre-write snapshot the baseline is
// untrustworthy, so the head must be returned unchanged.
func TestAttributeTestFailures_MissingSnapshotSkipsAttribution(t *testing.T) {
	head := TestVerification{
		Ran:     true,
		OK:      false,
		Outcome: VerifyFailed,
		Output:  "=== RUN   TestBroken\n--- FAIL: TestBroken (0.00s)\nFAIL\n",
		Command: []string{"go", "test", "."},
	}
	preWrite := map[string]string{"calc.go": "package verifyprobe\n"}
	writtenPaths := []string{"calc.go", "extra.go"}

	got := attributeTestFailures(context.Background(), t.TempDir(), []string{"."}, writtenPaths, preWrite, head)
	if got.Outcome != VerifyFailed {
		t.Errorf("missing snapshot should stay failed, got Outcome=%v", got.Outcome)
	}
	if len(got.PreExistingFailures) != 0 {
		t.Errorf("PreExistingFailures = %v; want empty when attribution is skipped", got.PreExistingFailures)
	}
	if got.Output != head.Output {
		t.Errorf("output should be unchanged when attribution is skipped, got %q want %q", got.Output, head.Output)
	}
}
