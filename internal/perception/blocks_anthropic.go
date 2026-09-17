package perception

import (
	"fmt"
	"strings"

	"codenerd/internal/types"
)

// Anthropic is the reference adapter for provider fidelity: it is the only
// surface here that can express every block kind natively — ordered text,
// signed thinking, tool_use with its id, tool_result paired back to it — so
// both directions are total and nothing has to be dropped.

// buildAnthropicMessagesFromHistory maps the provider-neutral history into
// Anthropic's message shape, block for block and in order.
//
// A turn that carries a single text block is sent as a bare string, which is
// what the endpoint expects for ordinary prose and what this function has
// always emitted; anything richer becomes the structured content-block form.
func buildAnthropicMessagesFromHistory(history []types.Message) ([]AnthropicMessage, error) {
	messages := make([]AnthropicMessage, 0, len(history))
	for i, m := range history {
		if m.Role != "user" && m.Role != "assistant" {
			return nil, fmt.Errorf("unsupported role %q in history[%d]", m.Role, i)
		}

		content := m.Content()

		// The bare-string form: nothing to say, or nothing but prose.
		if len(content) == 0 {
			messages = append(messages, AnthropicMessage{Role: m.Role, Content: ""})
			continue
		}
		if len(content) == 1 && content[0].Kind == types.BlockText {
			messages = append(messages, AnthropicMessage{Role: m.Role, Content: content[0].Text})
			continue
		}

		blocks := make([]AnthropicContentBlock, 0, len(content))
		for _, b := range content {
			switch b.Kind {
			case types.BlockText:
				// An empty or whitespace-only text block alongside tool blocks
				// is rejected by the endpoint, and carries nothing anyway.
				if strings.TrimSpace(b.Text) == "" {
					continue
				}
				blocks = append(blocks, AnthropicContentBlock{Type: "text", Text: b.Text})

			case types.BlockThinking:
				// A thinking block with no attestation cannot be replayed:
				// Anthropic rejects it. Dropping it loses reasoning continuity
				// for that turn, which is strictly better than failing the
				// call, and it can only happen for a block we did not receive
				// from Anthropic in the first place.
				if b.Signature == "" {
					continue
				}
				if b.Redacted {
					blocks = append(blocks, AnthropicContentBlock{Type: "redacted_thinking", Data: b.Signature})
					continue
				}
				blocks = append(blocks, AnthropicContentBlock{
					Type:      "thinking",
					Thinking:  b.Text,
					Signature: b.Signature,
				})

			case types.BlockToolUse:
				blocks = append(blocks, AnthropicContentBlock{
					Type:  "tool_use",
					ID:    b.ID,
					Name:  b.Name,
					Input: b.Input,
				})

			case types.BlockToolResult:
				if b.ToolUseID == "" {
					return nil, fmt.Errorf("%s message %d: tool_result missing tool_use_id", m.Role, i)
				}
				blocks = append(blocks, AnthropicContentBlock{
					Type:      "tool_result",
					ToolUseID: b.ToolUseID,
					Content:   b.Text,
					IsError:   b.IsError,
				})
			}
		}

		if len(blocks) == 0 {
			// Every block was dropped (whitespace text, unsigned thinking).
			// An empty content array is rejected; the bare string is not.
			messages = append(messages, AnthropicMessage{Role: m.Role, Content: ""})
			continue
		}
		messages = append(messages, AnthropicMessage{Role: m.Role, Content: blocks})
	}
	return messages, nil
}

// blocksFromAnthropicContent maps a response's content array back into ordered
// neutral blocks, preserving position, tool-use ids and thinking signatures.
func blocksFromAnthropicContent(content []AnthropicContentBlock) []types.ContentBlock {
	if len(content) == 0 {
		return nil
	}
	out := make([]types.ContentBlock, 0, len(content))
	for _, block := range content {
		switch block.Type {
		case "text":
			out = append(out, types.TextBlock(block.Text))
		case "thinking":
			out = append(out, types.ThinkingBlock(block.Thinking, block.Signature))
		case "redacted_thinking":
			out = append(out, types.RedactedThinkingBlock(block.Data))
		case "tool_use":
			out = append(out, types.ToolUseBlock(block.ID, block.Name, block.Input))
		case "tool_result":
			out = append(out, types.ToolResultBlock(block.ToolUseID, block.Content, block.IsError))
		}
	}
	return out
}

// anthropicToolResponse assembles the neutral response from a parsed Anthropic
// reply. Text and ToolCalls stay exactly what they were — the flat projection
// of the blocks — so nothing downstream that reads only those changes shape.
func anthropicToolResponse(resp *AnthropicResponse) *types.LLMToolResponse {
	blocks := blocksFromAnthropicContent(resp.Content)
	msg := types.NewAssistantMessage(blocks...)
	return &types.LLMToolResponse{
		Text:       msg.Text,
		ToolCalls:  msg.ToolCalls,
		Blocks:     blocks,
		StopReason: resp.StopReason,
		Usage: types.UsageMetadata{
			InputTokens:  resp.Usage.InputTokens,
			OutputTokens: resp.Usage.OutputTokens,
			TotalTokens:  resp.Usage.InputTokens + resp.Usage.OutputTokens,
		},
	}
}
