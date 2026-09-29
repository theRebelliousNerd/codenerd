package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"codenerd/internal/testfacts"
)

// failedHead builds a failed head verification whose Result names one failed
// test: the skip-path tests below exercise their branch (canceled, missing
// snapshot, no snapshots), not the no-failures early return.
func failedHead() TestVerification {
	res := &testfacts.Result{
		Status: testfacts.StatusFail,
		Packages: []*testfacts.Package{{
			Name:   "verifyprobe",
			Status: testfacts.StatusFail,
			Tests: []*testfacts.Test{
				{Name: "TestBroken", Status: testfacts.StatusFail},
				{Name: "TestOK", Status: testfacts.StatusPass},
			},
		}},
	}
	return TestVerification{
		Ran:     true,
		OK:      false,
		Outcome: VerifyFailed,
		Output:  verificationOutput(res),
		Command: []string{"go", "test", "-json", "."},
		Result:  res,
	}
}

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
		"go.mod":       "module verifyprobe\n\ngo 1.21\n",
		"calc.go":      origCalc,
		"calc_test.go": testFile,
	})
	preWrite := map[string]PreImage{"calc.go": existed(origCalc)}

	// Turn edit: add an unrelated exported func, tests untouched.
	writeWorkspaceFile(t, ws, "calc.go", origCalc+"\nfunc Unrelated() int { return 42 }\n")

	head := verifyTests(context.Background(), ws, []string{"."})
	if head.Outcome != VerifyFailed {
		t.Fatalf("head should fail on TestAlwaysFails, got Outcome=%v OK=%v output=%q", head.Outcome, head.OK, head.Output)
	}

	got := attributeTestFailures(context.Background(), ws, []string{"."}, []string{"calc.go"}, preWrite, head)
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

// TestAttributeTestFailures_ExpiredDeadlineStillAttributes covers F-ATTR-DEADLINE:
// a passed deadline must not skip baseline attribution. Identical setup to
// TestAttributeTestFailures_AllPreExistingPasses, but attributeTestFailures runs
// with an already-expired deadline context.
func TestAttributeTestFailures_ExpiredDeadlineStillAttributes(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	origCalc := "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n"
	testFile := "package verifyprobe\n\nimport \"testing\"\n\n" +
		"func TestAlwaysFails(t *testing.T) { t.Fatal(\"always fails\") }\n" +
		"func TestOK(t *testing.T) { if Add(2, 3) != 5 { t.Fatal(\"bad\") } }\n"
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":       "module verifyprobe\n\ngo 1.21\n",
		"calc.go":      origCalc,
		"calc_test.go": testFile,
	})
	preWrite := map[string]PreImage{"calc.go": existed(origCalc)}

	// Turn edit: add an unrelated exported func, tests untouched.
	writeWorkspaceFile(t, ws, "calc.go", origCalc+"\nfunc Unrelated() int { return 42 }\n")

	head := verifyTests(context.Background(), ws, []string{"."})
	if head.Outcome != VerifyFailed {
		t.Fatalf("head should fail on TestAlwaysFails, got Outcome=%v OK=%v output=%q", head.Outcome, head.OK, head.Output)
	}

	expired, cancel := context.WithDeadline(context.Background(), time.Now().Add(-time.Second))
	defer cancel()
	got := attributeTestFailures(expired, ws, []string{"."}, []string{"calc.go"}, preWrite, head)
	if got.Outcome != VerifyPassed {
		t.Fatalf("expired-deadline attribution should pass, got Outcome=%v output=%q", got.Outcome, got.Output)
	}
	if len(got.PreExistingFailures) != 1 || got.PreExistingFailures[0] != "TestAlwaysFails" {
		t.Errorf("PreExistingFailures = %v; want [TestAlwaysFails]", got.PreExistingFailures)
	}
}

