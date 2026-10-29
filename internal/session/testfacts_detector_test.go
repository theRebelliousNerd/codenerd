package session

import (
	"strings"
	"testing"
)

// A model that quotes the gate's Summary is presenting test output. The
// Summary has no "--- FAIL:" line; the detector still has to see it. Classic
// runner text stays matched too (models paste that as well).
func TestResponsePresentsGateSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	dir := jsonTestModule(t, failingModule())
	out := verificationOutput(parseJSONTest(t, dir, runJSONTest(t, dir, ".")))
	if !responsePresentsTestRunnerOutput(out) {
		t.Fatalf("the gate Summary was not recognised as test output:\n%s", out)
	}
	if !strings.Contains(out, "TestSub/case_one") {
		t.Fatalf("the fixture Summary lost the sanitized subtest:\n%s", out)
	}
	for _, prose := range []string{
		"FAIL the build if this is wrong",
		"the packages failed to compile",
		"repeated attempts are not a test result",
	} {
		if responsePresentsTestRunnerOutput(prose) {
			t.Errorf("prose %q was read as test output", prose)
		}
	}
}

func TestResponsePresentsBuildFailureSummary(t *testing.T) {
	if testing.Short() {
		t.Skip("shells out to the real go toolchain")
	}
	dir := jsonTestModule(t, map[string]string{
		"calc.go":        "package verifyprobe\n\nfunc Add(a, b int) int { return a + b }\n",
		"broken_test.go": "package verifyprobe\n\nimport \"testing\"\n\nfunc TestBroken(t *testing.T) { neverWritten() }\n",
	})
	out := verificationOutput(parseJSONTest(t, dir, runJSONTest(t, dir, ".")))
	if !responsePresentsTestRunnerOutput(out) {
		t.Fatalf("a build-failure Summary was not recognised as test output:\n%s", out)
	}
}
