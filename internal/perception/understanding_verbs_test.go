package perception

import (
	"strings"
	"testing"
)

// F-PERC-1: "Add table-driven tests to internal/tactile/python/..." classified
// as verify (-> /run_tests) rather than a write verb, three times on
// 2026-08-08. The turn ran the suite, wrote nothing, and reported success.
//
// The classification prompt's own table said verify meant "Test, validate,
// check correctness | Maybe (writes tests)". That "Maybe" is the ambiguity: the
// word "tests" dominates while the imperative carries the intent.
//
// This pins the disambiguation in the prompt, since the classification itself
// is a model decision that cannot be asserted deterministically.
func TestUnderstandingPromptDisambiguatesWritingFromRunningTests(t *testing.T) {
	p := understandingSystemPrompt

	if strings.Contains(p, "Maybe (writes tests)") {
		t.Error("the verify row still claims it may write tests, which is what sent write requests to /run_tests")
	}
	if !strings.Contains(p, "Writing a test is implement, not verify") {
		t.Error("prompt does not state that authoring a test is a write verb")
	}
	for _, want := range []string{
		"Test code is code",
		"RUNNING what already exists",
		"silently dropped",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("prompt is missing the disambiguation clause %q", want)
		}
	}
}

// verify must be unambiguously non-mutating, or the table gives the classifier
// permission to route a write there again.
func TestUnderstandingPromptVerifyRowIsNonMutating(t *testing.T) {
	for _, line := range strings.Split(understandingSystemPrompt, "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "| verify ") {
			if !strings.HasSuffix(strings.TrimSpace(line), "| No |") {
				t.Errorf("verify row does not declare itself non-mutating: %q", strings.TrimSpace(line))
			}
			return
		}
	}
	t.Fatal("verify row not found in the classification table")
}
