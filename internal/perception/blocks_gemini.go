package perception

import (
	"fmt"

	"codenerd/internal/types"
)

// Gemini's native content model is ordered — a turn is a list of parts and the
// order is the model's own — so text, reasoning and function calls survive a
// round trip in position. Two things do not, and both are properties of the
// vendor's protocol rather than of this code:
//
//   - There are NO tool-call ids on the wire. Gemini pairs a functionResponse
//     to its functionCall by tool NAME, and this client mints "call_N" ids from
//     the position of each call in the response so the rest of codeNERD has
//     something to pair on. An id therefore survives a round trip only in the
//     sense that the same position produces the same id; the id the model saw
//     is not a thing that exists. geminiToolNameByID below is what turns our
//     synthetic id back into the name the wire needs.
//
//   - A tool_result's IsError has no native field. It is carried inside the
//     response map as "is_error", which the model reads as data rather than as
//     protocol.
//
// Reasoning does have a native home: a part carries Thought and a
// ThoughtSignature that must be replayed verbatim and in position.

// geminiRoleFor maps a neutral role onto Gemini's vocabulary. Gemini calls the
// assistant "model"; tool output goes in its own "function" role.
func geminiRoleFor(role string) string {
	if role == "assistant" {
		return "model"
	}
	return "user"
}

// geminiToolNameByID indexes every tool_use block in a history by its id, so a
// later tool_result — which only knows the id — can be emitted with the tool
// name Gemini pairs on.
func geminiToolNameByID(history []types.Message) map[string]string {
	names := make(map[string]string)
	for _, m := range history {
		for _, b := range m.Content() {
			if b.Kind == types.BlockToolUse && b.ID != "" {
				names[b.ID] = b.Name
			}
		}
	}
	return names
}

// geminiContentsFromHistory maps the neutral history onto Gemini contents,
// part for part and in order.
//
// A turn's tool results become their own "function" content, because Gemini
// requires them under that role; everything else in the turn stays in one
// content under the turn's own role, in block order.
func geminiContentsFromHistory(history []types.Message) ([]GeminiContent, error) {
	names := geminiToolNameByID(history)
	contents := make([]GeminiContent, 0, len(history))

	for i, m := range history {
		if m.Role != "" && m.Role != "user" && m.Role != "assistant" {
			return nil, fmt.Errorf("unsupported role %q in history[%d]", m.Role, i)
		}

		var parts []GeminiPart
		var results []GeminiPart

		for _, b := range m.Content() {
			switch b.Kind {
			case types.BlockText:
				if b.Text == "" {
					continue
				}
				parts = append(parts, GeminiPart{Text: b.Text})

			case types.BlockThinking:
				// A thought part with no signature cannot be replayed for
				// continuity, and its text is the model's private reasoning —
				// sending it back as ordinary prose would put words in the
				// model's mouth. Drop it rather than misrepresent it.
				if b.Signature == "" {
					continue
				}
				parts = append(parts, GeminiPart{
					Text:             b.Text,
					Thought:          true,
					ThoughtSignature: b.Signature,
				})

			case types.BlockToolUse:
				call := &GeminiFunctionCall{Name: b.Name, Args: b.Input}
				if b.Signature != "" {
					call.ThoughtSignature = b.Signature
				}
				parts = append(parts, GeminiPart{FunctionCall: call})

			case types.BlockToolResult:
				name := names[b.ToolUseID]
				if name == "" {
					// No prior tool_use in this history names it. The id is
					// the best label left; Gemini will not pair it, and that
					// is visible in the payload rather than hidden.
					name = b.ToolUseID
				}
				part := GeminiPart{
					FunctionResponse: &GeminiFunctionResponse{
						Name: name,
						Response: map[string]any{
							"content":  b.Text,
							"is_error": b.IsError,
						},
					},
				}
				if b.Signature != "" {
					part.ThoughtSignature = b.Signature
				}
				results = append(results, part)
			}
		}

		if len(results) > 0 {
			contents = append(contents, GeminiContent{Role: "function", Parts: results})
		}
		if len(parts) > 0 {
			contents = append(contents, GeminiContent{Role: geminiRoleFor(m.Role), Parts: parts})
		}
	}
	return contents, nil
}

