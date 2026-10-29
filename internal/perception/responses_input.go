package perception

// Responses API input construction, shared by every Responses client:
// codeNERD history in, the protocol's ordered input items out, with
// encrypted reasoning replayed in position.

import (
	"codenerd/internal/logging"
	"codenerd/internal/types"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"strings"
)

// responsesInputText builds a message item whose content is a single text block.
//
// The block type differs by role and is not interchangeable: user and system
// text is "input_text", assistant text replayed back to the model is
// "output_text". Sending "input_text" on an assistant turn is rejected.
func responsesInputText(role, text string) map[string]any {
	blockType := "input_text"
	if role == "assistant" {
		blockType = "output_text"
	}
	return map[string]any{
		"role": role,
		"content": []map[string]any{
			{"type": blockType, "text": text},
		},
	}
}

// responsesReasoningItem rebuilds a reasoning block for replay. The encrypted
// content is opaque and is passed straight back.
func responsesReasoningItem(id, encrypted string) map[string]any {
	return map[string]any{
		"type":              "reasoning",
		"id":                id,
		"summary":           []any{},
		"encrypted_content": encrypted,
	}
}

// responsesFunctionCallItem replays an assistant tool call. call_id must match the
// function_call_output that answers it, or the API rejects the pair.
func responsesFunctionCallItem(callID, name, arguments string) map[string]any {
	if strings.TrimSpace(arguments) == "" {
		arguments = "{}"
	}
	return map[string]any{
		"type":      "function_call",
		"call_id":   callID,
		"name":      name,
		"arguments": arguments,
	}
}

// responsesFunctionOutputItem carries a tool result back to the model.
func responsesFunctionOutputItem(callID, output string) map[string]any {
	return map[string]any{
		"type":    "function_call_output",
		"call_id": callID,
		"output":  output,
	}
}

// responsesToolsFromDefinitions converts codeNERD tool definitions into Responses
// tools. The shape is flatter than Chat Completions: no nested "function" key.
func responsesToolsFromDefinitions(tools []ToolDefinition) []responsesTool {
	if len(tools) == 0 {
		return nil
	}
	out := make([]responsesTool, 0, len(tools))
	for _, t := range tools {
		out = append(out, responsesTool{
			Type:        "function",
			Name:        t.Name,
			Description: t.Description,
			Parameters:  t.InputSchema,
		})
	}
	return out
}

// responsesInputFromHistory converts codeNERD's conversation history into Responses
// input items, in the order the turn's content blocks record.
//
// Reasoning is replayed only from the turn's own blocks: a BlockThinking block
// carrying a Signature becomes a reasoning replay item, in position,
// interleaved with the text and calls exactly as the model emitted them. A text-only
// assistant turn replays no reasoning. There is no side cache and no second
// source of truth.
//
// Tool results become separate function_call_output items rather than user
// messages, and each is paired to its call by call_id.
func responsesInputFromHistory(systemPrompt string, history []types.Message) []any {
	input := make([]any, 0, len(history)+4)

	if strings.TrimSpace(systemPrompt) != "" {
		// "developer" is the preferred role for instructions on both OpenAI
		// and Meta; "system" is accepted but documented as the older spelling.
		input = append(input, responsesInputText("developer", systemPrompt))
	}

	seenCallIDs := make(map[string]struct{})
	seenReasoningIDs := make(map[string]struct{})
	seenOutputIDs := make(map[string]struct{})

	for _, msg := range history {
		textRole := "user"
		if msg.Role == "assistant" {
			textRole = "assistant"
		}
		blocks := msg.Content()

		for _, b := range blocks {
			switch b.Kind {
			case types.BlockThinking:
				// An unsigned thinking block has no encrypted_content and
				// nothing the API will accept; replaying an empty one would
				// only add an item the model cannot read.
				if b.Signature == "" {
					continue
				}
				if b.ID != "" {
					if _, ok := seenReasoningIDs[b.ID]; ok {
						logging.Get(logging.CategoryAPI).Warn("responses: duplicate reasoning id %s skipped", b.ID)
						continue
					}
					seenReasoningIDs[b.ID] = struct{}{}
				}
				input = append(input, responsesReasoningItem(b.ID, b.Signature))

			case types.BlockText:
				if strings.TrimSpace(b.Text) == "" {
					continue
				}
				input = append(input, responsesInputText(textRole, b.Text))

			case types.BlockToolUse:
				if _, ok := seenCallIDs[b.ID]; ok {
					logging.Get(logging.CategoryAPI).Warn("responses: duplicate function_call call_id %s skipped", b.ID)
					continue
				}
				seenCallIDs[b.ID] = struct{}{}
				// ToolCall input is a decoded map; the wire wants the JSON
				// text the model originally emitted. A marshal failure must
				// not drop the call — an unpaired function_call_output on the
				// next turn is rejected by the API, so send "{}" and let the
				// pairing survive.
				args := "{}"
				if len(b.Input) > 0 {
					if encoded, err := json.Marshal(b.Input); err == nil {
						args = string(encoded)
					} else {
						logging.Get(logging.CategoryAPI).Warn(
							"responses: could not marshal args for tool %s (%v); sending {}", b.Name, err)
					}
				}
				input = append(input, responsesFunctionCallItem(b.ID, b.Name, args))

			case types.BlockToolResult:
				// Tool results are their own item type and must not be folded
				// into the user text, or the model cannot pair them with their
				// calls.
				if _, ok := seenOutputIDs[b.ToolUseID]; ok {
					logging.Get(logging.CategoryAPI).Warn("responses: duplicate function_call_output call_id %s skipped", b.ToolUseID)
					continue
				}
				seenOutputIDs[b.ToolUseID] = struct{}{}
				input = append(input, responsesFunctionOutputItem(b.ToolUseID, b.Text))
			}
		}
	}

	return input
}

// promptCacheKey groups requests that share a cacheable prefix: the model and
// the tool catalog, which is the first thing on the wire. The provider routes
// on the key together with the prompt's first tokens, and more than ~15
// requests a minute on one route overflow to machines that do not hold the
// prefix.
//
// It was one key for every request (codenerd:<model>:<maxOutput>), so every
// slot, persona and parallel shard competed for one route whenever their
// catalogs began alike, and measured 2026-09-22 7% of later tool-loop rounds
// lost their whole stable prefix for no reason visible in the request. The
// output ceiling never shaped the prefix. The system prompt is left out on
// purpose: its skeleton is shared across turns (the assembler puts it first)
// while its tail is per turn, so hashing it would give every turn its own
// route and forfeit the skeleton a new turn's first round reuses. A key that
// varied per session or per request would partition the cache the same way.
func promptCacheKey(model string, tools []responsesTool) string {
	catalog, err := json.Marshal(tools)
	if err != nil {
		return "codenerd:" + model
	}
	sum := sha256.Sum256(catalog)
	return "codenerd:" + model + ":" + hex.EncodeToString(sum[:8])
}
