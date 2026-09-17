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
// The result aligns index-wise with the input: a skipped non-function call
// leaves a zero-valued entry (pinned by contract test). Blank arguments
// decode as an empty object: several providers serialize zero-arg calls as
// "" instead of "{}", and failing the whole turn on that variance is worse
// than calling with no args.
func MapOpenAIToolCallsToInternal(calls []OpenAIToolCall) ([]ToolCall, error) {
	result := make([]ToolCall, len(calls))
	for i, c := range calls {
		if c.Type != "function" {
			continue // Skip non-function tool calls (if any)
		}

		args := map[string]any{}
		if strings.TrimSpace(c.Function.Arguments) != "" {
			if err := json.Unmarshal([]byte(c.Function.Arguments), &args); err != nil {
				return nil, fmt.Errorf("failed to unmarshal arguments for tool %s: %w", c.Function.Name, err)
			}
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
func MapTypesHistoryToOpenAIMessages(systemPrompt string, history []types.Message) ([]OpenAIMessage, error) {
	msgs := make([]OpenAIMessage, 0, len(history)+1)
	if strings.TrimSpace(systemPrompt) != "" {
		msgs = append(msgs, OpenAIMessage{Role: "system", Content: systemPrompt})
	}
	for _, m := range history {
		switch {
		case len(m.ToolResults) > 0:
			// OpenAI expects one role=tool message per tool_result.
			for _, tr := range m.ToolResults {
				content := tr.Content
				if tr.IsError && content != "" {
					content = "ERROR: " + content
				}
				msgs = append(msgs, OpenAIMessage{
					Role:       "tool",
					Content:    content,
					ToolCallID: tr.ToolUseID,
				})
			}
		case len(m.ToolCalls) > 0:
			oaiCalls := make([]OpenAIToolCall, 0, len(m.ToolCalls))
			for _, tc := range m.ToolCalls {
				argsJSON, err := json.Marshal(tc.Input)
				if err != nil {
					return nil, fmt.Errorf("marshal tool args for %s: %w", tc.Name, err)
				}
				oaiCalls = append(oaiCalls, OpenAIToolCall{
					ID:   tc.ID,
					Type: "function",
					Function: OpenAIFunctionCall{
						Name:      tc.Name,
						Arguments: string(argsJSON),
					},
				})
			}
			msgs = append(msgs, OpenAIMessage{
				Role:      "assistant",
				Content:   m.Text,
				ToolCalls: oaiCalls,
			})
		default:
			role := m.Role
			if role == "" {
				role = "user"
			}
			msgs = append(msgs, OpenAIMessage{Role: role, Content: m.Text})
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
	if types.LengthStop(stopReason) {
		return nil, outputTruncated("", resp.Model, "CompleteWithTools", stopReason, c.Message.Content, 0, resp.Usage.CompletionTokens)
	}
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

// isTransientHTTPStatus reports whether an HTTP status is a transient,
// retryable server-side condition: 408 plus the 5xx family except 501
// (500, 502, 503, 504, 529 "overloaded", and any other 5xx a vendor
// invents). 501 Not Implemented is deterministic — the vendor does not
// support the operation, so retrying identical bytes will never start
// working (pinned by the compat and Gemini contracts).
// This is the single transient policy shared by the OpenAI-compatible
// helper, the OpenAI-compat vendor client, and the native vendor clients.
// One vendor predicate lives outside it: ZAI's 429-inclusive
// retry-after-aware predicate in client_zai_retry.go.
func isTransientHTTPStatus(code int) bool {
	return code == http.StatusRequestTimeout || (code >= 500 && code != http.StatusNotImplemented)
}

// ExecuteOpenAIRequest performs a non-streaming OpenAI-compatible request.
// Used by OpenAI, xAI, OpenRouter clients for tool calls.
func ExecuteOpenAIRequest(ctx context.Context, client *http.Client, baseURL, apiKey string, reqBody OpenAIRequest) (*OpenAIResponse, error) {
	// Retry loop
	maxRetries := 3
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		if attempt > 0 {
			// Context-aware backoff: a cancelled turn must not sleep
			// through up to 7s of retry delays before noticing.
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second
			select {
			case <-ctx.Done():
				return nil, fmt.Errorf("request cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(backoff):
			}
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

		// Transient 5xx (and 408) retry with the same backoff: a single
		// overloaded-model response must not kill the turn. Anything else
		// non-200 (auth, bad request) fails immediately — retrying those
		// only burns quota.
		if isTransientHTTPStatus(resp.StatusCode) {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			resp.Body.Close()
			lastErr = fmt.Errorf("transient status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
			// OpenRouter returns HTTP 200 with an error object when the
			// upstream provider fails mid-request. A numeric code inside a
			// 200 body (e.g. 502, 429) is exactly as transient as the same
			// HTTP status, which the loop above already retries.
			code := openAIResp.Error.Code
			if status, ok := code.HTTPStatus(); ok && (status == http.StatusTooManyRequests || isTransientHTTPStatus(status)) {
				lastErr = fmt.Errorf("transient in-body error %s: %s", code, openAIResp.Error.Message)
				continue
			}
			return nil, fmt.Errorf("API error (code %s): %s", code, openAIResp.Error.Message)
		}

		return &openAIResp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}
