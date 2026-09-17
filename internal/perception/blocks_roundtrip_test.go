package perception

import (
	"encoding/json"
	"reflect"
	"testing"

	"codenerd/internal/types"
)

// Round-trip tests for provider fidelity.
//
// Every one of these drives the SAME turn: prose, a tool call, more prose, a
// second tool call, behind a signed thinking block. A single text block proves
// nothing — the defect these tests exist for is that Text and ToolCalls are
// separate fields, so the order between them was never recorded and a turn
// rebuilt from them is a turn the model never produced.
//
// Where a provider cannot carry something, the test asserts exactly WHAT is
// lost rather than skipping the case. A silently narrowed assertion is how a
// fidelity limit turns into a fidelity claim.

func interleavedAssistantTurn() []types.ContentBlock {
	return []types.ContentBlock{
		types.ThinkingBlock("Check the config first, then the caller.", "sig-abc-123"),
		types.TextBlock("I'll check two things."),
		types.ToolUseBlock("toolu_A", "read_file", map[string]any{"path": "config.go"}),
		types.TextBlock("Now the caller."),
		types.ToolUseBlock("toolu_B", "search_code", map[string]any{"query": "LoadConfig"}),
	}
}

func interleavedHistory() []types.Message {
	return []types.Message{
		types.NewUserMessage(types.TextBlock("why does LoadConfig ignore the flag?")),
		types.NewAssistantMessage(interleavedAssistantTurn()...),
		types.NewUserMessage(
			types.ToolResultBlock("toolu_A", "package config\n...", false),
			types.ToolResultBlock("toolu_B", "no matches", true),
		),
	}
}

// =============================================================================
// ANTHROPIC — total fidelity
// =============================================================================

func TestAnthropicRoundTripPreservesOrderSignaturesAndIDs(t *testing.T) {
	history := interleavedHistory()

	msgs, err := buildAnthropicMessagesFromHistory(history)
	if err != nil {
		t.Fatalf("buildAnthropicMessagesFromHistory: %v", err)
	}
	if len(msgs) != 3 {
		t.Fatalf("built %d messages from a 3-turn history: %#v", len(msgs), msgs)
	}

	// Go out through the wire encoding and back, so the round trip is through
	// the JSON tags the API actually reads rather than through Go structs that
	// happen to share field names.
	assistant, ok := msgs[1].Content.([]AnthropicContentBlock)
	if !ok {
		t.Fatalf("assistant turn was not sent as content blocks: %T", msgs[1].Content)
	}
	encoded, err := json.Marshal(assistant)
	if err != nil {
		t.Fatalf("marshal assistant content: %v", err)
	}
	var decoded []AnthropicContentBlock
	if err := json.Unmarshal(encoded, &decoded); err != nil {
		t.Fatalf("unmarshal assistant content: %v", err)
	}

	got := blocksFromAnthropicContent(decoded)
	want := interleavedAssistantTurn()
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("assistant turn did not survive the round trip:\n got %#v\nwant %#v", got, want)
	}

	// Belt and braces on the two properties the whole exercise is about.
	if got[0].Signature != "sig-abc-123" {
		t.Errorf("thinking signature = %q, want it replayed verbatim", got[0].Signature)
	}
	if got[2].ID != "toolu_A" || got[4].ID != "toolu_B" {
		t.Errorf("tool_use ids lost: %q, %q", got[2].ID, got[4].ID)
	}
	if string(encoded) == "" || !jsonHasKey(t, encoded, "signature") {
		t.Error("the wire payload carries no signature field")
	}
}

func TestAnthropicToolResultTurnRoundTrips(t *testing.T) {
	history := interleavedHistory()
	msgs, err := buildAnthropicMessagesFromHistory(history)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	results, ok := msgs[2].Content.([]AnthropicContentBlock)
	if !ok {
		t.Fatalf("tool-result turn was not sent as content blocks: %T", msgs[2].Content)
	}
	got := blocksFromAnthropicContent(results)
	want := []types.ContentBlock{
		types.ToolResultBlock("toolu_A", "package config\n...", false),
		types.ToolResultBlock("toolu_B", "no matches", true),
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("tool results did not survive:\n got %#v\nwant %#v", got, want)
	}
}

