package session

import (
	"strings"
	"testing"

	working "codenerd/internal/context"
	"codenerd/internal/types"
)

// The meter counts and nothing else. Everything this file used to assert
// about granting extra rounds went with the extensions on 2026-09-18: a
// count of rounds was never evidence that a task was or was not finished.

func read(id, path, content string) ([]types.ToolCall, []types.ToolResult) {
	return []types.ToolCall{{ID: id, Name: "read_file", Input: map[string]any{"path": path}}},
		[]types.ToolResult{{ToolUseID: id, Content: content}}
}

func TestWorkingMeter_CountsWhatThePolicyReads(t *testing.T) {
	meter := newWorkingMeter(2)

	meter.observe(read("r1", "a.go", "package a"))
	meter.observe(
		[]types.ToolCall{{ID: "w1", Name: "write_file", Input: map[string]any{"path": "a.go", "content": "x"}}},
		[]types.ToolResult{{ToolUseID: "w1", Content: "ok"}},
	)
	meter.observe(read("r2", "b.go", "package b"))

	got := meter.workingProgress(true, 0)
	if got.Rounds != 3 {
		t.Errorf("Rounds = %d, want 3", got.Rounds)
	}
	if got.Writes != 1 {
		t.Errorf("Writes = %d, want 1", got.Writes)
	}
	if got.SinceWrite != 1 {
		t.Errorf("SinceWrite = %d, want 1 (the read after the write)", got.SinceWrite)
	}
	if got.SinceVerify != 3 {
		t.Errorf("SinceVerify = %d, want 3: nothing verified anything", got.SinceVerify)
	}
	if !got.WriteIntent {
		t.Error("WriteIntent was dropped on the way to the policy")
	}

	meter.observe(
		[]types.ToolCall{{ID: "t1", Name: "run_command", Input: map[string]any{"command": "go test ./internal/session -count=1"}}},
		[]types.ToolResult{{ToolUseID: "t1", Content: "ok"}},
	)
	if got := meter.workingProgress(true, 0); got.SinceVerify != 0 {
		t.Errorf("SinceVerify = %d after a focused verification, want 0", got.SinceVerify)
	}
}

// A failed call is neither a write nor a verification, so neither counter is
// reset by one; the policy sees a change task that has not written.
func TestWorkingMeter_FailedCallIsNotProgress(t *testing.T) {
	meter := newWorkingMeter(2)
	meter.observe(
		[]types.ToolCall{{ID: "w1", Name: "write_file", Input: map[string]any{"path": "a.go", "content": "x"}}},
		[]types.ToolResult{{ToolUseID: "w1", Content: "permission denied", IsError: true}},
	)
	if got := meter.workingProgress(true, 1); got.Writes != 0 || got.SinceWrite != 1 {
		t.Fatalf("progress = %+v, want no write counted for a failed write", got)
	}
}

// The span that makes a repeat a cycle is the policy's
// working_repeat_threshold, handed in by the loop. Comparing the whole event
// signature means a re-read whose returned bytes changed is progress, while
// identical read/read churn is a loop.
func TestWorkingMeter_RepeatedTailCycleUsesThePolicySpan(t *testing.T) {
	t.Run("identical rounds reach the span", func(t *testing.T) {
		meter := newWorkingMeter(2)
		meter.observe(read("first", "same.go", "unchanged"))
		if meter.repeatedTailCycle() {
			t.Fatal("one round cannot be a cycle")
		}
		meter.observe(read("second", "same.go", "unchanged"))
		if !meter.repeatedTailCycle() {
			t.Fatal("two identical rounds at threshold 2 is a cycle")
		}
	})

	t.Run("changed evidence is progress", func(t *testing.T) {
		meter := newWorkingMeter(2)
		meter.observe(read("before", "same.go", "before"))
		meter.observe(read("after", "same.go", "after"))
		if meter.repeatedTailCycle() {
			t.Fatal("a re-read that returned different bytes is not a cycle")
		}
	})

	t.Run("a wider span needs more repeats", func(t *testing.T) {
		meter := newWorkingMeter(3)
		meter.observe(read("first", "same.go", "unchanged"))
		meter.observe(read("second", "same.go", "unchanged"))
		if meter.repeatedTailCycle() {
			t.Fatal("two repeats must not be a cycle at threshold 3")
		}
		meter.observe(read("third", "same.go", "unchanged"))
		if !meter.repeatedTailCycle() {
			t.Fatal("three repeats at threshold 3 is a cycle")
		}
	})

	t.Run("a span below two claims nothing", func(t *testing.T) {
		meter := newWorkingMeter(1)
		meter.observe(read("first", "same.go", "unchanged"))
		meter.observe(read("second", "same.go", "unchanged"))
		if meter.repeatedTailCycle() {
			t.Fatal("a span of 1 would call every round a cycle; it must claim none")
		}
	})

	t.Run("a period-2 alternation is a cycle", func(t *testing.T) {
		meter := newWorkingMeter(2)
		for i := 0; i < 2; i++ {
			meter.observe(read("a", "a.go", "A"))
			meter.observe(read("b", "b.go", "B"))
		}
		if !meter.repeatedTailCycle() {
			t.Fatal("A,B,A,B at threshold 2 is a period-2 cycle")
		}
	})
}

