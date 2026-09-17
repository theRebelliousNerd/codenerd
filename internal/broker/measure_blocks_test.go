package broker

import (
	"encoding/json"
	"strings"
	"testing"

	"codenerd/internal/types"
)

// A turn that replays reasoning is billed for the reasoning. Measuring only
// Text/ToolCalls/ToolResults would report such a request materially smaller
// than the one that goes on the wire, and the calibrator would learn a ratio
// from a size that was never sent.

func TestMeasureCountsReplayedThinkingBlocks(t *testing.T) {
	reasoning := strings.Repeat("r", 400)
	signature := strings.Repeat("s", 200)

	withThinking := &Request{
		Messages: []types.Message{
			types.NewAssistantMessage(
				types.ThinkingBlock(reasoning, signature),
				types.TextBlock("done"),
			),
		},
	}
	withoutThinking := &Request{
		Messages: []types.Message{
			types.NewAssistantMessage(types.TextBlock("done")),
		},
	}

	got := measure(withThinking).History - measure(withoutThinking).History
	if want := len(reasoning) + len(signature); got < want {
		t.Fatalf("thinking block contributed %d chars, want at least %d — a replayed block is being billed and not counted", got, want)
	}
}

func TestMeasureIsUnchangedForALegacyMessage(t *testing.T) {
	// The flat form must measure exactly as it always did, or every existing
	// calibration sample is invalidated by this change.
	req := &Request{
		Messages: []types.Message{
			{Role: "assistant", Text: "sure",
				ToolCalls: []types.ToolCall{{ID: "t1", Name: "ls", Input: map[string]any{"p": "."}}}},
			{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "t1", Content: "a.go"}}},
		},
	}

	want := 0
	for _, m := range req.Messages {
		want += perMessageOverheadChars + len(m.Role) + len(m.Text)
		for _, c := range m.ToolCalls {
			want += perToolCallOverheadChars + len(c.ID) + len(c.Name) + jsonChars(c.Input)
		}
		for _, r := range m.ToolResults {
			want += perToolResultOverheadChar + len(r.ToolUseID) + len(r.Content)
		}
	}
	if got := measure(req).History; got != want {
		t.Fatalf("legacy history measured %d, want %d", got, want)
	}
}

func TestAnthropicCountRendersThinkingBlocksInTheirWireShape(t *testing.T) {
	msg := types.NewAssistantMessage(
		types.ThinkingBlock("reasoning", "sig-1"),
		types.TextBlock("answer"),
		types.ToolUseBlock("toolu_A", "ls", map[string]any{"p": "."}),
	)

	out, ok := convertCountMessage(&msg)
	if !ok {
		t.Fatal("a turn with content was reported as having nothing countable")
	}
	blocks, ok := out.Content.([]map[string]any)
	if !ok {
		t.Fatalf("content is %T, want a block array", out.Content)
	}
	if len(blocks) != 3 {
		t.Fatalf("rendered %d blocks, want 3: %#v", len(blocks), blocks)
	}
	if blocks[0]["type"] != "thinking" || blocks[0]["signature"] != "sig-1" {
		t.Fatalf("thinking block not rendered for counting: %#v", blocks[0])
	}
	// It must also be valid JSON in the shape the endpoint parses.
	if _, err := json.Marshal(out); err != nil {
		t.Fatalf("count message does not marshal: %v", err)
	}
}

func TestAnthropicCountKeepsTheBareStringFormForPlainText(t *testing.T) {
	msg := types.Message{Role: "user", Text: "hi"}
	out, ok := convertCountMessage(&msg)
	if !ok {
		t.Fatal("plain text turn reported as empty")
	}
	if out.Content != "hi" {
		t.Fatalf("content = %#v, want the bare string form", out.Content)
	}
}

func TestAnthropicCountSkipsAnEmptyTurn(t *testing.T) {
	msg := types.Message{Role: "user"}
	if _, ok := convertCountMessage(&msg); ok {
		t.Fatal("an empty turn became a countable message; the endpoint rejects an empty content array")
	}
}