// blocksFromGeminiParts maps a response candidate's parts back into ordered
// neutral blocks.
//
// Tool-use ids are minted positionally ("call_0", "call_1", …) to match
// GeminiClient.extractToolCalls, which is what the rest of the tool loop pairs
// results on. responseSignature is the turn-level thoughtSignature, used for a
// call that carries none of its own — the same precedence extractToolCalls
// applies, kept in one place so the two cannot drift.
func blocksFromGeminiParts(parts []GeminiResponsePart, responseSignature string) []types.ContentBlock {
	if len(parts) == 0 {
		return nil
	}
	out := make([]types.ContentBlock, 0, len(parts))
	calls := 0
	for _, part := range parts {
		switch {
		case part.FunctionCall != nil:
			signature := part.FunctionCall.ThoughtSignature
			if signature == "" {
				signature = part.ThoughtSignature
			}
			if signature == "" {
				signature = responseSignature
			}
			block := types.ToolUseBlock(fmt.Sprintf("call_%d", calls), part.FunctionCall.Name, part.FunctionCall.Args)
			block.Signature = signature
			out = append(out, block)
			calls++

		case part.Thought:
			signature := part.ThoughtSignature
			if signature == "" {
				signature = responseSignature
			}
			out = append(out, types.ThinkingBlock(part.Text, signature))

		case part.Text != "":
			out = append(out, types.TextBlock(part.Text))
		}
	}
	return out
}

// applyGeminiBlocks fills a response's ordered blocks and the flat projection
// they imply from a Gemini reply's first candidate.
//
// Text is now the concatenation of the answer's TEXT parts only. It previously
// included the text of thought parts, so with includeThoughts on, the model's
// private reasoning was prepended to the visible answer — and to anything that
// parsed that answer.
func applyGeminiBlocks(result *types.LLMToolResponse, resp *GeminiResponse) {
	if result == nil || resp == nil || len(resp.Candidates) == 0 {
		return
	}
	blocks := blocksFromGeminiParts(resp.Candidates[0].Content.Parts, resp.ThoughtSignature)
	msg := types.NewAssistantMessage(blocks...)
	result.Blocks = blocks
	result.Text = msg.Text
	result.ToolCalls = msg.ToolCalls
}

// applyFallbackThoughtSignatures fills in signatures the history did not carry
// from the client's memory of the last response.
//
// It exists for the legacy shape: a history built from flat Text/ToolCalls
// fields has no place to record which signature belonged to which call, so
// before ordered blocks the only signature available was the one the last
// response left on the client. A block-built history carries its own and this
// changes nothing.
func (c *GeminiClient) applyFallbackThoughtSignatures(contents []GeminiContent) {
	if c == nil || c.lastThoughtSignature == "" {
		return
	}
	byName := make(map[string]string, len(c.lastToolCalls))
	for _, call := range c.lastToolCalls {
		if call.signature != "" {
			byName[call.name] = call.signature
		}
	}
	for i := range contents {
		for j := range contents[i].Parts {
			part := &contents[i].Parts[j]
			switch {
			case part.FunctionResponse != nil:
				if part.ThoughtSignature != "" {
					continue
				}
				if sig, ok := byName[part.FunctionResponse.Name]; ok {
					part.ThoughtSignature = sig
					continue
				}
				part.ThoughtSignature = c.lastThoughtSignature
			case part.FunctionCall != nil:
				if part.FunctionCall.ThoughtSignature == "" && part.ThoughtSignature == "" {
					part.FunctionCall.ThoughtSignature = c.lastThoughtSignature
				}
			}
		}
	}
}
