package session

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeTaggedTest(t *testing.T, header string) (string, string) {
	t.Helper()
	ws := t.TempDir()
	rel := filepath.Join("pkg", "choice_test.go")
	if err := os.MkdirAll(filepath.Join(ws, "pkg"), 0o755); err != nil {
		t.Fatal(err)
	}
	src := header + "\n\npackage pkg\n\nimport \"testing\"\n\nfunc TestChoice(t *testing.T) { t.Fatal(\"ran\") }\n"
	if err := os.WriteFile(filepath.Join(ws, rel), []byte(src), 0o644); err != nil {
		t.Fatal(err)
	}
	return ws, rel
}

// An exclusive-or constraint is satisfied by one of its tags, not both:
// enabling every positive tag excluded the file, and the targeted run then
// matched nothing.
func TestSatisfyingTags_AnExclusiveOrTakesOneTag(t *testing.T) {
	t.Parallel()
	ws, rel := writeTaggedTest(t, "//go:build (feature_a && !feature_b) || (!feature_a && feature_b)")
	tags, ok, why := satisfyingTags(filepath.Join(ws, rel))
	if !ok || !slices.Equal(tags, []string{"feature_a"}) {
		t.Fatalf("satisfyingTags = %v, %v (%s); want [feature_a]", tags, ok, why)
	}
	ws, rel = writeTaggedTest(t, "//go:build integration && !race")
	if tags, ok, _ := satisfyingTags(filepath.Join(ws, rel)); !ok || !slices.Equal(tags, []string{"integration"}) {
		t.Fatalf("integration && !race = %v, %v", tags, ok)
	}
	ws, rel = writeTaggedTest(t, "//go:build feature_a && !feature_a")
	if _, ok, why := satisfyingTags(filepath.Join(ws, rel)); ok || why == "" {
		t.Fatal("an unsatisfiable constraint was given a configuration")
	}
}

// The tagged run is aimed at a configuration that includes the file, and a
// clean exit that ran none of the written tests is not a pass.
func TestGate_ATaggedRunThatRanNoTestIsNotAPass(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	ws, rel := writeTaggedTest(t, "//go:build (feature_a && !feature_b) || (!feature_a && feature_b)")
	streamDir := jsonTestModule(t, map[string]string{
		"lane_test.go": "package verifyprobe\n\nimport \"testing\"\n\nfunc TestChoice(t *testing.T) {}\n",
	})
	noTests := runJSONTest(t, streamDir, "-run", "^$")
	var taggedRun []string
	oldRunner, oldLook := verifyTestRunner, verifyLookPath
	verifyLookPath = func(string) (string, error) { return "go", nil }
	verifyTestRunner = func(_ context.Context, _ string, _ []string, _ string, args []string) ([]byte, error) {
		if strings.Contains(strings.Join(args, " "), "-tags") {
			taggedRun = args
		}
		return []byte(noTests), nil // matched nothing, exited 0
	}
	t.Cleanup(func() { verifyTestRunner, verifyLookPath = oldRunner, oldLook })

	green := TestVerification{Ran: true, OK: true, Outcome: VerifyPassed}
	got := withWrittenTagGatedTests(context.Background(), ws, []string{rel}, green)
	joined := strings.Join(taggedRun, " ")
	if !strings.Contains(joined, "-tags feature_a ") {
		t.Fatalf("the tagged run did not use a configuration that includes the file: %v", taggedRun)
	}
	if got.Verdict() == VerifyPassed {
		t.Fatalf("a clean exit that ran no written test passed the gate: %+v", got)
	}
	if !strings.Contains(got.Reason, "TestChoice") {
		t.Fatalf("the verdict does not name the test that never ran: %q", got.Reason)
	}
}