// TestAttributeTestFailures_CanceledSkips covers F-ATTR-DEADLINE: a canceled
// context skips baseline attribution and returns head unchanged.
func TestAttributeTestFailures_CanceledSkips(t *testing.T) {
	head := failedHead()
	preWrite := map[string]PreImage{"calc.go": existed("package verifyprobe\n")}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := attributeTestFailures(ctx, t.TempDir(), []string{"."}, []string{"calc.go"}, preWrite, head)
	if got.Outcome != VerifyFailed {
		t.Errorf("canceled attribution should stay failed, got Outcome=%v", got.Outcome)
	}
	if len(got.PreExistingFailures) != 0 {
		t.Errorf("PreExistingFailures = %v; want empty when attribution is skipped", got.PreExistingFailures)
	}
	if got.Output != head.Output {
		t.Errorf("output should be unchanged when attribution is skipped, got %q want %q", got.Output, head.Output)
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
		"go.mod":       "module verifyprobe\n\ngo 1.21\n",
		"calc.go":      origCalc,
		"calc_test.go": testFile,
	})
	preWrite := map[string]PreImage{"calc.go": existed(origCalc)}

	// Turn edit breaks Add, so TestOK is newly failing.
	writeWorkspaceFile(t, ws, "calc.go", "package verifyprobe\n\nfunc Add(a, b int) int { return a + b + 1 }\n")

	head := verifyTests(context.Background(), ws, []string{"."})
	if head.Outcome != VerifyFailed {
		t.Fatalf("head should fail, got Outcome=%v output=%q", head.Outcome, head.Output)
	}

	got := attributeTestFailures(context.Background(), ws, []string{"."}, []string{"calc.go"}, preWrite, head)
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

// A new subtest is charged even when its parent already failed. The name
// compared is the sanitized one testfacts reports ("old case" -> old_case).
func TestAttributeTestFailures_NewSubtestIsNotPreExisting(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	oldTest := "package verifyprobe\n\nimport \"testing\"\n\n" +
		"func TestAlways(t *testing.T) {\n" +
		"\tt.Run(\"old case\", func(t *testing.T) { t.Fatal(\"old fails\") })\n" +
		"}\n"
	newTest := "package verifyprobe\n\nimport \"testing\"\n\n" +
		"func TestAlways(t *testing.T) {\n" +
		"\tt.Run(\"old case\", func(t *testing.T) { t.Fatal(\"old fails\") })\n" +
		"\tt.Run(\"new case\", func(t *testing.T) { t.Fatal(\"new fails\") })\n" +
		"}\n"
	ws := writeBaselineModule(t, map[string]string{
		"go.mod":       "module verifyprobe\n\ngo 1.21\n",
		"calc.go":      "package verifyprobe\n",
		"calc_test.go": newTest,
	})
	preWrite := map[string]PreImage{"calc_test.go": existed(oldTest)}

	head := verifyTests(context.Background(), ws, []string{"."})
	if head.Outcome != VerifyFailed {
		t.Fatalf("head should fail, got Outcome=%v output=%q", head.Outcome, head.Output)
	}
	got := attributeTestFailures(context.Background(), ws, []string{"."}, []string{"calc_test.go"}, preWrite, head)
	if got.Outcome != VerifyFailed {
		t.Fatalf("a new subtest must stay charged, got Outcome=%v output=%q", got.Outcome, got.Output)
	}
	has := map[string]bool{}
	for _, n := range got.PreExistingFailures {
		has[n] = true
	}
	if !has["TestAlways/old_case"] {
		t.Errorf("PreExistingFailures = %v; want the sanitized subtest TestAlways/old_case", got.PreExistingFailures)
	}
	if has["TestAlways/new_case"] {
		t.Errorf("the new subtest was treated as pre-existing: %v", got.PreExistingFailures)
	}
	if !strings.Contains(got.Output, "new fails") {
		t.Errorf("output lost the new failure: %q", got.Output)
	}
}

