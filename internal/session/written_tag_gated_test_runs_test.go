package session

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A test file the turn wrote behind a build tag is run under that tag before
// the gate may say green. Observed 2026-09-18: a turn asked for one test wrote
// it under //go:build integration, the gate ran the package with the default
// tags (which exclude the file), the turn was recorded /done, and the test
// failed the first time anyone ran it with the tag.
func TestGate_RunsTheTagGatedTestTheTurnWrote(t *testing.T) {
	workspace := t.TempDir()
	rel := filepath.Join("pkg", "lane_test.go")
	if err := os.MkdirAll(filepath.Join(workspace, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := "//go:build integration\n\npackage pkg\n\nimport \"testing\"\n\nfunc TestLaneIsTaken(t *testing.T) { t.Fatal(\"never ran\") }\n"
	if err := os.WriteFile(filepath.Join(workspace, rel), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}

	var taggedRun []string
	oldRunner, oldLook := verifyTestRunner, verifyLookPath
	verifyLookPath = func(string) (string, error) { return "go", nil }
	verifyTestRunner = func(_ context.Context, _ string, _ []string, _ string, args []string) ([]byte, error) {
		joined := strings.Join(args, " ")
		if strings.Contains(joined, "-tags integration") {
			taggedRun = args
			return []byte("--- FAIL: TestLaneIsTaken\nFAIL\tpkg\n"), errors.New("exit status 1")
		}
		return []byte("ok\tpkg\n"), nil // the default tags never compile the file
	}
	t.Cleanup(func() { verifyTestRunner, verifyLookPath = oldRunner, oldLook })

	green := TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	got := withWrittenTagGatedTests(context.Background(), workspace, []string{rel}, green)

	if taggedRun == nil {
		t.Fatal("the gate never ran the written test under its own build tag")
	}
	if !strings.Contains(strings.Join(taggedRun, " "), "^(TestLaneIsTaken)$") {
		t.Errorf("the tagged run was not aimed at the written test: %v", taggedRun)
	}
	if got.Verdict() != VerifyFailed {
		t.Errorf("gate verdict = %v, want failed: the test the turn wrote fails under the tag that compiles it", got.Verdict())
	}

	// An untagged written test needs no second run, and a non-green verdict is left alone.
	taggedRun = nil
	plain := filepath.Join("pkg", "plain_test.go")
	if err := os.WriteFile(filepath.Join(workspace, plain), []byte("package pkg\n\nimport \"testing\"\n\nfunc TestPlain(t *testing.T) {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if out := withWrittenTagGatedTests(context.Background(), workspace, []string{plain}, green); out.Verdict() != VerifyPassed || taggedRun != nil {
		t.Errorf("an untagged test file triggered a tagged run or lost its green: %v / %v", out.Verdict(), taggedRun)
	}
	red := TestVerification{Ran: true, Outcome: VerifyFailed, Output: "already red"}
	if out := withWrittenTagGatedTests(context.Background(), workspace, []string{rel}, red); out.Output != "already red" || taggedRun != nil {
		t.Errorf("a verdict that was not green was replaced or re-run")
	}
}
