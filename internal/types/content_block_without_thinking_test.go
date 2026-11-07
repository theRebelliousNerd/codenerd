package types

import "testing"

// A turn whose replayed reasoning is dropped keeps its prose and its calls, in
// order; a turn that is nothing but reasoning, or that has none, is unchanged.
func TestWithoutThinkingKeepsEverythingButTheReasoning(t *testing.T) {
	m := NewAssistantMessage(
		RedactedThinkingBlock("sig-1"),
		TextBlock("checking"),
		ThinkingBlock("why", "sig-2"),
		ToolUseBlock("c1", "read_file", map[string]any{"path": "a.go"}),
	)
	got, n := m.WithoutThinking()
	if n != 2 {
		t.Fatalf("dropped %d reasoning blocks, want 2", n)
	}
	content := got.Content()
	if len(content) != 2 || content[0].Kind != BlockText || content[0].Text != "checking" ||
		content[1].Kind != BlockToolUse || content[1].ID != "c1" {
		t.Fatalf("content = %+v, want the text then the call", content)
	}
	if got.Text != "checking" || len(got.ToolCalls) != 1 || got.ToolCalls[0].ID != "c1" {
		t.Fatalf("flat projection = %q %+v, want it to match the blocks", got.Text, got.ToolCalls)
	}
	if len(m.Content()) != 4 {
		t.Fatal("the original message was modified")
	}

	onlyReasoning := NewAssistantMessage(RedactedThinkingBlock("sig"))
	if same, n := onlyReasoning.WithoutThinking(); n != 0 || len(same.Content()) != 1 {
		t.Fatalf("a turn of nothing but reasoning must keep it; dropped %d", n)
	}
	flat := Message{Role: "assistant", Text: "x", ToolCalls: []ToolCall{{ID: "c"}}}
	if _, n := flat.WithoutThinking(); n != 0 {
		t.Fatalf("a flat message has no reasoning to drop; dropped %d", n)
	}
	plain := NewAssistantMessage(TextBlock("x"))
	if _, n := plain.WithoutThinking(); n != 0 {
		t.Fatalf("a turn with no reasoning dropped %d", n)
	}
}
