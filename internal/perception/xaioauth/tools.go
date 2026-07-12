package xaioauth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"codenerd/internal/types"
)

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
