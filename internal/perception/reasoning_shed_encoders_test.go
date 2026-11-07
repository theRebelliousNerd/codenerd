package perception

import (
	"testing"

	"codenerd/internal/types"
)

// The working-request guard drops the replayed reasoning of older assistant
// turns when a request does not fit its window (Message.WithoutThinking), and
// keeps the latest turn's. Each provider encoder must accept a history in that
// shape: the stripped turn still says what it said and calls what it called, the
// kept turn still leads with its reasoning, and no reasoning is sent without
// the item it must precede.

// shedHistory is a two-round tool loop whose first assistant turn has had its
// reasoning dropped and whose second (the latest) keeps it.
func shedHistory(t *testing.T) []types.Message {
	t.Helper()
	first := types.RedactedThinkingBlock("enc-1")
	first.ID = "rs_1"
	second := types.RedactedThinkingBlock("enc-2")
	second.ID = "rs_2"
	older, dropped := types.NewAssistantMessage(first, types.TextBlock("looking at a.go"),
		types.ToolUseBlock("call_1", "read_file", map[string]any{"path": "a.go"})).WithoutThinking()
	if dropped != 1 {
		t.Fatalf("fixture: dropped %d reasoning blocks, want 1", dropped)
	}
	return []types.Message{
		types.NewUserMessage(types.TextBlock("fix a.go")),
		older,
		types.NewUserMessage(types.ToolResultBlock("call_1", "package a", false)),
		types.NewAssistantMessage(second, types.TextBlock("editing"),
			types.ToolUseBlock("call_2", "edit_file", map[string]any{"path": "a.go"})),
		types.NewUserMessage(types.ToolResultBlock("call_2", "ok", false)),
	}
}

func TestResponsesInput_AcceptsATurnWithItsReasoningShed(t *testing.T) {
	input := responsesInputFromHistory("SYSTEM", shedHistory(t), responsesInputOptions{})
	var kinds []string
	for i, raw := range input {
		item := raw.(map[string]any)
		kind, _ := item["type"].(string)
		if kind == "" {
			kind = "message:" + item["role"].(string)
		}
		kinds = append(kinds, kind)
		if kind == "reasoning" {
			if item["id"] != "rs_2" || item["encrypted_content"] != "enc-2" {
				t.Fatalf("the reasoning item on the wire is %v, want only the latest turn's", item)
			}
			if i+1 >= len(input) || input[i+1].(map[string]any)["role"] != "assistant" {
				t.Fatalf("the latest reasoning item must precede its turn's output: %v", kinds)
			}
		}
	}
	want := []string{
		"message:developer", "message:user",
		"message:assistant", "function_call", "function_call_output",
		"reasoning", "message:assistant", "function_call", "function_call_output",
	}
	if len(kinds) != len(want) {
		t.Fatalf("input items = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("input items = %v, want %v", kinds, want)
		}
	}
}

// A turn can end on a reasoning item when what followed it was dropped. The
// Responses API rejects that, so the encoder does not send it.
func TestResponsesInput_NeverEndsATurnOnReasoning(t *testing.T) {
	lone := types.RedactedThinkingBlock("enc-lone")
	lone.ID = "rs_lone"
	trailing := types.RedactedThinkingBlock("enc-trailing")
	trailing.ID = "rs_trailing"
	kept := types.RedactedThinkingBlock("enc-kept")
	kept.ID = "rs_kept"
	history := []types.Message{
		types.NewUserMessage(types.TextBlock("task")),
		// Blank prose after the reasoning: the text item is skipped.
		types.NewAssistantMessage(lone, types.TextBlock("  ")),
		types.NewUserMessage(types.TextBlock("go on")),
		types.NewAssistantMessage(kept, types.TextBlock("answer"), trailing),
	}
	input := responsesInputFromHistory("", history, responsesInputOptions{})
	for i, raw := range input {
		item := raw.(map[string]any)
		if item["type"] != "reasoning" {
			continue
		}
		if item["id"] != "rs_kept" {
			t.Fatalf("a reasoning item with nothing after it was sent: %v", item)
		}
		if i+1 >= len(input) {
			t.Fatalf("a reasoning item ends the input: %v", input)
		}
	}
	if got := countItemsByType(input, "reasoning"); got != 1 {
		t.Fatalf("%d reasoning items sent, want the one that precedes its answer", got)
	}
}

