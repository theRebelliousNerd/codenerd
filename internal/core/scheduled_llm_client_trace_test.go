package core

import (
	"strings"
	"testing"

	"codenerd/internal/types"
)

// The llm_io trace of a tool-loop round used to carry only
// "[TOOL_RESULTS history_turns=N tools=M]": a stalled run's transcript, and
// whether the orchestrator's steering at the end of a result ever reached
// the model, could not be read back. The rendered transcript carries the
// text, every call with its arguments, and every result whole.
func TestTraceMessagesRendersToolLoopTranscript(t *testing.T) {
	history := []types.Message{
		{Role: "user", Text: "fix it"},
		{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "c1", Name: "read_file", Input: map[string]any{"path": "a.go", "start_line": 10}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "c1", Content: strings.Repeat("line\n", 1000) + "[orchestrator] make the change now"}}},
	}
	got := traceMessages(history)
	if len(got) != 3 {
		t.Fatalf("rendered %d messages, want 3", len(got))
	}
	if got[0].Role != "user" || got[0].Content != "fix it" {
		t.Fatalf("text message rendered as %+v", got[0])
	}
	if !strings.Contains(got[1].Content, `[tool_use id=c1 name=read_file] {"path":"a.go","start_line":10}`) {
		t.Fatalf("tool call rendered as %q", got[1].Content)
	}
	if !strings.HasPrefix(got[2].Content, "[tool_result id=c1 error=false]\n") || !strings.HasSuffix(got[2].Content, "[orchestrator] make the change now") {
		t.Fatalf("tool result must be rendered whole, head: %q tail: %q", got[2].Content[:40], got[2].Content[len(got[2].Content)-40:])
	}
}
