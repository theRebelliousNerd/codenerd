package xaioauth

import (
	"encoding/json"
	"reflect"
	"testing"

	"codenerd/internal/types"
)

// The SuperGrok OAuth surface is Chat Completions, so it carries exactly what
// that format can carry: tool ids and their pairing, but not the order between
// prose and calls, and nothing at all for reasoning. These tests assert both
// halves — what survives and what does not — because an adapter that quietly
// dropped a signature would otherwise look identical to one that never saw it.

func interleavedHistory() []types.Message {
	return []types.Message{
		types.NewUserMessage(types.TextBlock("why does LoadConfig ignore the flag?")),
		types.NewAssistantMessage(
			types.ThinkingBlock("Check the config first, then the caller.", "sig-abc-123"),
			types.TextBlock("I'll check two things."),
			types.ToolUseBlock("toolu_A", "read_file", map[string]any{"path": "config.go"}),
			types.TextBlock("Now the caller."),
			types.ToolUseBlock("toolu_B", "search_code", map[string]any{"query": "LoadConfig"}),
		),
		types.NewUserMessage(
			types.ToolResultBlock("toolu_A", "package config", false),
			types.ToolResultBlock("toolu_B", "no matches", true),
		),
	}
}

func TestMapHistoryKeepsIDsAndPairingButCollapsesOrder(t *testing.T) {
	msgs, err := mapHistoryToChatMessages("sys", interleavedHistory())
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(msgs) != 5 {
		t.Fatalf("produced %d messages: %#v", len(msgs), msgs)
	}

	assistant := msgs[2]
	if len(assistant.ToolCalls) != 2 {
		t.Fatalf("assistant carries %d tool calls, want 2", len(assistant.ToolCalls))
	}
	if assistant.ToolCalls[0].ID != "toolu_A" || assistant.ToolCalls[1].ID != "toolu_B" {
		t.Errorf("tool ids lost: %#v", assistant.ToolCalls)
	}
	var args map[string]any
	if err := json.Unmarshal([]byte(assistant.ToolCalls[0].Function.Arguments), &args); err != nil {
		t.Fatalf("tool arguments are not valid JSON: %v", err)
	}
	if args["path"] != "config.go" {
		t.Errorf("tool arguments lost: %#v", args)
	}

	// The interleaving collapses: one content string per assistant message.
	if assistant.Content != "I'll check two things.Now the caller." {
		t.Errorf("collapsed content = %q", assistant.Content)
	}

	// And reasoning is gone, not smuggled into the prose.
	for _, m := range msgs {
		if m.Content == "Check the config first, then the caller." {
			t.Fatal("reasoning leaked into a content string")
		}
	}

	if msgs[3].ToolCallID != "toolu_A" || msgs[4].ToolCallID != "toolu_B" {
		t.Errorf("tool result pairing lost: %q, %q", msgs[3].ToolCallID, msgs[4].ToolCallID)
	}
	if msgs[4].Content != "ERROR: no matches" {
		t.Errorf("error flag lost: %q", msgs[4].Content)
	}
}

func TestMapHistoryLegacyMessagesKeepTheirExactWireShape(t *testing.T) {
	history := []types.Message{
		{Role: "user", Text: "hi"},
		{Role: "assistant", Text: "sure", ToolCalls: []types.ToolCall{{ID: "t1", Name: "ls", Input: map[string]any{"p": "."}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "t1", Content: "a.go"}}},
		{Role: "", Text: "thanks"},
	}
	msgs, err := mapHistoryToChatMessages("", history)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	want := []chatMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "sure", ToolCalls: []toolCall{{
			ID: "t1", Type: "function",
			Function: toolFunction{Name: "ls", Arguments: `{"p":"."}`},
		}}},
		{Role: "tool", Content: "a.go", ToolCallID: "t1"},
		{Role: "user", Content: "thanks"},
	}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("legacy history changed shape:\n got %#v\nwant %#v", msgs, want)
	}
}
