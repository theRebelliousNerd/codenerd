package chat

import (
	"strings"
	"testing"
)

// TestUpdate_ContinuationDoneOutcome feeds each continuationDoneMsg outcome
// variant through Update and asserts the rendered banner matches the outcome.
func TestUpdate_ContinuationDoneOutcome(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name       string
		msg        continuationDoneMsg
		wantPrefix string
	}{
		{
			name:       "completed",
			msg:        continuationDoneMsg{stepCount: 3, summary: "Completed 3 step(s); the requested behavior was verified.", outcome: continuationCompleted},
			wantPrefix: "✅",
		},
		{
			// The zero value is continuationUnverified, not completion. A
			// producer that states no outcome has observed none, and the
			// checkmark is reserved for a verdict.
			name:       "zeroValueRendersUnverified",
			msg:        continuationDoneMsg{stepCount: 3, summary: "Ran 3 step(s); the work is not verified."},
			wantPrefix: "⚠️",
		},
		{
			name:       "unverified",
			msg:        continuationDoneMsg{stepCount: 1, summary: "Ran 1 step(s); the work is not verified.", outcome: continuationUnverified},
			wantPrefix: "⚠️",
		},
		{
			name:       "failed",
			msg:        continuationDoneMsg{stepCount: 3, summary: "Step 3 failed: context deadline exceeded", outcome: continuationFailed},
			wantPrefix: "❌",
		},
		{
			name:       "interrupted",
			msg:        continuationDoneMsg{stepCount: 2, summary: "Stopped by user (Ctrl+X)", outcome: continuationInterrupted},
			wantPrefix: "⏹️",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			m := NewTestModel()

			updated, _ := m.Update(tc.msg)
			result, ok := updated.(Model)
			if !ok {
				t.Fatalf("Update did not return a Model")
			}

			if len(result.history) == 0 {
				t.Fatalf("expected an assistant message, got empty history")
			}
			last := result.history[len(result.history)-1]
			if last.Role != "assistant" {
				t.Errorf("expected last message role assistant, got %q", last.Role)
			}
			if !strings.HasPrefix(last.Content, tc.wantPrefix) {
				t.Errorf("expected last message to start with %q, got %q", tc.wantPrefix, last.Content)
			}
			if tc.msg.outcome != continuationCompleted && strings.Contains(last.Content, "✅") {
				t.Errorf("a non-completed outcome must never contain ✅, got %q", last.Content)
			}
			if len(result.pendingSubtasks) != 0 {
				t.Errorf("expected pendingSubtasks to be cleared, got %d", len(result.pendingSubtasks))
			}
			if result.isInterrupted {
				t.Errorf("expected isInterrupted to be cleared")
			}
		})
	}
}
