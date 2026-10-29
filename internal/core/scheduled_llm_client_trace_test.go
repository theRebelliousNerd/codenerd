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

// A tool-loop request replays the model's earlier reasoning in position, and on
// the Meta Responses surface that reasoning is an encrypted signature: ~3% of
// every request. The trace renders it, in wire order, so a logged request can
// be reconstructed; it used to be dropped.
func TestTraceMessagesRendersReplayedReasoningInPosition(t *testing.T) {
	signature := strings.Repeat("Q", 4096)
	history := []types.Message{
		{Role: "user", Text: "fix it"},
		types.NewMessage("assistant",
			types.RedactedThinkingBlock(signature),
			types.TextBlock("reading the file"),
			types.ToolUseBlock("c1", "read_file", map[string]any{"path": "a.go"})),
	}
	got := traceMessages(history)
	body := got[1].Content
	reasoning := strings.Index(body, "[reasoning id= redacted=true tokens=0 signature_chars=4096]")
	text := strings.Index(body, "reading the file")
	call := strings.Index(body, "[tool_use id=c1 name=read_file]")
	if reasoning < 0 || text < 0 || call < 0 {
		t.Fatalf("assistant turn rendered as %q", body)
	}
	if !(reasoning < text && text < call) {
		t.Errorf("blocks out of wire order: reasoning@%d text@%d call@%d", reasoning, text, call)
	}
	if !strings.Contains(body, "[signature] "+signature) {
		t.Error("the replayed signature is not in the trace whole")
	}
}

// The tool schemas a request offers are in the trace whole, with a catalog id
// that is equal exactly when the catalogs are.
func TestTraceToolCatalogCarriesTheSchemas(t *testing.T) {
	coder := []types.ToolDefinition{{Name: "read_file", Description: "read a file", InputSchema: map[string]any{"type": "object"}}}
	reviewer := []types.ToolDefinition{{Name: "git_diff", Description: "show the diff", InputSchema: map[string]any{"type": "object"}}}
	a, b, c := traceToolCatalog(coder), traceToolCatalog(coder), traceToolCatalog(reviewer)
	if !strings.Contains(a, `"description":"read a file"`) || !strings.Contains(a, `"input_schema":{"type":"object"}`) {
		t.Fatalf("catalog trace lacks the schema: %q", a)
	}
	id := func(s string) string { return strings.SplitN(strings.SplitN(s, "catalog=", 2)[1], "]", 2)[0] }
	if id(a) != id(b) || id(a) == id(c) {
		t.Errorf("catalog ids: %s %s %s", id(a), id(b), id(c))
	}
}