func TestWorkingMeter_NilMeasuresNothingWithoutPanic(t *testing.T) {
	var meter *workingMeter
	meter.observe(read("r1", "a.go", "package a"))
	if meter.repeatedTailCycle() {
		t.Fatal("a nil meter must claim no cycle")
	}
	if got := meter.workingProgress(true, 2); got.Rounds != 0 || !got.WriteIntent || got.FailedRounds != 2 {
		t.Fatalf("progress = %+v, want the caller's own inputs and no counts", got)
	}
}

func TestFocusedVerificationCall_RejectsChainedShell(t *testing.T) {
	call := types.ToolCall{Name: "run_command", Input: map[string]any{"command": "go test ./...; remove-everything"}}
	if isFocusedVerificationCall(call) {
		t.Fatal("chained shell command must not count as focused verification")
	}
}

// A stop the user sees names the rule that fired and the facts that satisfied
// it. Until 2026-09-18 the same place said "tool call budget exceeded", which
// told the user about the harness's arithmetic rather than about their task.
func TestDescribeWorkingStop_NamesTheDerivation(t *testing.T) {
	cases := []struct {
		reason   string
		progress working.WorkingProgress
		want     []string
	}{
		{
			reason:   "read_only_stall",
			progress: working.WorkingProgress{WriteIntent: true, Rounds: 24},
			want:     []string{"working_stop(/read_only_stall)", "working_progress(/write, 24, 0, _, _)", "working_stall_rounds"},
		},
		{
			reason:   "tool_failures",
			progress: working.WorkingProgress{FailedRounds: 3},
			want:     []string{"working_stop(/tool_failures)", "working_control(_, 3)"},
		},
		{
			reason:   "repeated_cycle",
			progress: working.WorkingProgress{Cycle: true},
			want:     []string{"working_stop(/repeated_cycle)", "working_control(/yes,", "working_repeat_threshold"},
		},
	}
	for _, tc := range cases {
		t.Run(tc.reason, func(t *testing.T) {
			got := describeWorkingStop(tc.reason, tc.progress, 7)
			for _, want := range tc.want {
				if !strings.Contains(got, want) {
					t.Errorf("message %q does not contain %q", got, want)
				}
			}
			if !strings.Contains(got, "task unresolved") {
				t.Errorf("message %q must say the task is unresolved, not that it is done", got)
			}
			if !strings.Contains(got, "7 tool call(s) executed") {
				t.Errorf("message %q must report what was executed", got)
			}
			for _, banned := range budgetVocabulary {
				if strings.Contains(got, banned) {
					t.Errorf("message %q still uses budget vocabulary %q", got, banned)
				}
			}
		})
	}
}

// !Continue with no reason is a policy bug — a rule that withholds
// continuation without deriving a working_stop — and must read as one.
func TestDescribeWorkingStop_UnnamedStopIsAPolicyBug(t *testing.T) {
	got := describeWorkingStop("", working.WorkingProgress{Rounds: 4}, 4)
	if !strings.Contains(got, "without deriving a working_stop reason") {
		t.Fatalf("message = %q, want it to name the missing derivation", got)
	}
}

// Steering reports what the turn has done. A count of what is left is not a
// fact about the task, and a model told how many calls remain optimizes for
// the count.
func TestWorkingNudgeText_ReportsWhatTheTurnDidNotWhatIsLeft(t *testing.T) {
	p := working.WorkingProgress{WriteIntent: true, Rounds: 8, SinceWrite: 5}
	for _, kind := range []string{"/implement", "/verify", "/conclude"} {
		text := workingNudgeText(kind, p)
		if strings.TrimSpace(text) == "" {
			t.Fatalf("%s produced no steering text", kind)
		}
		for _, banned := range budgetVocabulary {
			if strings.Contains(text, banned) {
				t.Errorf("%s nudge %q still uses budget vocabulary %q", kind, text, banned)
			}
		}
	}
	if workingNudgeText("/nothing-policy-derives", p) != "" {
		t.Error("an unknown steering kind must produce no text rather than invent some")
	}
}
