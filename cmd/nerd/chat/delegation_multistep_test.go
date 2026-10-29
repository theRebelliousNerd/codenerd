package chat

import (
	"strings"
	"testing"

	"codenerd/internal/perception"
)

// "add a test" must not become the target "a": the decomposition of a fix and
// its test carries what perception read (the target, the verb, the user's
// limits), not the regex capture between the verb and "test".
func TestDecomposeTask_VerbPairUsesTheIntentNotTheCapture(t *testing.T) {
	input := "OK the plan: divide by the real count, and return 0 for an empty slice instead of panicking. Only touch stats.go and add a test for the empty case in small_test.go."
	intent := perception.Intent{
		Verb: "/fix", Category: "/mutation", Target: "Mean in stats.go", Confidence: 0.95,
		Constraint: "divide by the real count; return 0 for an empty slice; only touch stats.go",
	}
	steps := decomposeTask(input, intent, t.TempDir())
	if len(steps) != 2 {
		t.Fatalf("got %d steps, want a fix and its test: %+v", len(steps), steps)
	}
	if steps[0].Verb != "/fix" || steps[1].Verb != "/test" {
		t.Errorf("verbs = %s, %s; want /fix, /test", steps[0].Verb, steps[1].Verb)
	}
	for i, s := range steps {
		if s.Target != "Mean in stats.go" {
			t.Errorf("step %d target = %q, want the intent's target", i, s.Target)
		}
		if strings.Contains(s.Task, " a ") || strings.HasSuffix(s.Task, " a") {
			t.Errorf("step %d task carries the regex capture: %q", i, s.Task)
		}
	}
	if !strings.Contains(steps[0].Task, "only touch stats.go") {
		t.Errorf("the first step lost the user's limits: %q", steps[0].Task)
	}
}
