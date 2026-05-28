package perception

import (
	"bytes"
	"codenerd/internal/logging"
	"codenerd/internal/types"
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
	// NERD-EVOLVE-START: P1P2-prompt-caching
	// enableSystemCaching, when true, sends the system prompt as a structured block
	// with cache_control: {"type": "ephemeral"} to enable Anthropic prompt caching.
	// This reduces TTFT by 20-40% for static system prompts (e.g. understandingSystemPrompt).
	enableSystemCaching bool
	// NERD-EVOLVE-END: P1P2-prompt-caching
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
		apiKey:     config.APIKey,
		baseURL:    config.BaseURL,
		model:      config.Model,
		httpClient: NewSharedHTTPClient(config.Timeout),
	}
}

// NERD-EVOLVE-START: P1P2-prompt-caching
// EnableSystemCaching enables Anthropic prompt caching for the system prompt.
// When enabled, CompleteWithSystem wraps the system message in a structured block
// with cache_control: {"type": "ephemeral"}, reducing TTFT by 20-40% for repeated calls
// with the same static system prompt (e.g. the perception understandingSystemPrompt).
// This is a no-op for non-Anthropic providers.
func (c *AnthropicClient) EnableSystemCaching() {
	c.enableSystemCaching = true
}

// buildCachedSystemRequest constructs an anthropicCachedRequest with cache_control
// on the system message. This is used instead of the plain AnthropicRequest when
// enableSystemCaching is true.
func (c *AnthropicClient) buildCachedSystemRequest(base AnthropicRequest) ([]byte, error) {
	cached := anthropicCachedRequest{
		Model:       base.Model,
		MaxTokens:   base.MaxTokens,
		Messages:    base.Messages,
		Tools:       base.Tools,
		Temperature: base.Temperature,
		Stream:      base.Stream,
	}
	if base.System != "" {
		cached.System = []AnthropicSystemCacheBlock{
			{
				Type: "text",
				Text: base.System,
				CacheControl: &AnthropicCacheControl{
					Type: "ephemeral",
				},
			},
		}
	}
	return json.Marshal(cached)
}

// NERD-EVOLVE-END: P1P2-prompt-caching

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

		// NERD-EVOLVE-START: P1P2-prompt-caching
		var jsonData []byte
		var marshalErr error
		if c.enableSystemCaching {
			jsonData, marshalErr = c.buildCachedSystemRequest(reqBody)
		} else {
			jsonData, marshalErr = json.Marshal(reqBody)
		}
		if marshalErr != nil {
			logging.PerceptionError("[Anthropic] CompleteWithSystem: failed to marshal request: %v", marshalErr)
			return "", fmt.Errorf("failed to marshal request: %w", marshalErr)
		}
		// NERD-EVOLVE-END: P1P2-prompt-caching

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(jsonData))
		if err != nil {
			logging.PerceptionError("[Anthropic] CompleteWithSystem: failed to create request: %v", err)
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", "2023-06-01")
		// NERD-EVOLVE-START: P1P2-prompt-caching
		if c.enableSystemCaching {
			// anthropic-beta header required for prompt caching (as of 2024-07)
			req.Header.Set("anthropic-beta", "prompt-caching-2024-07-31")
		}
		// NERD-EVOLVE-END: P1P2-prompt-caching

		resp, err := c.httpClient.Do(req)
		if err != nil {
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
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
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
			errorChan <- fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
			return
		}

		scanner, releaseScanner := newPooledScanner(resp.Body, 1024*1024)
		defer releaseScanner()

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
						Text string `json:"text,omitzero"`
					} `json:"delta,omitzero"`
					Error *struct {
						Type    string `json:"type"`
						Message string `json:"message"`
					} `json:"error,omitzero"`
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

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
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
	return &LLMToolResponse{
		Text:       strings.TrimSpace(textBuilder.String()),
		ToolCalls:  result.ToolCalls,
		StopReason: result.StopReason,
		Usage: types.UsageMetadata{
			InputTokens:  anthropicResp.Usage.InputTokens,
			OutputTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:  anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

// CompleteWithToolResults continues a tool-using conversation. Pass the full
// history (user → assistant tool_use → user tool_result …) and Anthropic
// returns its next response, which may include more tool calls or a final
// answer. This is what makes the agent tool loop actually work — without it,
// tool results never reach the model.
func (c *AnthropicClient) CompleteWithToolResults(ctx context.Context, systemPrompt string, history []types.Message, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Anthropic] CompleteWithToolResults: model=%s tools=%d history=%d",
		c.model, len(tools), len(history))

	if c.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}

	if len(history) == 0 {
		return nil, fmt.Errorf("history must contain at least one message")
	}

	anthropicTools := make([]AnthropicTool, len(tools))
	for i, t := range tools {
		anthropicTools[i] = AnthropicTool{
			Name:        t.Name,
			Description: t.Description,
			InputSchema: t.InputSchema,
		}
	}

	messages, err := buildAnthropicMessagesFromHistory(history)
	if err != nil {
		return nil, fmt.Errorf("invalid history: %w", err)
	}

	reqBody := AnthropicRequest{
		Model:       c.model,
		MaxTokens:   8192,
		System:      systemPrompt,
		Messages:    messages,
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
		logging.PerceptionError("[Anthropic] CompleteWithToolResults: request failed after %v: %v", time.Since(startTime), err)
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		logging.PerceptionError("[Anthropic] CompleteWithToolResults: status %d: %s", resp.StatusCode, string(body))
		return nil, fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}
	if anthropicResp.Error != nil {
		return nil, fmt.Errorf("API error: %s", anthropicResp.Error.Message)
	}

	var textBuilder strings.Builder
	var toolCalls []types.ToolCall
	for _, block := range anthropicResp.Content {
		switch block.Type {
		case "text":
			textBuilder.WriteString(block.Text)
		case "tool_use":
			toolCalls = append(toolCalls, types.ToolCall{
				ID:    block.ID,
				Name:  block.Name,
				Input: block.Input,
			})
		}
	}

	return &types.LLMToolResponse{
		Text:       strings.TrimSpace(textBuilder.String()),
		ToolCalls:  toolCalls,
		StopReason: anthropicResp.StopReason,
		Usage: types.UsageMetadata{
			InputTokens:  anthropicResp.Usage.InputTokens,
			OutputTokens: anthropicResp.Usage.OutputTokens,
			TotalTokens:  anthropicResp.Usage.InputTokens + anthropicResp.Usage.OutputTokens,
		},
	}, nil
}

