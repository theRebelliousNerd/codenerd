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

// OpenAIClient implements LLMClient for OpenAI API.
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client

	// provider is the id usage accounting records this client under. It is a
	// field rather than a constant because OllamaClient reuses this transport
	// against a local endpoint; without it every locally served token would be
	// billed to "openai" in the by-provider breakdown.
	provider Provider

	mu          sync.Mutex
	lastRequest time.Time
}

// OpenAI Codex Models (2025):
// - gpt-5.1-codex-max  : Best for long-horizon, agentic coding tasks
// - gpt-5.1-codex-mini : Smaller, more cost-effective version
// - gpt-5-codex        : Previous generation Codex model
// - gpt-5-codex-mini   : Previous generation smaller model
// Standard models: gpt-4o, gpt-4o-mini, gpt-4-turbo

// DefaultOpenAIConfig returns sensible defaults using Codex.
func DefaultOpenAIConfig(apiKey string) OpenAIConfig {
	return OpenAIConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.openai.com/v1",
		Model:   "gpt-5.1-codex-max", // Best Codex model for coding agents
		Timeout: 10 * time.Minute,    // Large context models need extended timeout
	}
}

// NewOpenAIClient creates a new OpenAI client.
func NewOpenAIClient(apiKey string) *OpenAIClient {
	config := DefaultOpenAIConfig(apiKey)
	return NewOpenAIClientWithConfig(config)
}

// NewOpenAIClientWithConfig creates a new OpenAI client with custom config.
func NewOpenAIClientWithConfig(config OpenAIConfig) *OpenAIClient {
	return &OpenAIClient{
		apiKey:     config.APIKey,
		baseURL:    config.BaseURL,
		model:      config.Model,
		provider:   ProviderOpenAI,
		httpClient: NewSharedHTTPClient(config.Timeout),
	}
}

