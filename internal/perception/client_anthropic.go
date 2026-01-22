package perception

import (
	"bufio"
	"bytes"
	"codenerd/internal/logging"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// AnthropicClient implements LLMClient for direct Anthropic API.
type AnthropicClient struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

// DefaultAnthropicConfig returns sensible defaults.
func DefaultAnthropicConfig(apiKey string) AnthropicConfig {
	return AnthropicConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.anthropic.com/v1",
		Model:   "claude-sonnet-4-5-20250514",
		Timeout: 10 * time.Minute, // Large context models need extended timeout
	}
}

// NewAnthropicClient creates a new Anthropic client.
func NewAnthropicClient(apiKey string) *AnthropicClient {
	config := DefaultAnthropicConfig(apiKey)
	return NewAnthropicClientWithConfig(config)
}

// NewAnthropicClientWithConfig creates a new Anthropic client with custom config.
func NewAnthropicClientWithConfig(config AnthropicConfig) *AnthropicClient {
	return &AnthropicClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Complete sends a prompt and returns the completion.
func (c *AnthropicClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *AnthropicClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Anthropic] CompleteWithSystem: model=%s system_len=%d user_len=%d", c.model, len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		logging.PerceptionError("[Anthropic] CompleteWithSystem: API key not configured")
		return "", fmt.Errorf("API key not configured")
	}

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}

	isPiggyback := strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(systemPrompt, "surface_response") ||
		strings.Contains(userPrompt, "PiggybackEnvelope") ||
		strings.Contains(userPrompt, "control_packet")

	// Rate limiting
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	reqBody := AnthropicRequest{
		Model:     c.model,
		MaxTokens: 8192, // Higher limit for complex tasks
		System:    systemPrompt,
		Messages: []AnthropicMessage{
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
	}

	// Retry loop for rate limits and transient errors
	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(1<<uint(i-1)) * time.Second)
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			logging.PerceptionError("[Anthropic] CompleteWithSystem: failed to marshal request: %v", err)
			return "", fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(jsonData))
		if err != nil {
			logging.PerceptionError("[Anthropic] CompleteWithSystem: failed to create request: %v", err)
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limit exceeded (429)")
			continue
		}

		if resp.StatusCode == http.StatusBadRequest && isPiggyback {
			// Some requests may fail with schema issues, retry without Piggyback
			bodyStr := string(body)
			if strings.Contains(bodyStr, "schema") || strings.Contains(bodyStr, "json") {
				lastErr = fmt.Errorf("schema validation error: %s", bodyStr)
				continue
			}
		}

		if resp.StatusCode != http.StatusOK {
			logging.PerceptionError("[Anthropic] CompleteWithSystem: API returned status %d", resp.StatusCode)
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var anthropicResp AnthropicResponse
		if err := json.Unmarshal(body, &anthropicResp); err != nil {
			logging.PerceptionError("[Anthropic] CompleteWithSystem: failed to parse response: %v", err)
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if anthropicResp.Error != nil {
			logging.PerceptionError("[Anthropic] CompleteWithSystem: API error: %s", anthropicResp.Error.Message)
			return "", fmt.Errorf("API error: %s", anthropicResp.Error.Message)
		}

		if len(anthropicResp.Content) == 0 {
			logging.PerceptionError("[Anthropic] CompleteWithSystem: no completion returned")
			return "", fmt.Errorf("no completion returned")
		}

		var result strings.Builder
		for _, content := range anthropicResp.Content {
			if content.Type == "text" {
				result.WriteString(content.Text)
			}
		}

		response := strings.TrimSpace(result.String())
		logging.Perception("[Anthropic] CompleteWithSystem: completed in %v response_len=%d", time.Since(startTime), len(response))
		return response, nil
	}

	logging.PerceptionError("[Anthropic] CompleteWithSystem: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// CompleteWithStreaming sends a prompt with streaming enabled.
// Returns channels of incremental content deltas.
func (c *AnthropicClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	logging.PerceptionDebug("[Anthropic] CompleteWithStreaming: starting streaming model=%s", c.model)

	go func() {
		defer close(contentChan)
		defer close(errorChan)

		// Auto-apply timeout if context has no deadline (centralized timeout handling)
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
			defer cancel()
		}

		startTime := time.Now()

		if c.apiKey == "" {
			logging.PerceptionError("[Anthropic] CompleteWithStreaming: API key not configured")
			errorChan <- fmt.Errorf("API key not configured")
			return
		}

		reqBody := AnthropicRequest{
			Model:     c.model,
			MaxTokens: 4096,
			System:    systemPrompt,
			Messages: []AnthropicMessage{
				{Role: "user", Content: userPrompt},
			},
			Temperature: 0.1,
			Stream:      true,
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			errorChan <- fmt.Errorf("failed to marshal request: %w", err)
			return
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(jsonData))
		if err != nil {
			errorChan <- fmt.Errorf("failed to create request: %w", err)
			return
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		req.Header.Set("Accept", "text/event-stream")

		resp, err := c.httpClient.Do(req)
		if err != nil {
			errorChan <- fmt.Errorf("request failed: %w", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			errorChan <- fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
			return
		}

		scanner := bufio.NewScanner(resp.Body)
		scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

		scanDone := make(chan struct{})
		scanErrChan := make(chan error, 1)

		go func() {
			defer close(scanDone)
			for scanner.Scan() {
				line := scanner.Text()
				if !strings.HasPrefix(line, "data:") {
					continue
				}
				data := strings.TrimSpace(strings.TrimPrefix(line, "data:"))
				if data == "" {
					continue
				}
				if data == "[DONE]" {
					return
				}

				var evt struct {
					Type  string `json:"type"`
					Delta *struct {
						Type string `json:"type"`
						Text string `json:"text,omitempty"`
					} `json:"delta,omitempty"`
					Error *struct {
						Type    string `json:"type"`
						Message string `json:"message"`
					} `json:"error,omitempty"`
				}
				if err := json.Unmarshal([]byte(data), &evt); err != nil {
					continue
				}
				if evt.Error != nil {
					scanErrChan <- fmt.Errorf("API error: %s", evt.Error.Message)
					return
				}
				if evt.Type == "content_block_delta" && evt.Delta != nil && evt.Delta.Text != "" {
					select {
					case contentChan <- evt.Delta.Text:
					case <-ctx.Done():
						return
					}
				}
			}
			if err := scanner.Err(); err != nil {
				scanErrChan <- err
			}
		}()

		select {
		case <-scanDone:
			select {
			case err := <-scanErrChan:
				logging.PerceptionError("[Anthropic] CompleteWithStreaming: stream error after %v: %v", time.Since(startTime), err)
				errorChan <- fmt.Errorf("stream error: %w", err)
			default:
				logging.Perception("[Anthropic] CompleteWithStreaming: completed in %v", time.Since(startTime))
			}
		case <-ctx.Done():
			resp.Body.Close()
			<-scanDone
			logging.PerceptionWarn("[Anthropic] CompleteWithStreaming: cancelled after %v", time.Since(startTime))
			errorChan <- ctx.Err()
		}
	}()

	return contentChan, errorChan
}

// CompleteWithTools sends a prompt with tool definitions and returns response with tool calls.
func (c *AnthropicClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	// Auto-apply timeout if context has no deadline
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Anthropic] CompleteWithTools: model=%s tools=%d system_len=%d user_len=%d",
		c.model, len(tools), len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	// Convert tools to Anthropic format
	anthropicTools := make([]AnthropicTool, len(tools))
	for i, t := range tools {
		anthropicTools[i] = AnthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	reqBody := AnthropicRequest{
		Model:       c.model,
		MaxTokens:   8192, // Higher limit for tool use
		System:      systemPrompt,
		Messages:    []AnthropicMessage{{Role: "user", Content: userPrompt}},
		Tools:       anthropicTools,
		Temperature: 0.1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(jsonData))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		logging.PerceptionError("[Anthropic] CompleteWithTools: request failed after %v: %v", time.Since(startTime), err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logging.PerceptionError("[Anthropic] CompleteWithTools: API returned status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", anthropicResp.Error.Message)
	}

	// Parse response content into text and tool calls
	result := &LLMToolResponse{
		StopReason: anthropicResp.StopReason,
	}

	var textBuilder strings.Builder
	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			textBuilder.WriteString(block.Text)
		case "tool_use":
			result.ToolCalls = append(result.ToolCalls, ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}
	result.Text = strings.TrimSpace(textBuilder.String())

	logging.Perception("[Anthropic] CompleteWithTools: completed in %v text_len=%d tool_calls=%d stop_reason=%s",
		time.Since(startTime), len(result.Text), len(result.ToolCalls), result.StopReason)

	return result, nil
}

// SetModel changes the model used for completions.
func (c *AnthropicClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *AnthropicClient) GetModel() string {
	return c.model
}

// SchemaCapable reports whether this client supports response schema enforcement.
// Anthropic doesn't support API-level JSON schema enforcement, but does support
// JSON output via prompt instructions.
func (c *AnthropicClient) SchemaCapable() bool {
	return false
}

// ShouldUsePiggybackTools returns true if this client should use Piggyback Protocol
// for tool invocation instead of native function calling.
// For Anthropic, we use native tool calling which is well-supported.
func (c *AnthropicClient) ShouldUsePiggybackTools() bool {
	return false
}