func TestAnthropicMessages_AcceptsATurnWithItsReasoningShed(t *testing.T) {
	messages, err := buildAnthropicMessagesFromHistory(shedHistory(t))
	if err != nil {
		t.Fatalf("buildAnthropicMessagesFromHistory: %v", err)
	}
	older, ok := messages[1].Content.([]AnthropicContentBlock)
	if !ok || len(older) != 2 || older[0].Type != "text" || older[0].Text != "looking at a.go" ||
		older[1].Type != "tool_use" || older[1].ID != "call_1" {
		t.Fatalf("the stripped turn must still say and call what it did: %+v", messages[1].Content)
	}
	latest, ok := messages[3].Content.([]AnthropicContentBlock)
	if !ok || len(latest) != 3 || latest[0].Type != "redacted_thinking" || latest[0].Data != "enc-2" ||
		latest[1].Type != "text" || latest[2].Type != "tool_use" {
		t.Fatalf("the latest tool-use turn must lead with its reasoning: %+v", messages[3].Content)
	}
}

func TestGeminiContents_AcceptsATurnWithItsReasoningShed(t *testing.T) {
	history := shedHistory(t)
	// A Gemini function call carries its own signature, which a shed turn keeps.
	call := types.ToolUseBlock("call_1", "read_file", map[string]any{"path": "a.go"})
	call.Signature = "gemini-call-sig"
	older, dropped := types.NewAssistantMessage(types.ThinkingBlock("why", "gemini-thought-sig"), types.TextBlock("looking at a.go"), call).WithoutThinking()
	if dropped != 1 {
		t.Fatalf("fixture: dropped %d reasoning blocks, want 1", dropped)
	}
	history[1] = older

	contents, err := geminiContentsFromHistory(history)
	if err != nil {
		t.Fatalf("geminiContentsFromHistory: %v", err)
	}
	var modelTurns []GeminiContent
	for _, c := range contents {
		if c.Role == "model" {
			modelTurns = append(modelTurns, c)
		}
	}
	if len(modelTurns) != 2 {
		t.Fatalf("%d model turns encoded, want 2", len(modelTurns))
	}
	first := modelTurns[0].Parts
	if len(first) != 2 || first[0].Thought || first[0].Text != "looking at a.go" ||
		first[1].FunctionCall == nil || first[1].FunctionCall.Name != "read_file" || first[1].FunctionCall.ThoughtSignature != "gemini-call-sig" {
		t.Fatalf("the stripped turn must keep its prose and its signed call: %+v", first)
	}
	latest := modelTurns[1].Parts
	if len(latest) != 3 || !latest[0].Thought || latest[0].ThoughtSignature != "enc-2" {
		t.Fatalf("the latest turn must lead with its signed thought part: %+v", latest)
	}
}

func TestChatCompletionsMessages_AcceptATurnWithItsReasoningShed(t *testing.T) {
	msgs, err := MapTypesHistoryToOpenAIMessages("SYSTEM", shedHistory(t))
	if err != nil {
		t.Fatalf("MapTypesHistoryToOpenAIMessages: %v", err)
	}
	var assistants []OpenAIMessage
	for _, m := range msgs {
		if m.Role == "assistant" {
			assistants = append(assistants, m)
		}
	}
	if len(assistants) != 2 {
		t.Fatalf("%d assistant messages, want 2", len(assistants))
	}
	if assistants[0].Content != "looking at a.go" || len(assistants[0].ToolCalls) != 1 || assistants[0].ToolCalls[0].ID != "call_1" {
		t.Fatalf("the stripped turn must keep its prose and call: %+v", assistants[0])
	}
}
