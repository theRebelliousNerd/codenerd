package session

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// roundsOfHistory builds a native tool-loop history of n rounds after the
// user's input: one assistant call and one user result per round.
func roundsOfHistory(n int) []types.Message {
	history := []types.Message{{Role: "user", Text: "fix target.go"}}
	for round := 1; round <= n; round++ {
		id := fmt.Sprintf("call-%d", round)
		history = append(history,
			types.Message{Role: "assistant", ToolCalls: []types.ToolCall{{ID: id, Name: "read_file"}}},
			types.Message{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: id, Content: "result " + id}}})
	}
	return history
}

// A provider prefix cache covers a request up to its first changed byte. A
// window that drops its oldest round every round moves the first message after
// the anchor every round, so the transcript was never served from cache
// (measured 2026-09-21: the prefix changed on 767 of 854 follow-up calls). With
// slack the window's first round holds still between cuts.
func TestTranscriptStart_MovesOncePerSlackCycle(t *testing.T) {
	const rounds, slack = 3, 3
	// history index of round r's assistant message
	at := func(r int) int { return 1 + 2*(r-1) }

	want := map[int]int{ // total rounds -> first kept round
		1: 1, 2: 1, 3: 1, // fewer than the window: everything is kept
		4: 1, 5: 1, 6: 1, // grows by appending: 4, 5, 6 rounds kept
		7: 5, // the cut: back to 3 rounds (5, 6, 7)
		8: 5, 9: 5, 10: 5,
		11: 9,
	}
	for total, first := range want {
		if got := transcriptStart(roundsOfHistory(total), rounds, slack); got != at(first) {
			t.Errorf("%d rounds: window starts at history[%d], want round %d at history[%d]", total, got, first, at(first))
		}
	}

	// Zero slack is the old every-round slide, still a valid policy.
	for total := 3; total <= 6; total++ {
		if got, first := transcriptStart(roundsOfHistory(total), rounds, 0), total-rounds+1; got != at(first) {
			t.Errorf("slack 0, %d rounds: starts at history[%d], want round %d", total, got, first)
		}
	}
	// No round yet: nothing of history is transcript (the anchor carries the input).
	if got := transcriptStart(roundsOfHistory(0), rounds, slack); got != 1 {
		t.Errorf("no rounds: start = %d, want len(history) = 1", got)
	}
}

// The request is ordered stable to volatile. The system prompt reaches the
// provider exactly as compiled, and the working section -- which follows the
// model's attention and changes on most rounds -- is the last thing in the
// request, after the newest tool result.
func TestPrepareWorkingRequest_PutsTheVolatileSectionLast(t *testing.T) {
	e, ctx, history, _ := oneWorkingRound(t, 200000)
	// An earlier read that is no longer in the transcript: the kind of
	// observation the section exists to carry.
	earlier := types.ToolCall{ID: "call-0", Name: "read_file", Input: map[string]any{"path": "target.go", "start_line": 400}}
	if err := e.recordWorkingResult(ctx, earlier, "needle-line-437 seen on an earlier round\n", nil); err != nil {
		t.Fatalf("recordWorkingResult: %v", err)
	}

	parts, section, err := e.workingRequestParts(ctx, "SYSTEM", history, nil)
	if err != nil {
		t.Fatalf("workingRequestParts: %v", err)
	}
	if section == "" {
		t.Fatal("no working section was selected, so its placement was not exercised")
	}
	messages, err := e.prepareWorkingRequest(ctx, "SYSTEM", history, nil)
	if err != nil {
		t.Fatalf("prepareWorkingRequest: %v", err)
	}
	if len(messages) != len(parts) {
		t.Fatalf("the section added a message: %d -> %d; two user turns in a row is not accepted by every provider", len(parts), len(messages))
	}

	last := messages[len(messages)-1]
	if last.Role != "user" || len(last.ToolResults) == 0 {
		t.Fatalf("the last turn is not the newest tool result: %+v", last)
	}
	blocks := last.Content()
	tail := blocks[len(blocks)-1]
	if tail.Kind != types.BlockText || !strings.HasPrefix(tail.Text, workingSectionHeader) || !strings.Contains(tail.Text, section) {
		t.Errorf("the section is not the last content of the last turn; tail block = %q", truncateForFailure(tail.Text))
	}
	// Everything before the last turn is byte-identical with and without the
	// section: that is the cacheable prefix.
	for i := range messages[:len(messages)-1] {
		if fmt.Sprintf("%+v", messages[i]) != fmt.Sprintf("%+v", parts[i]) {
			t.Errorf("message %d changed when the section was attached", i)
		}
	}
	// The caller's history is not written through.
	if history[len(history)-1].Text != "" {
		t.Error("attaching the section wrote into the caller's history")
	}
}

func TestMessageWithTrailingText_BothViews(t *testing.T) {
	flat := types.Message{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "a", Content: "out"}}}
	built := types.NewUserMessage(types.ToolResultBlock("a", "out", false))

	for name, m := range map[string]types.Message{"flat": flat, "block-built": built} {
		got := m.WithTrailingText("SECTION").Content()
		if len(got) != 2 || got[0].Kind != types.BlockToolResult || got[1].Kind != types.BlockText || got[1].Text != "SECTION" {
			t.Errorf("%s: content = %+v, want the tool result then the text", name, got)
		}
		if len(m.Content()) != 1 {
			t.Errorf("%s: the original message was modified", name)
		}
	}
	if got := (types.Message{Role: "user", Text: "anchor"}).WithTrailingText("SECTION").Text; got != "anchor\n\nSECTION" {
		t.Errorf("text turn = %q", got)
	}
}
