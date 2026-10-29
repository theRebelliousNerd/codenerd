package types

import (
	"reflect"
	"testing"
)

// interleavedTurn is the shape the whole representation exists for: prose, a
// tool call, more prose, a second tool call, with a signed thinking block in
// front. Nothing about that turn is expressible in Text + ToolCalls.
func interleavedTurn() []ContentBlock {
	return []ContentBlock{
		ThinkingBlock("Two things to check: the config, then the caller.", "sig-abc-123"),
		TextBlock("I'll check two things."),
		ToolUseBlock("toolu_A", "read_file", map[string]any{"path": "config.go"}),
		TextBlock("Now the caller."),
		ToolUseBlock("toolu_B", "search_code", map[string]any{"query": "LoadConfig"}),
	}
}

func TestContentReturnsBlocksInTheOrderTheyWereGiven(t *testing.T) {
	want := interleavedTurn()
	got := NewAssistantMessage(want...).Content()

	if !reflect.DeepEqual(got, want) {
		t.Fatalf("blocks did not survive construction:\n got %#v\nwant %#v", got, want)
	}
}

func TestFlatFieldsAreAProjectionOfTheBlocks(t *testing.T) {
	m := NewAssistantMessage(interleavedTurn()...)

	// Text is the text blocks, concatenated in order. The thinking block's
	// text is deliberately absent: it is not part of the visible answer.
	if want := "I'll check two things.Now the caller."; m.Text != want {
		t.Errorf("Text projection = %q, want %q", m.Text, want)
	}
	if len(m.ToolCalls) != 2 {
		t.Fatalf("ToolCalls projection has %d calls, want 2", len(m.ToolCalls))
	}
	if m.ToolCalls[0].ID != "toolu_A" || m.ToolCalls[1].ID != "toolu_B" {
		t.Errorf("tool ids not projected in order: %q, %q", m.ToolCalls[0].ID, m.ToolCalls[1].ID)
	}
	if len(m.ToolResults) != 0 {
		t.Errorf("assistant turn projected %d tool results, want 0", len(m.ToolResults))
	}
}

func TestReasoningNeverLeaksIntoTheFlatText(t *testing.T) {
	m := NewAssistantMessage(
		ThinkingBlock("the user is probably wrong about this", "sig-1"),
		TextBlock("Sure, here you go."),
	)
	if m.Text != "Sure, here you go." {
		t.Fatalf("thinking text leaked into the flat projection: %q", m.Text)
	}
	// It is still there in the blocks, with its signature intact.
	blocks := m.Content()
	if blocks[0].Kind != BlockThinking || blocks[0].Signature != "sig-1" {
		t.Fatalf("thinking block lost from the content: %#v", blocks[0])
	}
}

func TestLegacyMessageLiftsToTheOrderTheAdaptersAlreadyEmitted(t *testing.T) {
	// A struct literal built the old way. Content must lift it to
	// tool_result, then text, then tool_use — the exact order the Anthropic,
	// OpenAI and Meta adapters emitted by hand before blocks existed, so that
	// no legacy turn changes shape on any wire.
	m := Message{
		Role:        "user",
		Text:        "here are the results",
		ToolCalls:   []ToolCall{{ID: "t1", Name: "x"}},
		ToolResults: []ToolResult{{ToolUseID: "t0", Content: "out"}},
	}

	got := m.Content()
	wantKinds := []ContentBlockKind{BlockToolResult, BlockText, BlockToolUse}
	if len(got) != len(wantKinds) {
		t.Fatalf("lifted %d blocks, want %d: %#v", len(got), len(wantKinds), got)
	}
	for i, kind := range wantKinds {
		if got[i].Kind != kind {
			t.Errorf("block %d is %q, want %q", i, got[i].Kind, kind)
		}
	}
	if m.HasNativeBlocks() {
		t.Error("a literal-built message claims native blocks; adapters would trust an order that was never recorded")
	}
}

func TestEmptyLegacyMessageLiftsToNothing(t *testing.T) {
	// An empty text must not become an empty text block: an empty content
	// array is rejected by Anthropic, and the adapters rely on len()==0 to
	// choose the bare-string form.
	if got := (Message{Role: "user"}).Content(); len(got) != 0 {
		t.Fatalf("empty message lifted to %d blocks: %#v", len(got), got)
	}
}

func TestBlocksAreTheOneSourceOfTruth(t *testing.T) {
	// The flat fields cannot be set independently of the blocks — the field is
	// unexported, so the only way to get blocks is through a constructor that
	// fills both. This pins the rule that makes that worth relying on: what
	// Content returns is the block list, never a re-derivation from the flat
	// fields.
	m := NewAssistantMessage(interleavedTurn()...)
	before := m.Content()

	// Mutating the projection is what internal/session's history eviction
	// does. It must not silently rewrite the content.
	m.Text = "tampered"
	m.ToolCalls = nil

	if !reflect.DeepEqual(m.Content(), before) {
		t.Fatal("Content changed when the flat projection was mutated: the two views disagree, which is the failure mode this design exists to prevent")
	}
}

func TestAssistantMessageFromCarriesBlocksWhenTheProviderReportedThem(t *testing.T) {
	blocks := interleavedTurn()
	resp := &LLMToolResponse{
		Text:      "I'll check two things.Now the caller.",
		ToolCalls: []ToolCall{{ID: "toolu_A"}, {ID: "toolu_B"}},
		Blocks:    blocks,
	}

	m := AssistantMessageFrom(resp)
	if !m.HasNativeBlocks() {
		t.Fatal("AssistantMessageFrom dropped the response's ordered blocks")
	}
	if !reflect.DeepEqual(m.Content(), blocks) {
		t.Fatalf("blocks not carried verbatim:\n got %#v\nwant %#v", m.Content(), blocks)
	}
}

func TestAssistantMessageFromFallsBackWhenTheProviderReportedNoOrder(t *testing.T) {
	// A nil Blocks means "this surface does not report order", not "empty
	// turn". The flat fields must still make it into the history.
	resp := &LLMToolResponse{
		Text:      "done",
		ToolCalls: []ToolCall{{ID: "call_0", Name: "read_file"}},
	}
	m := AssistantMessageFrom(resp)

	if m.HasNativeBlocks() {
		t.Error("fallback claimed native blocks it never received")
	}
	got := m.Content()
	if len(got) != 2 || got[0].Kind != BlockText || got[1].ID != "call_0" {
		t.Fatalf("fallback lost the turn: %#v", got)
	}
}

func TestAssistantMessageFromNilResponse(t *testing.T) {
	m := AssistantMessageFrom(nil)
	if m.Role != "assistant" || len(m.Content()) != 0 {
		t.Fatalf("nil response produced %#v", m)
	}
}

func TestToolResultBlockRoundTripsThroughItsProjection(t *testing.T) {
	b := ToolResultBlock("toolu_A", "boom", true)
	got := b.ToolResult()
	want := ToolResult{ToolUseID: "toolu_A", Content: "boom", IsError: true}
	if got != want {
		t.Fatalf("projection = %#v, want %#v", got, want)
	}
}

func TestRedactedThinkingKeepsOnlyItsSignature(t *testing.T) {
	b := RedactedThinkingBlock("enc-blob")
	if !b.Redacted || b.Signature != "enc-blob" || b.Text != "" {
		t.Fatalf("redacted thinking block malformed: %#v", b)
	}
}