func TestAnthropicRedactedThinkingRoundTrips(t *testing.T) {
	history := []types.Message{
		types.NewAssistantMessage(
			types.RedactedThinkingBlock("encrypted-blob"),
			types.TextBlock("done"),
		),
	}
	msgs, err := buildAnthropicMessagesFromHistory(history)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	blocks := msgs[0].Content.([]AnthropicContentBlock)
	if blocks[0].Type != "redacted_thinking" || blocks[0].Data != "encrypted-blob" {
		t.Fatalf("redacted thinking not emitted in its own wire shape: %#v", blocks[0])
	}
	got := blocksFromAnthropicContent(blocks)
	if !got[0].Redacted || got[0].Signature != "encrypted-blob" {
		t.Fatalf("redacted thinking did not survive: %#v", got[0])
	}
}

func TestAnthropicDropsUnsignedThinkingRatherThanSendingIt(t *testing.T) {
	// An unsigned thinking block cannot have come from Anthropic and cannot be
	// replayed to it — the API rejects it. Dropping it keeps the call alive;
	// sending it fails the whole turn.
	history := []types.Message{
		types.NewAssistantMessage(
			types.ThinkingBlock("reasoning from some other provider", ""),
			types.TextBlock("hello"),
		),
	}
	msgs, err := buildAnthropicMessagesFromHistory(history)
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	got := blocksFromAnthropicContent(msgs[0].Content.([]AnthropicContentBlock))
	want := []types.ContentBlock{types.TextBlock("hello")}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("unsigned thinking reached the wire: %#v", got)
	}
}

func TestAnthropicLegacyMessagesKeepTheirExactWireShape(t *testing.T) {
	// The migration's central risk: a message built the old way must produce
	// byte-identical output to what it produced before blocks existed.
	history := []types.Message{
		{Role: "user", Text: "hi"},
		{Role: "assistant", Text: "sure", ToolCalls: []types.ToolCall{{ID: "t1", Name: "ls", Input: map[string]any{"p": "."}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "t1", Content: "a.go"}}},
		{Role: "assistant", Text: "found it"},
	}
	msgs, err := buildAnthropicMessagesFromHistory(history)
	if err != nil {
		t.Fatalf("build: %v", err)
	}

	if msgs[0].Content != "hi" {
		t.Errorf("plain user turn = %#v, want the bare string form", msgs[0].Content)
	}
	assistant := msgs[1].Content.([]AnthropicContentBlock)
	if len(assistant) != 2 || assistant[0].Type != "text" || assistant[1].Type != "tool_use" {
		t.Errorf("assistant turn = %#v, want text then tool_use", assistant)
	}
	results := msgs[2].Content.([]AnthropicContentBlock)
	if len(results) != 1 || results[0].Type != "tool_result" || results[0].ToolUseID != "t1" {
		t.Errorf("tool-result turn = %#v", results)
	}
	if msgs[3].Content != "found it" {
		t.Errorf("final turn = %#v, want the bare string form", msgs[3].Content)
	}
}

func TestAnthropicRejectsToolResultWithoutAnID(t *testing.T) {
	history := []types.Message{
		{Role: "user", ToolResults: []types.ToolResult{{Content: "orphan"}}},
	}
	if _, err := buildAnthropicMessagesFromHistory(history); err == nil {
		t.Fatal("an unpaired tool_result was accepted; Anthropic rejects the request and the loop stalls")
	}
}

