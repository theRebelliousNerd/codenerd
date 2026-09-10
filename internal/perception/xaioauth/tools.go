package xaioauth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// CompleteWithToolResults continues a multi-turn tool-calling conversation
// (OpenAI-compatible tool role messages). Required so campaign/session executors
// can feed tool results back after the first tool_use turn under SuperGrok OAuth.
func (c *Client) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}
	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = "You are codeNERD. Respond in English. Be concise."
	}

	c.rateLimitPace()

	token, err := c.tokens.AccessToken(ctx)
	if err != nil {
		if loadErr := c.tokens.Load(); loadErr == nil {
			token, err = c.tokens.AccessToken(ctx)
		}
		if err != nil {
			return nil, err
		}
	}

	msgs, err := mapHistoryToChatMessages(systemPrompt, history)
	if err != nil {
		return nil, err
	}
	chatTools := make([]chatTool, 0, len(tools))
	for _, t := range tools {
		chatTools = append(chatTools, chatTool{
			Type: "function",
			Function: chatToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}
	reqBody := chatRequest{
		Model:       c.cfg.Model,
		Messages:    msgs,
		Tools:       chatTools,
		ToolChoice:  "auto",
		MaxTokens:   8192,
		Temperature: 0.1,
	}

	status, body, err := doJSON(ctx, c.httpClient, "POST", chatURL(c.cfg.BaseURL), token, reqBody, 10<<20)
	if err != nil {
		return nil, err
	}
	if status == 401 {
		c.tokens.InvalidateAccess()
		token, err = c.tokens.AccessToken(ctx)
		if err != nil {
			return nil, err
		}
		status, body, err = doJSON(ctx, c.httpClient, "POST", chatURL(c.cfg.BaseURL), token, reqBody, 10<<20)
		if err != nil {
			return nil, err
		}
	}
	if status != 200 {
		return nil, classifyHTTPError(status, body, nil)
	}
	return parseToolChatResponse(body)
}

// mapHistoryToChatMessages converts the neutral history into SuperGrok's
// OpenAI-shaped chat messages, reading the turn's ordered content blocks.
//
// It carries the same limits as every Chat Completions surface: an assistant
// turn has one content string and one tool_calls array, so text interleaved
// between two tool calls collapses ahead of both, and there is no request-side
// field for reasoning, so thinking blocks are counted out here rather than
// folded into the prose. See BlockFidelity in internal/perception.
func mapHistoryToChatMessages(systemPrompt string, history []types.Message) ([]chatMessage, error) {
	msgs := make([]chatMessage, 0, len(history)+1)
	if strings.TrimSpace(systemPrompt) != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: systemPrompt})
	}
	for _, m := range history {
		role := m.Role
		if role == "" {
			role = "user"
		}

		var text strings.Builder
		var calls []toolCall
		var results []chatMessage
		droppedThinking := 0

		for _, b := range m.Content() {
			switch b.Kind {
			case types.BlockText:
				text.WriteString(b.Text)

			case types.BlockThinking:
				// Unrepresentable on Chat Completions; see the note above.
				droppedThinking++

			case types.BlockToolUse:
				argsJSON, err := json.Marshal(b.Input)
				if err != nil {
					return nil, fmt.Errorf("marshal tool args for %s: %w", b.Name, err)
				}
				calls = append(calls, toolCall{
					ID:   b.ID,
					Type: "function",
					Function: toolFunction{
						Name:      b.Name,
						Arguments: string(argsJSON),
					},
				})

			case types.BlockToolResult:
				content := b.Text
				if b.IsError && content != "" {
					content = "ERROR: " + content
				}
				results = append(results, chatMessage{
					Role:       "tool",
					Content:    content,
					ToolCallID: b.ToolUseID,
				})
			}
		}

		if droppedThinking > 0 {
			logging.Get(logging.CategoryAPI).Warn(
				"xai-oauth: dropped %d thinking block(s) from a %s turn — Chat Completions has no request-side field for reasoning, so continuity is lost for this turn",
				droppedThinking, role)
		}

		msgs = append(msgs, results...)
		switch {
		case len(calls) > 0:
			msgs = append(msgs, chatMessage{
				Role:      "assistant",
				Content:   text.String(),
				ToolCalls: calls,
			})
		case len(results) > 0:
			if strings.TrimSpace(text.String()) != "" {
				msgs = append(msgs, chatMessage{Role: role, Content: text.String()})
			}
		default:
			msgs = append(msgs, chatMessage{Role: role, Content: text.String()})
		}
	}
	return msgs, nil
}

func parseToolChatResponse(body []byte) (*types.LLMToolResponse, error) {
	var resp chatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return nil, fmt.Errorf("parse tool response: %w", err)
	}
	if resp.Error != nil {
		return nil, fmt.Errorf("API error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	choice := resp.Choices[0]
	internalCalls := make([]types.ToolCall, 0, len(choice.Message.ToolCalls))
	for _, tc := range choice.Message.ToolCalls {
		var args map[string]any
		if tc.Function.Arguments != "" {
			if err := json.Unmarshal([]byte(tc.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("tool args for %s: %w", tc.Function.Name, err)
			}
		}
		internalCalls = append(internalCalls, types.ToolCall{
			ID:    tc.ID,
			Name:  tc.Function.Name,
			Input: args,
		})
	}
	stopReason := choice.FinishReason
	if types.LengthStop(stopReason) {
		produced := 0
		if resp.Usage != nil {
			produced = resp.Usage.CompletionTokens
		}
		return nil, &types.OutputTruncated{
			Provider: "xai-oauth", Method: "CompleteWithTools", Reason: stopReason,
			Partial: choice.Message.Content, OutputTokens: produced,
		}
	}
	if stopReason == "tool_calls" {
		stopReason = "tool_use"
	}
	out := &types.LLMToolResponse{
		Text:       choice.Message.Content,
		ToolCalls:  internalCalls,
		StopReason: stopReason,
	}
	if resp.Usage != nil {
		out.Usage = types.UsageMetadata{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		}
	}
	return out, nil
}

// CompleteWithTools implements types.LLMClient with OpenAI-compatible tool calling.
func (c *Client) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = "You are codeNERD. Respond in English. Be concise."
	}

	c.rateLimitPace()

	token, err := c.tokens.AccessToken(ctx)
	if err != nil {
		if loadErr := c.tokens.Load(); loadErr == nil {
			token, err = c.tokens.AccessToken(ctx)
		}
		if err != nil {
			return nil, err
		}
	}

	chatTools := make([]chatTool, 0, len(tools))
	for _, t := range tools {
		chatTools = append(chatTools, chatTool{
			Type: "function",
			Function: chatToolFunc{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		})
	}

	reqBody := chatRequest{
		Model: c.cfg.Model,
		Messages: []chatMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Tools:       chatTools,
		ToolChoice:  "auto",
		MaxTokens:   8192,
		Temperature: 0.1,
	}

	status, body, err := doJSON(ctx, c.httpClient, "POST", chatURL(c.cfg.BaseURL), token, reqBody, 10<<20)
	if err != nil {
		return nil, err
	}
	if status == 401 {
		c.tokens.InvalidateAccess()
		token, err = c.tokens.AccessToken(ctx)
		if err != nil {
			return nil, err
		}
		status, body, err = doJSON(ctx, c.httpClient, "POST", chatURL(c.cfg.BaseURL), token, reqBody, 10<<20)
		if err != nil {
			return nil, err
		}
	}
	if status != 200 {
		return nil, classifyHTTPError(status, body, nil)
	}

	return parseToolChatResponse(body)
}