// Partition compares full sanitized names: a sibling subtest that passed
// at baseline is not pre-existing, and the parent that failed in both is.
func TestPartitionPreExisting_SubtestSpelling(t *testing.T) {
	head := []string{"TestSub", "TestSub/case_one", "TestSub/case_two"}
	baseline := &testfacts.Result{
		Packages: []*testfacts.Package{{
			Name: "verifyprobe",
			Tests: []*testfacts.Test{
				{Name: "TestSub", Status: testfacts.StatusFail},
				{Name: "TestSub/case_one", Status: testfacts.StatusFail},
				{Name: "TestSub/case_two", Status: testfacts.StatusPass},
			},
		}},
	}
	got := partitionPreExisting(head, baseline)
	want := []string{"TestSub", "TestSub/case_one"}
	if len(got) != len(want) {
		t.Fatalf("partitionPreExisting = %v; want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("partitionPreExisting = %v; want %v", got, want)
		}
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
	preWrite := map[string]PreImage{"extra.go": {}}

	head := verifyTests(context.Background(), ws, []string{"."})
	if head.Outcome != VerifyFailed {
		t.Fatalf("head should fail on TestExtra, got Outcome=%v output=%q", head.Outcome, head.Output)
	}

	got := attributeTestFailures(context.Background(), ws, []string{"."}, []string{"extra.go"}, preWrite, head)
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
	res := &testfacts.Result{
		Status: testfacts.StatusBuildFailed,
		Packages: []*testfacts.Package{{
			Name:   "verifyprobe",
			Status: testfacts.StatusBuildFailed,
			Output: []string{"./calc.go:3:24: undefined: Foo"},
		}},
		BuildFailures: []testfacts.BuildFailure{{
			Package: "verifyprobe", File: "./calc.go", Line: 3, Column: 24, Message: "undefined: Foo",
		}},
	}
	head := TestVerification{
		Ran:     true,
		OK:      false,
		Outcome: VerifyFailed,
		Output:  verificationOutput(res),
		Command: []string{"go", "test", "-json", "."},
		Result:  res,
	}
	preWrite := map[string]PreImage{"calc.go": existed("package verifyprobe\n")}

	got := attributeTestFailures(context.Background(), t.TempDir(), []string{"."}, []string{"calc.go"}, preWrite, head)
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
	head := failedHead()
	preWrite := map[string]PreImage{"calc.go": existed("package verifyprobe\n")}
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

// TestAttributeTestFailures_NoSnapshotsLogsReason covers the F-VERIFY-1c
// silent branch: when the test gate does not discount pre-existing failures
// for lack of pre-write snapshots, the head must be returned unchanged.
func TestAttributeTestFailures_NoSnapshotsLogsReason(t *testing.T) {
	head := failedHead()

	got := attributeTestFailures(context.Background(), t.TempDir(), []string{"."}, nil, nil, head)
	if got.Outcome != head.Outcome {
		t.Errorf("no-snapshot attribution should return head unchanged, got Outcome=%v want %v", got.Outcome, head.Outcome)
	}
	if got.Output != head.Output {
		t.Errorf("no-snapshot attribution should return head unchanged, got output %q want %q", got.Output, head.Output)
	}
}

// TestRunBaselineTests_UsesGateRunnerAndArgs pins the F-VERIFY-1 seam: the
// baseline must run through the gate's verifyTestRunner with the gate's
// overlay/run argv, so a fake runner sees the exact gate-shaped invocation.
func TestRunBaselineTests_UsesGateRunnerAndArgs(t *testing.T) {
	if testing.Short() {
		t.Skip("captures a real go test -json stream for the fake runner")
	}
	// The baseline run's stdout is a real -json stream captured at test
	// time; the gate parses it rather than scanning text.
	stream := runJSONTest(t, jsonTestModule(t, map[string]string{
		"calc.go": "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n",
		"calc_test.go": "package verifyprobe\n\nimport \"testing\"\n\n" +
			"func TestX(t *testing.T) { t.Fatal(\"baseline fails\") }\n",
	}), ".")
	old := verifyTestRunner
	var gotName string
	var gotArgs []string
	verifyTestRunner = func(_ context.Context, _ string, _ []string, name string, args []string) ([]byte, error) {
		gotName = name
		gotArgs = append([]string(nil), args...)
		return []byte(stream), errors.New("exit status 1")
	}
	t.Cleanup(func() { verifyTestRunner = old })

	const overlayPath = "test-overlay.json"
	const runArg = "^(TestX)$"
	res, outcome := runBaselineTests(context.Background(), t.TempDir(), overlayPath, runArg, []string{"."})
	if outcome != VerifyFailed {
		t.Fatalf("runBaselineTests should report outcome=VerifyFailed for a failed baseline run, got %v", outcome)
	}
	if names := failedTopLevels(res); len(names) != 1 || names[0] != "TestX" {
		t.Errorf("baseline Result names %v; want [TestX]", names)
	}
	if gotName != "go" {
		t.Errorf("runner name = %q; want %q", gotName, "go")
	}
	joined := strings.Join(gotArgs, " ")
	if !strings.Contains(joined, "-json") {
		t.Errorf("runner args %q should run go test -json", gotArgs)
	}
	findFlag := func(flag string) (string, bool) {
		for i, a := range gotArgs {
			if a == flag && i+1 < len(gotArgs) {
				return gotArgs[i+1], true
			}
		}
		return "", false
	}
	if v, ok := findFlag("-overlay"); !ok || v != overlayPath {
		t.Errorf("runner args %q should contain -overlay %q", gotArgs, overlayPath)
	}
	if v, ok := findFlag("-run"); !ok || v != runArg {
		t.Errorf("runner args %q should contain -run %q", gotArgs, runArg)
	}
}
