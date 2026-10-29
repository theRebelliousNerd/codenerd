package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

// countTestLaunches wraps the test runner and counts the go test processes it
// starts. It swaps a package seam, so its callers do not run in parallel.
func countTestLaunches(t *testing.T) *int {
	t.Helper()
	launches := 0
	real := verifyTestRunner
	verifyTestRunner = func(ctx context.Context, dir string, env []string, name string, args []string) ([]byte, error) {
		if len(args) > 0 && args[0] == "test" {
			launches++
		}
		return real(ctx, dir, env, name, args)
	}
	t.Cleanup(func() { verifyTestRunner = real })
	return &launches
}

// Two gate measurements with no write between them are one measurement: the
// second round reads the first's verdicts and launches no go test. A write
// between them is a new revision, measured again. (Ladder R1-24 measured the
// importer gate twice, 2.5 minutes each, on a tree no one had touched.)
func TestGateTests_AnUnchangedTreeIsMeasuredOnce(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws := importerModule(t, aAfter)
	result := mutationResult()
	result.WrittenPaths = []string{"a/a.go"}
	result.PreWriteContents = map[string]PreImage{"a/a.go": existed(aBefore)}
	launches := countTestLaunches(t)

	first, _ := gateTests(context.Background(), ws, result, true)
	importers := result.ImporterCheck
	// Own packages, importers, and the pre-turn baseline that attributes
	// the importer failure.
	measured := *launches
	if measured < 2 {
		t.Fatalf("the first gate launched %d go test run(s), want at least 2 (own packages, importers)", measured)
	}
	if first.Verdict() != VerifyPassed || importers.Verdict() != VerifyFailed {
		t.Fatalf("own %s, importers %s: want passed and failed (b pins what a returns)", first.Verdict(), importers.Verdict())
	}

	// The closure asks again, with and without coverage, on the same writes.
	again, _ := gateTests(context.Background(), ws, result, true)
	recheck, _ := gateOwnTests(context.Background(), ws, result, false)
	if *launches != measured {
		t.Fatalf("re-measuring an unchanged tree launched %d more go test run(s), want 0", *launches-measured)
	}
	if again.Verdict() != first.Verdict() || recheck.Verdict() != first.Verdict() || result.ImporterCheck.Verdict() != importers.Verdict() {
		t.Fatalf("a recalled measurement changed its verdicts: own %s/%s, importers %s", again.Verdict(), recheck.Verdict(), result.ImporterCheck.Verdict())
	}

	// A write -- back to what b expects -- is a new revision.
	if err := os.WriteFile(filepath.Join(ws, "a", "a.go"), []byte(aBefore), 0o600); err != nil {
		t.Fatal(err)
	}
	gateTests(context.Background(), ws, result, true)
	if got := *launches - measured; got != 2 {
		t.Fatalf("after a write the gate launched %d go test run(s), want 2 (own packages, importers)", got)
	}
	if result.ImporterCheck.Verdict() != VerifyPassed {
		t.Fatalf("importers after the write = %s, want passed: the old failure was reused", result.ImporterCheck.Verdict())
	}
}