// buildAnthropicMessagesFromHistory maps the provider-neutral history into
// Anthropic's message shape. Assistant turns combine optional text + tool_use
// blocks. User tool-result turns must use the structured content-block form,
// pairing tool_use_id → result content.
func buildAnthropicMessagesFromHistory(history []types.Message) ([]AnthropicMessage, error) {
	messages := make([]AnthropicMessage, 0, len(history))
	for i, m := range history {
		switch m.Role {
		case "user":
			if len(m.ToolResults) > 0 {
				blocks := make([]AnthropicContentBlock, 0, len(m.ToolResults))
				for _, tr := range m.ToolResults {
					if tr.ToolUseID == "" {
						return nil, fmt.Errorf("user message %d: tool_result missing tool_use_id", i)
					}
					blocks = append(blocks, AnthropicContentBlock{
						Type:      "tool_result",
						ToolUseID: tr.ToolUseID,
						Content:   tr.Content,
						IsError:   tr.IsError,
					})
				}
				messages = append(messages, AnthropicMessage{Role: "user", Content: blocks})
				continue
			}
			messages = append(messages, AnthropicMessage{Role: "user", Content: m.Text})

		case "assistant":
			// Assistant turns may carry both text and tool_use blocks.
			if len(m.ToolCalls) == 0 {
				messages = append(messages, AnthropicMessage{Role: "assistant", Content: m.Text})
				continue
			}
			blocks := make([]AnthropicContentBlock, 0, 1+len(m.ToolCalls))
			if strings.TrimSpace(m.Text) != "" {
				blocks = append(blocks, AnthropicContentBlock{
					Type: "text",
					Text: m.Text,
				})
			}
			for _, tc := range m.ToolCalls {
				blocks = append(blocks, AnthropicContentBlock{
					Type:  "tool_use",
					ID:    tc.ID,
					Name:  tc.Name,
					Input: tc.Input,
				})
			}
			messages = append(messages, AnthropicMessage{Role: "assistant", Content: blocks})

		default:
			return nil, fmt.Errorf("unsupported role %q in history[%d]", m.Role, i)
		}
	}
	return messages, nil
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