func TestAnthropicResponseParsesBlocksAndProjection(t *testing.T) {
	raw := `{"content":[
		{"type":"thinking","thinking":"hmm","signature":"sig-9"},
		{"type":"text","text":"one"},
		{"type":"tool_use","id":"toolu_A","name":"ls","input":{"p":"."}},
		{"type":"text","text":"two"}
	],"stop_reason":"tool_use","usage":{"input_tokens":11,"output_tokens":7}}`
	var resp AnthropicResponse
	if err := json.Unmarshal([]byte(raw), &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	got := anthropicToolResponse(&resp)
	if len(got.Blocks) != 4 {
		t.Fatalf("parsed %d blocks, want 4: %#v", len(got.Blocks), got.Blocks)
	}
	if got.Blocks[0].Signature != "sig-9" {
		t.Errorf("signature lost on parse: %#v", got.Blocks[0])
	}
	// The flat projection is what every existing caller reads, and it must not
	// change: text concatenated, reasoning excluded.
	if got.Text != "onetwo" {
		t.Errorf("Text projection = %q, want %q", got.Text, "onetwo")
	}
	if len(got.ToolCalls) != 1 || got.ToolCalls[0].ID != "toolu_A" {
		t.Errorf("ToolCalls projection = %#v", got.ToolCalls)
	}
	if got.Usage.TotalTokens != 18 {
		t.Errorf("usage = %#v", got.Usage)
	}
}

// =============================================================================
// OPENAI CHAT COMPLETIONS — the shared surface, and what it cannot hold
// =============================================================================

func TestChatCompletionsKeepsIDsAndPairingButCollapsesOrder(t *testing.T) {
	msgs, err := MapTypesHistoryToOpenAIMessages("sys", interleavedHistory())
	if err != nil {
		t.Fatalf("map: %v", err)
	}

	// system, user, assistant(2 calls), tool, tool
	if len(msgs) != 5 {
		t.Fatalf("produced %d messages: %#v", len(msgs), msgs)
	}

	assistant := msgs[2]
	if assistant.Role != "assistant" {
		t.Fatalf("message 2 is %q, want assistant", assistant.Role)
	}

	// What survives: both tool calls, in order, with their ids and arguments.
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

	// What does NOT survive, asserted rather than assumed: the interleaving
	// collapses to all-text-then-all-calls, because an assistant message has
	// one content string.
	if assistant.Content != "I'll check two things.Now the caller." {
		t.Errorf("collapsed content = %q; the text that came after the first call belongs before both", assistant.Content)
	}

	// And the thinking block is gone entirely: there is no request-side field
	// for reasoning on this surface.
	for _, m := range msgs {
		if m.Content == "Check the config first, then the caller." {
			t.Fatal("reasoning was smuggled into a content string; it would corrupt structured-output parses")
		}
	}

	// Results keep their pairing.
	if msgs[3].ToolCallID != "toolu_A" || msgs[4].ToolCallID != "toolu_B" {
		t.Errorf("tool result pairing lost: %q, %q", msgs[3].ToolCallID, msgs[4].ToolCallID)
	}
	if msgs[4].Content != "ERROR: no matches" {
		t.Errorf("error flag lost: %q", msgs[4].Content)
	}
}

func TestChatCompletionsLegacyMessagesKeepTheirExactWireShape(t *testing.T) {
	history := []types.Message{
		{Role: "user", Text: "hi"},
		{Role: "assistant", Text: "sure", ToolCalls: []types.ToolCall{{ID: "t1", Name: "ls", Input: map[string]any{"p": "."}}}},
		{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "t1", Content: "a.go"}}},
		{Role: "", Text: "thanks"},
	}
	msgs, err := MapTypesHistoryToOpenAIMessages("", history)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	want := []OpenAIMessage{
		{Role: "user", Content: "hi"},
		{Role: "assistant", Content: "sure", ToolCalls: []OpenAIToolCall{{
			ID: "t1", Type: "function",
			Function: OpenAIFunctionCall{Name: "ls", Arguments: `{"p":"."}`},
		}}},
		{Role: "tool", Content: "a.go", ToolCallID: "t1"},
		{Role: "user", Content: "thanks"},
	}
	if !reflect.DeepEqual(msgs, want) {
		t.Fatalf("legacy history changed shape:\n got %#v\nwant %#v", msgs, want)
	}
}

// =============================================================================
// META RESPONSES — ordered items, reasoning replayed from the message
// =============================================================================

