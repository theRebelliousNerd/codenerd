package perception

import (
	"strings"
	"testing"

	"codenerd/internal/prompt"
)

// understandingAtomContent is the classification prompt's owning atom as the
// binary ships it. The atom is the only source: the embedded Go copy was
// deleted 2026-10-28.
func understandingAtomContent(t *testing.T) string {
	t.Helper()
	corpus, err := prompt.LoadEmbeddedCorpus()
	if err != nil {
		t.Fatalf("LoadEmbeddedCorpus: %v", err)
	}
	atom, ok := corpus.Get("perception_understanding")
	if !ok {
		t.Fatal("the perception_understanding atom is missing from the embedded corpus")
	}
	return atom.Content
}

// F-PERC-1: "Add table-driven tests to internal/tactile/python/..." classified
// as verify (-> /run_tests) rather than a write verb, three times on
// 2026-08-08. The turn ran the suite, wrote nothing, and reported success.
//
// The classification prompt's own table said verify meant "Test, validate,
// check correctness | Maybe (writes tests)". That "Maybe" is the ambiguity: the
// word "tests" dominates while the imperative carries the intent.
//
// This pins the disambiguation in the atom, since the classification itself
// is a model decision that cannot be asserted deterministically.
func TestUnderstandingPromptDisambiguatesWritingFromRunningTests(t *testing.T) {
	p := understandingAtomContent(t)

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
	for _, line := range strings.Split(understandingAtomContent(t), "\n") {
		if strings.HasPrefix(strings.TrimSpace(line), "| `verify` ") {
			if !strings.HasSuffix(strings.TrimSpace(line), "| No |") {
				t.Errorf("verify row does not declare itself non-mutating: %q", strings.TrimSpace(line))
			}
			return
		}
	}
	t.Fatal("verify row not found in the classification table")
}

// Every action type the adapter maps to a verb has a row the model can pick.
func TestUnderstandingPromptListsEveryMappedActionType(t *testing.T) {
	p := understandingAtomContent(t)
	for _, action := range []string{
		"investigate", "implement", "modify", "refactor", "verify", "explain",
		"research", "configure", "attack", "revert", "review", "remember",
		"forget", "chat", "approve", "diagnose_self", "migrate", "optimize",
		"document", "benchmark", "profile", "audit", "scaffold", "lint", "format",
	} {
		if !strings.Contains(p, "| `"+action+"` |") {
			t.Errorf("the action table has no %q row", action)
		}
	}
}
