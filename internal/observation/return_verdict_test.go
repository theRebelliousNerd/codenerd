package observation

import (
	"strings"
	"testing"
)

// S1 gave observation.Return a typed Outcome — the producer's kernel verdict
// for the turn — precisely so that no consumer downstream would have to read a
// status out of prose or out of "it did not return an error".
//
// ProjectReturn does not read it. It computes Status from two things only: is
// Failure non-empty, and is Output non-empty. Everything else is "completed".
// That is the claim S1 deleted from cmd/nerd/chat's injectShardResultFacts,
// alive one layer down, on the surface the model actually reads: the delegate
// action (internal/core/virtual_store_actions.go handleDelegate) hands
// ReturnResult.Text to the parent as the tool result AND asserts it into the
// kernel as delegation_result/2.
//
// So a coder subagent that wrote three files, could not show a green test gate
// and was recorded /unverified by the very kernel read S4 made binding is
// announced to the parent as "coder returned completed".
//
// The rule these tests pin: a verdict the producer recorded outranks the
// absence of an error. "completed" is reserved for a producer that said so.
func TestProjectReturnStatusAgreesWithTurnOutcome(t *testing.T) {
	limits := DefaultReturnLimits()

	cases := []struct {
		name    string
		outcome string
		// notCompleted is the whole assertion: whatever word the projection
		// chooses for these, it must not be the one that means "this is done".
		notCompleted bool
	}{
		{
			name:         "unverifiedIsNotCompleted",
			outcome:      "/unverified",
			notCompleted: true,
		},
		{
			name:         "hollowIsNotCompleted",
			outcome:      "/hollow",
			notCompleted: true,
		},
		{
			name: "failedIsNotCompleted",
			// A /failed turn whose error never became prose: the executor
			// records the verdict on the result, and observedReturn only fills
			// Failure from res.Error. A turn failed by the kernel's
			// turn_build_failed arm has an outcome and no error string.
			outcome:      "/failed",
			notCompleted: true,
		},
		{
			name:         "doneIsCompleted",
			outcome:      "/done",
			notCompleted: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			r := Return{
				Agent:   "coder",
				Task:    "add the retry",
				Output:  "I added the retry to internal/broker/compression.go and explained it.",
				Outcome: tc.outcome,
				Stage:   "artifact_changed",
				Changed: []string{"internal/broker/compression.go"},
			}

			got := ProjectReturn(r, limits)
			text := got.Text("subagent_expand")

			if tc.notCompleted {
				if got.Status == StatusCompleted {
					t.Errorf("a %s turn is projected Status=%q; the producer's verdict must outrank \"no error\"",
						tc.outcome, got.Status)
				}
				if strings.Contains(text, "returned "+StatusCompleted) {
					t.Errorf("the parent is told %q for a %s turn; the text the model reads must not claim completion",
						verdictHeadline(text), tc.outcome)
				}
				// The verdict must survive to the reader in some form, or the
				// parent has no way to ask for the work to be finished.
				if !strings.Contains(text, tc.outcome) && got.Status == StatusCompleted {
					t.Errorf("the %s verdict reaches the parent nowhere in:\n%s", tc.outcome, text)
				}
				return
			}
			if got.Status != StatusCompleted {
				t.Errorf("a %s turn is projected Status=%q, want %q", tc.outcome, got.Status, StatusCompleted)
			}
		})
	}
}

// The converse, so the fix cannot be "call everything unverified": a producer
// that recorded no verdict at all is in exactly the position ProjectReturn was
// always in — it knows only that nothing errored — and the existing behaviour
// there is the honest one. This case must keep passing.
func TestProjectReturn_AbsentVerdictKeepsTheOldReading(t *testing.T) {
	r := Return{
		Agent:  "researcher",
		Task:   "find the producer of final_system_prompt",
		Output: "Nothing in the repository produces it.",
	}
	got := ProjectReturn(r, DefaultReturnLimits())
	if got.Status != StatusCompleted {
		t.Fatalf("Status = %q, want %q: a producer with no verdict is not newly demoted", got.Status, StatusCompleted)
	}
}

// A failure still outranks a verdict: a turn that errored is failed whatever
// atom the kernel got to.
func TestProjectReturn_FailureStillOutranksTheVerdict(t *testing.T) {
	r := Return{
		Agent:   "coder",
		Task:    "apply the patch",
		Output:  "partial",
		Outcome: "/done",
		Failure: "tool execution failed: apply_edits: no such file",
	}
	got := ProjectReturn(r, DefaultReturnLimits())
	if got.Status != StatusFailed {
		t.Fatalf("Status = %q, want %q", got.Status, StatusFailed)
	}
}

// verdictHeadline is the first line of a projection — the "<agent> returned
// <status>" header the parent reads before anything else.
func verdictHeadline(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}
