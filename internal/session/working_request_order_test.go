package session

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The request is ordered stable to volatile. The system prompt reaches the
// provider exactly as compiled, and the harness's text for a round -- here the
// focus file's view -- rides the end of that round's last user turn, after its
// tool results, without adding a turn: two user turns in a row is not accepted
// by every provider. The caller's history is not written through.
func TestPrepareWorkingRequest_AppendsTheHarnessTextToTheRoundsLastTurn(t *testing.T) {
	e, ctx, history, _ := oneWorkingRound(t, 200000)
	e.SetFileContextProvider(&stubFileContext{section: "outline of target.go"})

	messages, err := e.prepareWorkingRequest(ctx, "SYSTEM", history, nil)
	if err != nil {
		t.Fatalf("prepareWorkingRequest: %v", err)
	}
	// prior (none) + anchor + the round's two turns
	if len(messages) != 3 {
		t.Fatalf("the request has %d turns, want the anchor and the round's two", len(messages))
	}
	last := messages[len(messages)-1]
	if last.Role != "user" || len(last.ToolResults) == 0 {
		t.Fatalf("the last turn is not the newest tool result: %+v", last)
	}
	blocks := last.Content()
	tail := blocks[len(blocks)-1]
	if tail.Kind != types.BlockText || !strings.HasPrefix(tail.Text, fmt.Sprintf(workingViewHeader, "target.go")) || !strings.Contains(tail.Text, "outline of target.go") {
		t.Errorf("the focus view is not the last content of the round's last turn; tail block = %q", truncateForFailure(tail.Text))
	}
	if history[len(history)-1].Text != "" {
		t.Error("appending the view wrote into the caller's history")
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
