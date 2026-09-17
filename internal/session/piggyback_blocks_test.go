package session

import (
	"testing"

	"codenerd/internal/types"
)

// Piggyback promotion rewrites a turn: an envelope in the prose becomes a
// surface string plus N tool calls. It used to write only the flat fields.
//
// That was harmless while nothing read the blocks, and stopped being harmless
// the moment the tool loop started appending assistant turns with
// AssistantMessageFrom — which PREFERS blocks. A block-carrying response would
// then have gone into history holding the original envelope text and no
// tool_use blocks at all: an assistant turn claiming it called nothing,
// followed by a user turn answering calls that are not in the transcript.
// Strict providers reject that outright ("Missing tool use for tool_result"),
// and permissive ones answer from a transcript that never happened.
//
// The envelope is a codeNERD protocol rather than a provider feature, so any
// provider can emit one — including the ones whose adapters populate blocks.
func TestPromotedPiggybackCallsReachTheTurnAppendedToHistory(t *testing.T) {
	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		config:       DefaultExecutorConfig(),
	}

	envelope := piggySingleRequest("reading x", "req-blocks-1", "read_file", map[string]any{"path": "x"})
	resp := &types.LLMToolResponse{
		Text: envelope,
		// What a native adapter produces for a turn with no tool_use blocks:
		// reasoning, then the prose that happens to be an envelope.
		Blocks: []types.ContentBlock{
			types.ThinkingBlock("the caller wants x read", "sig-piggy-1"),
			types.TextBlock(envelope),
		},
	}

	if !executor.promotePiggybackToolRequests(resp) {
		t.Fatal("promotion did not fire, so this test proves nothing about it")
	}

	turn := types.AssistantMessageFrom(resp)
	if len(turn.ToolCalls) != 1 || turn.ToolCalls[0].Name != "read_file" {
		t.Fatalf("the history turn offers %#v; the promoted call did not survive "+
			"into the blocks AssistantMessageFrom reads", turn.ToolCalls)
	}

	var sawEnvelope bool
	var sawSignature string
	for _, b := range turn.Content() {
		switch b.Kind {
		case types.BlockText:
			if b.Text == envelope {
				sawEnvelope = true
			}
		case types.BlockThinking:
			sawSignature = b.Signature
		}
	}
	if sawEnvelope {
		t.Error("the raw control envelope survived into the turn's blocks; the model " +
			"would be replayed its own protocol packet as prose")
	}
	if sawSignature != "sig-piggy-1" {
		t.Errorf("promotion dropped the thinking signature (%q); it cannot be "+
			"regenerated, and the provider requires it replayed verbatim", sawSignature)
	}
}

// The other half: a response the provider gave no ordering for must not
// acquire one here. Nil blocks mean "this provider did not tell us the order",
// and the flat fallback is what carries the promoted calls.
func TestPromotionLeavesAFlatResponseFlat(t *testing.T) {
	executor := &Executor{
		kernel:       &MockKernel{},
		virtualStore: &MockVirtualStore{},
		config:       DefaultExecutorConfig(),
	}

	resp := &types.LLMToolResponse{
		Text: piggySingleRequest("reading y", "req-blocks-2", "read_file", map[string]any{"path": "y"}),
	}
	if !executor.promotePiggybackToolRequests(resp) {
		t.Fatal("promotion did not fire")
	}
	if resp.Blocks != nil {
		t.Errorf("promotion invented %d block(s) for a provider that sent none", len(resp.Blocks))
	}
	if turn := types.AssistantMessageFrom(resp); len(turn.ToolCalls) != 1 {
		t.Errorf("the flat fallback lost the promoted call: %#v", turn.ToolCalls)
	}
}