// Complete sends a prompt and returns the completion.
func (c *OpenAIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *OpenAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[OpenAI] CompleteWithSystem: model=%s system_len=%d user_len=%d", c.model, len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		logging.PerceptionError("[OpenAI] CompleteWithSystem: API key not configured")
		return "", fmt.Errorf("API key not configured")
	}

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}

	isPiggyback := !types.IsStructuredOutputOnlyCtx(ctx) &&
		(strings.Contains(systemPrompt, "control_packet") ||
			strings.Contains(systemPrompt, "surface_response") ||
			strings.Contains(userPrompt, "PiggybackEnvelope") ||
			strings.Contains(userPrompt, "control_packet"))

	// Rate limiting
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	messages := []OpenAIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody := OpenAIRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   4096,
		Temperature: 0.1,
	}
	if isPiggyback {
		reqBody.ResponseFormat = BuildOpenAIPiggybackEnvelopeSchema()
	}

	// Retry loop for rate limits
	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(1<<uint(i-1)) * time.Second)
		}

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request: %w", err)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

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

		if resp.StatusCode != http.StatusOK {
			// Some providers/models reject response_format; retry once without it.
			if isPiggyback && reqBody.ResponseFormat != nil && resp.StatusCode == http.StatusBadRequest {
				bodyStr := string(body)
				if strings.Contains(bodyStr, "response_format") || strings.Contains(bodyStr, "json_schema") {
					reqBody.ResponseFormat = nil
					lastErr = fmt.Errorf("request rejected structured output, retrying without response_format: %s", bodyStr)
					continue
				}
			}
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var openaiResp OpenAIResponse
		if err := json.Unmarshal(body, &openaiResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if openaiResp.Error != nil {
			return "", fmt.Errorf("API error: %s", openaiResp.Error.Message)
		}

		if len(openaiResp.Choices) == 0 {
			logging.PerceptionError("[OpenAI] CompleteWithSystem: no completion returned")
			return "", fmt.Errorf("no completion returned")
		}

		trackUsage(ctx, c.model, c.provider,
			openaiResp.Usage.PromptTokens, openaiResp.Usage.CompletionTokens, usageOpChat)

		response := strings.TrimSpace(openaiResp.Choices[0].Message.Content)
		logging.Perception("[OpenAI] CompleteWithSystem: completed in %v response_len=%d", time.Since(startTime), len(response))
		return response, nil
	}

	logging.PerceptionError("[OpenAI] CompleteWithSystem: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// CompleteWithStreaming sends a prompt with streaming enabled.
// Returns channels of incremental content deltas.
func (c *OpenAIClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	logging.PerceptionDebug("[OpenAI] CompleteWithStreaming: starting streaming model=%s", c.model)

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
			logging.PerceptionError("[OpenAI] CompleteWithStreaming: API key not configured")
			errorChan <- fmt.Errorf("API key not configured")
			return
		}

		if strings.TrimSpace(systemPrompt) == "" {
			systemPrompt = defaultSystemPrompt
		}

		isPiggyback := !types.IsStructuredOutputOnlyCtx(ctx) &&
			(strings.Contains(systemPrompt, "control_packet") ||
				strings.Contains(systemPrompt, "surface_response") ||
				strings.Contains(userPrompt, "PiggybackEnvelope") ||
				strings.Contains(userPrompt, "control_packet"))

		// Rate limiting
		c.mu.Lock()
		elapsed := time.Since(c.lastRequest)
		if elapsed < 100*time.Millisecond {
			time.Sleep(100*time.Millisecond - elapsed)
		}
		c.lastRequest = time.Now()
		c.mu.Unlock()

		messages := []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		}

		reqBody := OpenAIRequest{
			Model:       c.model,
			Messages:    messages,
			MaxTokens:   4096,
			Temperature: 0.1,
			Stream:      true,
			StreamOptions: &OpenAIStreamOptions{
				IncludeUsage: true,
			},
		}
		if isPiggyback {
			reqBody.ResponseFormat = BuildOpenAIPiggybackEnvelopeSchema()
		}

		// Retry loop for initial request setup / rate limits (before streaming begins).
		maxRetries := 3
		var lastErr error

		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				time.Sleep(time.Duration(1<<uint(attempt-1)) * time.Second)
			}

			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				errorChan <- fmt.Errorf("failed to marshal request: %w", err)
				return
			}

			req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
			if err != nil {
				errorChan <- fmt.Errorf("failed to create request: %w", err)
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+c.apiKey)
			req.Header.Set("Accept", "text/event-stream")

			resp, err := c.httpClient.Do(req)
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

				// Some providers/models reject response_format; retry once without it.
				if isPiggyback && reqBody.ResponseFormat != nil && resp.StatusCode == http.StatusBadRequest {
					bodyStr := string(body)
					if strings.Contains(bodyStr, "response_format") || strings.Contains(bodyStr, "json_schema") {
						reqBody.ResponseFormat = nil
						lastErr = fmt.Errorf("request rejected structured output, retrying without response_format: %s", bodyStr)
						continue
					}
				}

				errorChan <- fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
				return
			}

			defer resp.Body.Close()

			scanner, releaseScanner := newPooledScanner(resp.Body, 1024*1024)
			defer releaseScanner()

			scanDone := make(chan struct{})
			scanErrChan := make(chan error, 1)

			// include_usage makes the vendor send one trailing chunk carrying
			// the final billed counts and no choices. Recorded once below,
			// after the stream ends — never per delta.
			var billed struct{ input, output int }

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

					var chunk OpenAIResponse
					if err := json.Unmarshal([]byte(data), &chunk); err != nil {
						continue
					}
					if chunk.Error != nil {
						scanErrChan <- fmt.Errorf("API error: %s", chunk.Error.Message)
						return
					}
					if chunk.Usage.PromptTokens > 0 || chunk.Usage.CompletionTokens > 0 {
						billed.input = chunk.Usage.PromptTokens
						billed.output = chunk.Usage.CompletionTokens
					}
					if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
						delta := chunk.Choices[0].Delta.Content
						if delta != "" {
							select {
							case contentChan <- delta:
							case <-ctx.Done():
								return
							}
						}
					}
				}
				if err := scanner.Err(); err != nil {
					scanErrChan <- err
				}
			}()

			defer func() {
				trackUsage(ctx, c.model, c.provider, billed.input, billed.output, usageOpChat)
			}()

			select {
			case <-scanDone:
				select {
				case err := <-scanErrChan:
					logging.PerceptionError("[OpenAI] CompleteWithStreaming: stream error after %v: %v", time.Since(startTime), err)
					errorChan <- fmt.Errorf("stream error: %w", err)
				default:
					logging.Perception("[OpenAI] CompleteWithStreaming: completed in %v", time.Since(startTime))
				}
			case <-ctx.Done():
				resp.Body.Close()
				<-scanDone
				logging.PerceptionWarn("[OpenAI] CompleteWithStreaming: cancelled after %v", time.Since(startTime))
				errorChan <- ctx.Err()
			}
			return
		}

		logging.PerceptionError("[OpenAI] CompleteWithStreaming: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
		errorChan <- fmt.Errorf("max retries exceeded: %w", lastErr)
	}()

	return contentChan, errorChan
}

