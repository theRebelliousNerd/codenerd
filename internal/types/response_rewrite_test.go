package types

import "testing"

// LLMToolResponse is the one place in this design where the flat fields and the
// block list are both settable, so it is the one place they can be given
// contradictory values. Message avoided that by hiding its block list behind
// constructors; a response cannot, because adapters build it field by field as
// they parse. These pin the invariant that every post-parse edit keeps.

func TestRewriteReplacesBothViewsAndKeepsReasoningInFront(t *testing.T) {
	resp := &LLMToolResponse{
		Text: `{"surface":"ignore me","control":{}}`,
		Blocks: []ContentBlock{
			ThinkingBlock("weigh the two options", "sig-keep-me"),
			TextBlock(`{"surface":"ignore me","control":{}}`),
		},
	}

	calls := []ToolCall{{ID: "t1", Name: "read_file", Input: map[string]any{"path": "a.go"}}}
	resp.Rewrite("checking a.go", calls)

	if resp.Text != "checking a.go" {
		t.Errorf("flat Text = %q", resp.Text)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].ID != "t1" {
		t.Errorf("flat ToolCalls = %#v", resp.ToolCalls)
	}

	want := []ContentBlockKind{BlockThinking, BlockText, BlockToolUse}
	if len(resp.Blocks) != len(want) {
		t.Fatalf("blocks = %#v, want %v", resp.Blocks, want)
	}
	for i, kind := range want {
		if resp.Blocks[i].Kind != kind {
			t.Errorf("block %d is %s, want %s", i, resp.Blocks[i].Kind, kind)
		}
	}
	// The signature is the whole reason thinking survives a rewrite: providers
	// require it replayed verbatim, and regenerating it is not a thing a client
	// can do.
	if resp.Blocks[0].Signature != "sig-keep-me" {
		t.Errorf("the rewrite dropped the thinking signature: %q", resp.Blocks[0].Signature)
	}
	if resp.Blocks[1].Text != "checking a.go" {
		t.Errorf("text block = %q", resp.Blocks[1].Text)
	}
	if resp.Blocks[2].ID != "t1" || resp.Blocks[2].Name != "read_file" {
		t.Errorf("tool_use block = %#v", resp.Blocks[2])
	}

	// The envelope must be gone from BOTH views. Leaving it in the blocks is
	// how the raw control packet gets replayed to the model as its own prose.
	for _, b := range resp.Blocks {
		if b.Kind == BlockText && b.Text != "checking a.go" {
			t.Errorf("a stale text block survived the rewrite: %q", b.Text)
		}
	}
}

// A response the provider gave no ordering for must not be given one here.
// Nil blocks mean "this provider did not tell us the order", and inventing one
// turns an honest absence into a claim the adapters will act on.
func TestRewriteDoesNotInventBlocksForAProviderThatSentNone(t *testing.T) {
	resp := &LLMToolResponse{Text: "plain"}
	resp.Rewrite("still plain", []ToolCall{{ID: "t1", Name: "read_file"}})

	if resp.Blocks != nil {
		t.Errorf("rewrite invented %d block(s) for a response that had none", len(resp.Blocks))
	}
	if resp.Text != "still plain" || len(resp.ToolCalls) != 1 {
		t.Errorf("flat fields not updated: %q / %#v", resp.Text, resp.ToolCalls)
	}
}

// `resp.ToolCalls = nil` is the tempting form and it is half a change: the
// tool_use blocks stay, so a caller reading the blocks still sees the calls the
// assignment was written to retract — and AssistantMessageFrom prefers blocks.
func TestClearToolCallsRetractsTheCallsFromBothViews(t *testing.T) {
	resp := &LLMToolResponse{
		Text: "done",
		Blocks: []ContentBlock{
			TextBlock("done"),
			ToolUseBlock("t1", "write_file", nil),
		},
		ToolCalls: []ToolCall{{ID: "t1", Name: "write_file"}},
	}

	resp.ClearToolCalls()

	if len(resp.ToolCalls) != 0 {
		t.Errorf("flat ToolCalls survived: %#v", resp.ToolCalls)
	}
	for _, b := range resp.Blocks {
		if b.Kind == BlockToolUse {
			t.Fatalf("a tool_use block survived ClearToolCalls: %#v", b)
		}
	}
	if resp.Text != "done" {
		t.Errorf("clearing the calls took the prose with it: %q", resp.Text)
	}
	// AssistantMessageFrom is the consumer this exists for.
	if calls := AssistantMessageFrom(resp).ToolCalls; len(calls) != 0 {
		t.Errorf("the message built from the cleared response still offers %#v", calls)
	}
}

// Responses are copied by value in the session package — `copy := *pending`
// then clear — so the clear must not reach through the copy into the
// original's backing array.
func TestClearToolCallsOnACopyLeavesTheOriginalIntact(t *testing.T) {
	original := &LLMToolResponse{
		Text:      "working",
		Blocks:    []ContentBlock{TextBlock("working"), ToolUseBlock("t1", "read_file", nil)},
		ToolCalls: []ToolCall{{ID: "t1", Name: "read_file"}},
	}

	duplicate := *original
	duplicate.ClearToolCalls()

	if len(original.ToolCalls) != 1 {
		t.Errorf("clearing the copy emptied the original's flat calls")
	}
	found := false
	for _, b := range original.Blocks {
		if b.Kind == BlockToolUse {
			found = true
		}
	}
	if !found {
		t.Error("clearing the copy reached into the original's blocks; the pending " +
			"response the caller still holds no longer offers what it is pending on")
	}
}