func TestMetaResponsesRoundTripPreservesOrderReasoningAndIDs(t *testing.T) {
	raw := `{"id":"resp_1","output":[
		{"type":"reasoning","id":"rs_1","encrypted_content":"enc-1"},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"I'll check two things."}]},
		{"type":"function_call","call_id":"call_A","name":"read_file","arguments":"{\"path\":\"config.go\"}"},
		{"type":"message","role":"assistant","content":[{"type":"output_text","text":"Now the caller."}]},
		{"type":"function_call","call_id":"call_B","name":"search_code","arguments":"{\"query\":\"LoadConfig\"}"}
	]}`
	var reply metaResponsesReply
	if err := json.Unmarshal([]byte(raw), &reply); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	resp := metaToolResponseFromReply(&reply)
	wantKinds := []types.ContentBlockKind{
		types.BlockThinking, types.BlockText, types.BlockToolUse, types.BlockText, types.BlockToolUse,
	}
	if len(resp.Blocks) != len(wantKinds) {
		t.Fatalf("parsed %d blocks, want %d: %#v", len(resp.Blocks), len(wantKinds), resp.Blocks)
	}
	for i, kind := range wantKinds {
		if resp.Blocks[i].Kind != kind {
			t.Fatalf("block %d is %q, want %q", i, resp.Blocks[i].Kind, kind)
		}
	}
	if resp.Blocks[0].Signature != "enc-1" || resp.Blocks[0].ID != "rs_1" {
		t.Fatalf("reasoning item lost its encrypted content or id: %#v", resp.Blocks[0])
	}

	// Now the other direction: the turn goes back into the input array in the
	// same order, with the reasoning read off the message rather than out of
	// the side cache.
	history := []types.Message{
		types.NewUserMessage(types.TextBlock("why?")),
		types.AssistantMessageFrom(resp),
	}
	input := metaInputFromHistory("sys", history, nil)

	wantTypes := []string{
		"", // developer instruction message (no "type" key)
		"", // user message
		"reasoning",
		"", // assistant text
		"function_call",
		"", // assistant text
		"function_call",
	}
	if len(input) != len(wantTypes) {
		t.Fatalf("built %d input items, want %d: %#v", len(input), len(wantTypes), input)
	}
	for i, want := range wantTypes {
		item, ok := input[i].(map[string]any)
		if !ok {
			t.Fatalf("input item %d is %T", i, input[i])
		}
		got, _ := item["type"].(string)
		if got != want {
			t.Fatalf("input item %d has type %q, want %q", i, got, want)
		}
	}
	if reasoning := input[2].(map[string]any); reasoning["encrypted_content"] != "enc-1" || reasoning["id"] != "rs_1" {
		t.Fatalf("reasoning replayed without its verbatim blob: %#v", reasoning)
	}
	if call := input[4].(map[string]any); call["call_id"] != "call_A" || call["arguments"] != `{"path":"config.go"}` {
		t.Fatalf("first call replayed wrong: %#v", call)
	}
	if call := input[6].(map[string]any); call["call_id"] != "call_B" {
		t.Fatalf("second call replayed wrong: %#v", call)
	}
}

func TestMetaResponsesFallsBackToTheSideCacheForALegacyTurn(t *testing.T) {
	// A turn built the old way carries no reasoning of its own, and the
	// per-turn cache is the only place it can come from. This is the path
	// every internal/session-built history still takes.
	history := []types.Message{
		{Role: "user", Text: "go"},
		{Role: "assistant", Text: "on it", ToolCalls: []types.ToolCall{{ID: "call_A", Name: "ls"}}},
	}
	cache := map[string][]metaResponsesItem{
		metaTurnKey(1): {{ID: "rs_legacy", EncryptedContent: "enc-legacy"}},
	}
	input := metaInputFromHistory("", history, cache)

	// user, reasoning (from cache, ahead of the turn), assistant text, call
	if len(input) != 4 {
		t.Fatalf("built %d items: %#v", len(input), input)
	}
	item := input[1].(map[string]any)
	if item["type"] != "reasoning" || item["encrypted_content"] != "enc-legacy" {
		t.Fatalf("cache fallback did not replay: %#v", item)
	}
}