// SetModel changes the model used for completions.
func (c *OpenAIClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *OpenAIClient) GetModel() string {
	return c.model
}

// CompleteWithTools sends a prompt with tool definitions.
// CompleteWithTools sends a prompt with tool definitions.
func (c *OpenAIClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	// Map generic tools to OpenAI tools
	openAITools := MapToolDefinitionsToOpenAI(tools)

	reqBody := OpenAIRequest{
		Model: c.model,
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Tools:      openAITools,
		ToolChoice: "auto", // Default to auto when tools are present
		Stream:     false,  // Use non-streaming for simpler/safer tool parsing
	}

	resp, err := c.completeNonStreaming(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	if len(resp.Choices) == 0 {
		return nil, fmt.Errorf("no choices in response")
	}

	choice := resp.Choices[0]
	toolCalls, err := MapOpenAIToolCallsToInternal(choice.Message.ToolCalls)
	if err != nil {
		return nil, err
	}

	// OpenAI uses "tool_calls" as finish_reason when tool calls are present
	stopReason := choice.FinishReason
	if stopReason == "tool_calls" {
		stopReason = "tool_use" // Standardize on "tool_use"
	}

	return &LLMToolResponse{
		Text:       choice.Message.Content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage: types.UsageMetadata{
			InputTokens:  resp.Usage.PromptTokens,
			OutputTokens: resp.Usage.CompletionTokens,
			TotalTokens:  resp.Usage.TotalTokens,
		},
	}, nil
}

// completeNonStreaming performs a non-streaming completion request.
func (c *OpenAIClient) completeNonStreaming(ctx context.Context, reqBody OpenAIRequest) (*OpenAIResponse, error) {
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

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
		if err != nil {
			return nil, fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		// No Accept: text/event-stream for non-streaming

		resp, err := c.httpClient.Do(req)
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
		resp.Body.Close() // Close immediately after read
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

		// Tracked here rather than at each caller: this is the single
		// non-streaming HTTP path on this client, so every billed request is
		// counted exactly once even when a caller retries at a higher level.
		trackUsage(ctx, reqBody.Model, c.provider,
			openAIResp.Usage.PromptTokens, openAIResp.Usage.CompletionTokens, usageOpFor(len(reqBody.Tools)))

		return &openAIResp, nil
	}

	return nil, fmt.Errorf("max retries exceeded: %w", lastErr)
}

// ModelIdentity reports the provider and model this client serves, satisfying
// types.ModelIdentifier. The provider is the recorded one rather than a
// hardcoded "openai": this client also fronts OpenAI-compatible vendors, and
// pinning an atom to the wrong vendor is exactly the leak pinning exists to
// stop.
func (c *OpenAIClient) ModelIdentity() (string, string) {
	provider := string(c.provider)
	if provider == "" {
		provider = string(ProviderOpenAI)
	}
	return provider, c.model
}
