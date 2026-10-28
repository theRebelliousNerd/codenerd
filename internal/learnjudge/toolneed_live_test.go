//go:build jevlive

package learnjudge

import (
	"context"
	"fmt"
	"testing"
)

// Requests and failed attempts, labeled with whether the kernel should derive
// a capability gap (policy/jev_learning.mg tool_gap_observed), beside what the
// old trigger did (any failed attempt, or a capability regex).
var toolNeedCases = []struct {
	name, request, failed string
	wantGap, oldGap       bool
}{
	{"explicit-tool", "build me a tool that converts Avro files to CSV", "", true, false},
	{"explicit-regex", "I need a tool to validate the JSON schema of every config file", "", true, true},
	{"avro-failed", "summarize the records in data/events.avro", "the agent tried read_file on data/events.avro and got binary it could not decode; no tool parses Avro", true, true},
	{"timeout-failed", "fix the nil pointer in internal/session/executor.go", "LLM call failed: context deadline exceeded", false, true},
	{"test-failed", "make Max return 0 for an empty slice", "go test ./internal/stats: --- FAIL: TestMaxEmpty: index out of range", false, true},
	{"bad-arg-failed", "find every caller of Executor.Run", "search_code failed: invalid regex 'Executor.Run(' missing closing )", false, true},
	{"big-task", "rename Executor to Runner across the whole codebase and update the docs", "", false, false},
	{"question", "how does the JIT compiler pick atoms?", "", false, false},
	{"no-tool-for-question", "is there no tool for listing goroutines? how do I see them?", "", false, true},
	{"pdf-failed", "extract the tables from docs/spec.pdf into markdown", "read_file returned binary PDF content; nothing in the harness extracts PDF text", true, true},
}

func TestLiveToolNeed(t *testing.T) {
	c, _ := liveClient(t, "tool_need")
	yes := func(p float64) bool { return p >= 0.7 }
	right, oldRight := 0, 0
	var lines []string
	for _, tc := range toolNeedCases {
		j, err := JudgeToolNeed(probeLabel(context.Background(), tc.name), c, ToolNeedInput{Request: tc.request, FailedAttempt: tc.failed})
		if err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		gap := yes(j.AsksForTool) || (j.AskedAboutFailure && yes(j.FailedForMissingTool) && yes(j.NeedsMissingOp))
		mark := "ok"
		if gap == tc.wantGap {
			right++
		} else {
			mark = "MISS"
		}
		if tc.oldGap == tc.wantGap {
			oldRight++
		}
		lines = append(lines, fmt.Sprintf("%-4s %-20s gap=%-5v want=%-5v old=%-5v asks=%.2f missing_op=%.2f failed_for_missing=%.2f kind=%s",
			mark, tc.name, gap, tc.wantGap, tc.oldGap, j.AsksForTool, j.NeedsMissingOp, j.FailedForMissingTool, j.Kind))
	}
	lines = append([]string{fmt.Sprintf("tool-need probe: kernel table on Jev %d/%d right; old trigger (any failure or regex) %d/%d right", right, len(toolNeedCases), oldRight, len(toolNeedCases))}, lines...)
	probeSummary(t, "tool_need", lines)
}
