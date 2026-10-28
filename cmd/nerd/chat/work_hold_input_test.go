package chat

import (
	"strings"
	"testing"

	"codenerd/internal/perception"
)

// "/go" typed into the box is spent once read: it must not sit in the input
// while the held work runs (it did, for the whole run).
func TestApprovePendingWork_ClearsTheTypedGo(t *testing.T) {
	m := NewTestModel()
	m.textarea.SetValue("/go")
	m, _ = m.approvePendingWork()
	if got := m.textarea.Value(); got != "" {
		t.Fatalf("input still holds %q after /go", got)
	}
}

// A read-only "run the tests" must not become a write_tests task in the
// background lane: that would create test files with no hold, since a query is
// never held by the work mode.
func TestBackgroundTask_RunningTestsIsNotWritingThem(t *testing.T) {
	m := NewTestModel()
	got := m.backgroundTask(perception.Intent{Verb: "/test", Category: "/query", Target: "WordCount", Constraint: "tell me what fails"})
	if strings.Contains(got, "write_tests") || !strings.HasPrefix(got, "run_tests") {
		t.Fatalf("task = %q, want run_tests", got)
	}
}
