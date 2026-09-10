package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"codenerd/internal/types"
)

// MapToolDefinitionsToOpenAI converts generic tool definitions to OpenAI-compatible format.
func MapToolDefinitionsToOpenAI(tools []ToolDefinition) []OpenAITool {
	result := make([]OpenAITool, len(tools))
	for i, t := range tools {
		result[i] = OpenAITool{
			Type: "function",
			Function: OpenAIFunction{
				Name:        t.Name,
				Description: t.Description,
				Parameters:  t.InputSchema,
			},
		}
	}
	return result
}

// MapOpenAIToolCallsToInternal converts OpenAI tool calls to generic tool calls.
func MapOpenAIToolCallsToInternal(calls []OpenAIToolCall) ([]ToolCall, error) {
	result := make([]ToolCall, len(calls))
	for i, c := range calls {
		if c.Type != "function" {
			continue // Skip non-function tool calls (if any)
		}

		var args map[string]any
		if err := json.Unmarshal([]byte(c.Function.Arguments), &args); err != nil {
			return nil, fmt.Errorf("failed to unmarshal arguments for tool %s: %w", c.Function.Name, err)
		}

		result[i] = ToolCall{
			ID:    c.ID,
			Name:  c.Function.Name,
			Input: args,
		}
	}
	return result, nil
}

// MapTypesHistoryToOpenAIMessages converts the multi-turn tool-calling history
// used by types.ToolResultsProvider into OpenAI-compatible chat messages.
//
// Expected history shape (from session executor):
//
//	user(text) → assistant(tool_calls) → user(tool_results) → assistant(...) …
//
// # What this surface cannot represent
//
// Chat Completions gives an assistant turn one content string and one
// tool_calls array. There is no place to say that prose came BETWEEN two tool
// calls, so a turn of text → tool_use → text → tool_use is flattened to all
// the text, then both calls. That is a limit of the wire format, not of the
// block list: the ordering is read faithfully here and then collapsed on
// purpose, because the alternative — splitting the turn into several assistant
// messages — puts a tool_calls message somewhere other than immediately before
// its tool results, which the endpoint rejects.
//
// Thinking blocks are dropped for the same kind of reason and it is worth
// being precise about it: Chat Completions has no request-side field for
// reasoning at all. Some vendors (DashScope/Qwen) RETURN reasoning_content;
// none of them accept it back. A thinking block replayed here would have
// nowhere to go, so it is counted out at the boundary rather than smuggled
// into the content string, where it would corrupt structured-output parses.
// See BlockFidelity in provider_fidelity.go for the machine-readable form.
func MapTypesHistoryToOpenAIMessages(systemPrompt string, history []types.Message) ([]OpenAIMessage, error) {
	msgs := make([]OpenAIMessage, 0, len(history)+1)
	if strings.TrimSpace(systemPrompt) != "" {
		msgs = append(msgs, OpenAIMessage{Role: "system", Content: systemPrompt})
	}
	for _, m := range history {
		role := m.Role
		if role == "" {
			role = "user"
		}

		var text strings.Builder
		var calls []OpenAIToolCall
		var results []OpenAIMessage

		for _, b := range m.Content() {
			switch b.Kind {
			case types.BlockText:
				text.WriteString(b.Text)

			case types.BlockThinking:
				// Unrepresentable here; see the note above.

			case types.BlockToolUse:
				argsJSON, err := json.Marshal(b.Input)
				if err != nil {
					return nil, fmt.Errorf("marshal tool args for %s: %w", b.Name, err)
				}
				calls = append(calls, OpenAIToolCall{
					ID:   b.ID,
					Type: "function",
					Function: OpenAIFunctionCall{
						Name:      b.Name,
						Arguments: string(argsJSON),
					},
				})

			case types.BlockToolResult:
				// OpenAI expects one role=tool message per tool_result.
				content := b.Text
				if b.IsError && content != "" {
					content = "ERROR: " + content
				}
				results = append(results, OpenAIMessage{
					Role:       "tool",
					Content:    content,
					ToolCallID: b.ToolUseID,
				})
			}
		}

		msgs = append(msgs, results...)
		switch {
		case len(calls) > 0:
			msgs = append(msgs, OpenAIMessage{
				Role:      "assistant",
				Content:   text.String(),
				ToolCalls: calls,
			})
		case len(results) > 0:
			// A turn that is nothing but tool results adds no further message.
			// Text alongside them is rare and used to be discarded; it is now
			// emitted after the results, where it reads as the user's own
			// comment on them.
			if strings.TrimSpace(text.String()) != "" {
				msgs = append(msgs, OpenAIMessage{Role: role, Content: text.String()})
			}
		default:
			msgs = append(msgs, OpenAIMessage{Role: role, Content: text.String()})
		}
	}
	return msgs, nil
}

// OpenAIToolResponseFromResponse maps an OpenAI-compatible chat response into
// the internal LLMToolResponse used by the session executor tool loop.
func OpenAIToolResponseFromResponse(resp *OpenAIResponse) (*LLMToolResponse, error) {
	if resp == nil || len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}
	c := resp.Choices[0]
	toolCalls, err := MapOpenAIToolCallsToInternal(c.Message.ToolCalls)
	if err != nil {
		return nil, err
	}
	stopReason := c.FinishReason
	if stopReason == "tool_calls" {
		stopReason = "tool_use"
	}
	return &LLMToolResponse{
		Text:       c.Message.Content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage: types.UsageMetadata{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}

// ExecuteOpenAIRequest performs a non-streaming OpenAI-compatible request.
// Used by OpenAI, xAI, OpenRouter clients for tool calls.
func ExecuteOpenAIRequest(ctx context.Context, client *http.Client, baseURL, apiKey string, reqBody OpenAIRequest) (*OpenAIResponse, error) {
	// Retry loop
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", baseURL+"/chat/completions", bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)

		resp, err := client.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limit exceeded (429): %s", strings.TrimSpace(string(body)))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			resp.Body.Close()
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to read response body: %w", err)
		}

		var openAIResp OpenAIResponse
		if err := json.Unmarshal(body, &openAIResp); err != nil {
			return nil, fmt.Errorf("failed to unmarshal response: %w", err)
		}

		if openAIResp.Error != nil {
			return nil, fmt.Errorf("API error: %s", openAIResp.Error.Message)
		}

		return &openAIResp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