func TestMetaResponsesMessageReasoningWinsOverTheSideCache(t *testing.T) {
	// Precedence is one-way: if the turn brought its own reasoning, the cache
	// is not consulted at all. Two sources replaying into the same slot would
	// double the blocks and corrupt the input array.
	history := []types.Message{
		types.NewAssistantMessage(
			types.RedactedThinkingBlock("enc-from-message"),
			types.TextBlock("hi"),
		),
	}
	cache := map[string][]metaResponsesItem{
		metaTurnKey(0): {{ID: "rs_stale", EncryptedContent: "enc-stale"}},
	}
	input := metaInputFromHistory("", history, cache)

	reasoning := 0
	for _, raw := range input {
		item := raw.(map[string]any)
		if item["type"] == "reasoning" {
			reasoning++
			if item["encrypted_content"] != "enc-from-message" {
				t.Fatalf("stale cached reasoning replayed: %#v", item)
			}
		}
	}
	if reasoning != 1 {
		t.Fatalf("replayed %d reasoning items, want exactly 1", reasoning)
	}
}

// =============================================================================
// GEMINI — ordered parts, per-part signatures, and no tool ids on the wire
// =============================================================================

func TestGeminiRoundTripPreservesOrderAndPerPartSignatures(t *testing.T) {
	// Gemini's ids are minted from position, so build the turn with the ids
	// blocksFromGeminiParts will mint back.
	turn := []types.ContentBlock{
		types.ThinkingBlock("plan it", "sig-think"),
		types.TextBlock("I'll check two things."),
		withSignature(types.ToolUseBlock("call_0", "read_file", map[string]any{"path": "config.go"}), "sig-A"),
		types.TextBlock("Now the caller."),
		withSignature(types.ToolUseBlock("call_1", "search_code", map[string]any{"query": "LoadConfig"}), "sig-B"),
	}
	history := []types.Message{
		types.NewUserMessage(types.TextBlock("why?")),
		types.NewAssistantMessage(turn...),
	}

	contents, err := geminiContentsFromHistory(history)
	if err != nil {
		t.Fatalf("geminiContentsFromHistory: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("built %d contents: %#v", len(contents), contents)
	}
	if contents[1].Role != "model" {
		t.Fatalf("assistant turn sent under role %q, want \"model\"", contents[1].Role)
	}

	got := blocksFromGeminiParts(responsePartsFrom(contents[1].Parts), "")
	if !reflect.DeepEqual(got, turn) {
		t.Fatalf("Gemini turn did not survive the round trip:\n got %#v\nwant %#v", got, turn)
	}
}

func TestGeminiToolResultsGoUnderTheFunctionRolePairedByName(t *testing.T) {
	// Gemini has no tool-call ids. A result is paired to its call by NAME, so
	// the mapper has to resolve the name from the call earlier in the history.
	history := []types.Message{
		types.NewAssistantMessage(types.ToolUseBlock("call_0", "read_file", map[string]any{"path": "a.go"})),
		types.NewUserMessage(types.ToolResultBlock("call_0", "contents", true)),
	}
	contents, err := geminiContentsFromHistory(history)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(contents) != 2 {
		t.Fatalf("built %d contents: %#v", len(contents), contents)
	}
	fn := contents[1]
	if fn.Role != "function" {
		t.Fatalf("tool results sent under role %q, want \"function\"", fn.Role)
	}
	resp := fn.Parts[0].FunctionResponse
	if resp == nil || resp.Name != "read_file" {
		t.Fatalf("result not paired to the call's name: %#v", fn.Parts[0])
	}
	if resp.Response["is_error"] != true {
		t.Fatalf("error flag lost: %#v", resp.Response)
	}
}

func TestGeminiResponseParsingSeparatesThoughtTextFromTheAnswer(t *testing.T) {
	// Previously the parser concatenated every part with text, thought parts
	// included, so the model's private reasoning was prepended to the answer.
	resp := &GeminiResponse{ThoughtSignature: "sig-turn"}
	resp.Candidates = []GeminiResponseCandidate{{}}
	resp.Candidates[0].Content.Parts = []GeminiResponsePart{
		{Text: "internal reasoning", Thought: true, ThoughtSignature: "sig-part"},
		{Text: "the answer"},
		{FunctionCall: &GeminiFunctionCall{Name: "ls", Args: map[string]any{"p": "."}}},
	}

	out := &types.LLMToolResponse{}
	applyGeminiBlocks(out, resp)

	if out.Text != "the answer" {
		t.Errorf("Text = %q; thought text must not reach the visible answer", out.Text)
	}
	if len(out.Blocks) != 3 || out.Blocks[0].Kind != types.BlockThinking {
		t.Fatalf("blocks = %#v", out.Blocks)
	}
	if out.Blocks[0].Signature != "sig-part" {
		t.Errorf("per-part signature lost: %#v", out.Blocks[0])
	}
	// A call with no signature of its own inherits the turn's, matching
	// extractToolCalls so the two cannot disagree.
	if out.Blocks[2].Signature != "sig-turn" {
		t.Errorf("turn-level signature not applied to the call: %#v", out.Blocks[2])
	}
	if len(out.ToolCalls) != 1 || out.ToolCalls[0].ID != "call_0" {
		t.Errorf("tool call projection = %#v", out.ToolCalls)
	}
}

func TestGeminiDropsUnsignedThinkingRatherThanReplayItAsProse(t *testing.T) {
	history := []types.Message{
		types.NewAssistantMessage(
			types.ThinkingBlock("reasoning with no attestation", ""),
			types.TextBlock("hi"),
		),
	}
	contents, err := geminiContentsFromHistory(history)
	if err != nil {
		t.Fatalf("map: %v", err)
	}
	if len(contents[0].Parts) != 1 || contents[0].Parts[0].Text != "hi" {
		t.Fatalf("unsigned reasoning leaked into the request: %#v", contents[0].Parts)
	}
}

func TestGeminiIsAToolResultsProvider(t *testing.T) {
	// The method used to take a pre-built []GeminiContent and had no caller,
	// so Gemini's native multi-turn path was unreachable.
	var _ types.ToolResultsProvider = (*GeminiClient)(nil)
}

// =============================================================================
// CLI ENGINES — no block channel exists, and the tests say so
// =============================================================================

func TestCLIEnginesDoNotClaimANativeHistoryChannel(t *testing.T) {
	// Both shell out to an agent that owns its own conversation. There is no
	// history parameter to carry blocks through, so they must NOT satisfy
	// types.ToolResultsProvider — a false claim here would route a block-
	// carrying history into a text-only transport and lose it silently.
	for _, c := range []any{&ClaudeCodeCLIClient{}, &CodexCLIClient{}} {
		if _, ok := c.(types.ToolResultsProvider); ok {
			t.Errorf("%T claims ToolResultsProvider; it has no channel for content blocks", c)
		}
	}
}

// =============================================================================
// helpers
// =============================================================================

func withSignature(b types.ContentBlock, sig string) types.ContentBlock {
	b.Signature = sig
	return b
}

// responsePartsFrom converts request parts into the response-part shape so the
// Gemini round trip can close. The two are distinct structs in the vendor's
// schema (a response part carries no functionResponse), not an oversight here.
func responsePartsFrom(parts []GeminiPart) []GeminiResponsePart {
	out := make([]GeminiResponsePart, 0, len(parts))
	for _, p := range parts {
		out = append(out, GeminiResponsePart{
			Text:             p.Text,
			Thought:          p.Thought,
			InlineData:       p.InlineData,
			FileData:         p.FileData,
			FunctionCall:     p.FunctionCall,
			ThoughtSignature: p.ThoughtSignature,
		})
	}
	return out
}

func jsonHasKey(t *testing.T, encoded []byte, key string) bool {
	t.Helper()
	var raw []map[string]any
	if err := json.Unmarshal(encoded, &raw); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	for _, m := range raw {
		if _, ok := m[key]; ok {
			return true
		}
	}
	return false
}
