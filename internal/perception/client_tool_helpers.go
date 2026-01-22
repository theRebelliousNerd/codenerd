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

		var args map[string]interface{}
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
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			lastErr = fmt.Errorf("rate limit exceeded (429): %s", strings.TrimSpace(string(body)))
			continue
		}

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		body, err := io.ReadAll(resp.Body)
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
