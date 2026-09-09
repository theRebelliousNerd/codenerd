package xaioauth

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// OpenAI-compatible request/response types local to this package (no XAIClient coupling).

type chatMessage struct {
	Role string `json:"role"`
	// Content must not use omitempty: xAI rejects assistant tool_call turns
	// that omit the content field (HTTP 422 missing field `content`).
	Content    string     `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string       `json:"id"`
	Type     string       `json:"type"`
	Function toolFunction `json:"function"`
}

type toolFunction struct {
	Name      string `json:"name"`
	Arguments string `json:"arguments"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	Stream      bool          `json:"stream,omitempty"`
	Tools       []chatTool    `json:"tools,omitempty"`
	ToolChoice  any           `json:"tool_choice,omitempty"`
}

type chatTool struct {
	Type     string       `json:"type"`
	Function chatToolFunc `json:"function"`
}

type chatToolFunc struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Parameters  any    `json:"parameters,omitempty"`
}

type chatResponse struct {
	Choices []struct {
		Message      chatMessage `json:"message"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage *struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage,omitempty"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error,omitempty"`
}

// Complete implements types.LLMClient.
func (c *Client) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem implements types.LLMClient.
func (c *Client) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.cfg.Timeout)
		defer cancel()
	}

	start := time.Now()
	logging.PerceptionDebug("[XAI-OAuth] CompleteWithSystem: model=%s system_len=%d user_len=%d",
		c.cfg.Model, len(systemPrompt), len(userPrompt))

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = "You are codeNERD. Respond in English. Be concise. When summarizing code, ground answers only in provided text."
	}

	c.rateLimitPace()

	messages := []chatMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	text, err := c.chatOnce(ctx, c.cfg.Model, messages, nil)
	if err != nil {
		// Fallback model on rate limit
		if _, ok := err.(*RateLimitedError); ok && c.cfg.FallbackModel != "" && c.cfg.FallbackModel != c.cfg.Model {
			logging.PerceptionWarn("[XAI-OAuth] primary rate limited; trying fallback model=%s", c.cfg.FallbackModel)
			text, err = c.chatOnce(ctx, c.cfg.FallbackModel, messages, nil)
		}
	}
	if err != nil {
		logging.PerceptionError("[XAI-OAuth] CompleteWithSystem failed after %v: %v", time.Since(start), err)
		return "", err
	}
	logging.Perception("[XAI-OAuth] CompleteWithSystem: completed in %v response_len=%d", time.Since(start), len(text))
	return text, nil
}

func (c *Client) chatOnce(ctx context.Context, model string, messages []chatMessage, tools []chatTool) (string, error) {
	token, err := c.tokens.AccessToken(ctx)
	if err != nil {
		// One reload attempt (e.g. credentials written after construction)
		if loadErr := c.tokens.Load(); loadErr == nil {
			token, err = c.tokens.AccessToken(ctx)
		}
		if err != nil {
			return "", err
		}
	}

	reqBody := chatRequest{
		Model:       model,
		Messages:    messages,
		MaxTokens:   8192,
		Temperature: 0.1,
		Stream:      false,
	}
	if len(tools) > 0 {
		reqBody.Tools = tools
		reqBody.ToolChoice = "auto"
	}

	status, body, err := doJSON(ctx, c.httpClient, "POST", chatURL(c.cfg.BaseURL), token, reqBody, 10<<20)
	if err != nil {
		return "", err
	}

	if status == 401 {
		// Force refresh and retry once
		c.tokens.InvalidateAccess()
		token, err = c.tokens.AccessToken(ctx)
		if err != nil {
			return "", err
		}
		status, body, err = doJSON(ctx, c.httpClient, "POST", chatURL(c.cfg.BaseURL), token, reqBody, 10<<20)
		if err != nil {
			return "", err
		}
	}

	if status != 200 {
		return "", classifyHTTPError(status, body, nil)
	}

	var resp chatResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		return "", fmt.Errorf("parse chat response: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("API error: %s", resp.Error.Message)
	}
	if len(resp.Choices) == 0 {
		return "", fmt.Errorf("no completion returned")
	}
	return strings.TrimSpace(resp.Choices[0].Message.Content), nil
}
