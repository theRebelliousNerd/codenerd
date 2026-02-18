package perception

import (
	"bufio"
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

// OpenRouterClient implements LLMClient for OpenRouter API.
// OpenRouter provides access to multiple LLM providers through a single API.
type OpenRouterClient struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
	lastRequest time.Time
	siteURL     string // Optional: Your site URL for rankings
	siteName    string // Optional: Your app name for rankings
}

// DefaultOpenRouterConfig returns sensible defaults.
func DefaultOpenRouterConfig(apiKey string) OpenRouterConfig {
	return OpenRouterConfig{
		APIKey:   apiKey,
		BaseURL:  "https://openrouter.ai/api/v1",
		Model:    "anthropic/claude-3.5-sonnet", // Good default for coding
		Timeout:  10 * time.Minute,              // Large context models need extended timeout
		SiteName: "codeNERD",
	}
}

// NewOpenRouterClient creates a new OpenRouter client.
func NewOpenRouterClient(apiKey string) *OpenRouterClient {
	config := DefaultOpenRouterConfig(apiKey)
	return NewOpenRouterClientWithConfig(config)
}

// NewOpenRouterClientWithConfig creates a new OpenRouter client with custom config.
func NewOpenRouterClientWithConfig(config OpenRouterConfig) *OpenRouterClient {
	return &OpenRouterClient{
		apiKey:   config.APIKey,
		baseURL:  config.BaseURL,
		model:    config.Model,
		siteURL:  config.SiteURL,
		siteName: config.SiteName,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Complete sends a prompt and returns the completion.
func (c *OpenRouterClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *OpenRouterClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[OpenRouter] CompleteWithSystem: model=%s system_len=%d user_len=%d", c.model, len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		logging.PerceptionError("[OpenRouter] CompleteWithSystem: API key not configured")
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

	messages := []OpenRouterMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody := OpenRouterRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   4096,
		Temperature: 0.1,
	}
	if isPiggyback {
		reqBody.ResponseFormat = BuildOpenRouterPiggybackEnvelopeSchema()
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
		// OpenRouter-specific headers
		req.Header.Set("HTTP-Referer", c.siteURL)
		req.Header.Set("X-Title", c.siteName)

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

		var orResp OpenRouterResponse
		if err := json.Unmarshal(body, &orResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if orResp.Error != nil {
			return "", fmt.Errorf("API error: %s", orResp.Error.Message)
		}

		if len(orResp.Choices) == 0 {
			logging.PerceptionError("[OpenRouter] CompleteWithSystem: no completion returned")
			return "", fmt.Errorf("no completion returned")
		}

		response := strings.TrimSpace(orResp.Choices[0].Message.Content)
		logging.Perception("[OpenRouter] CompleteWithSystem: completed in %v response_len=%d", time.Since(startTime), len(response))
		return response, nil
	}

	logging.PerceptionError("[OpenRouter] CompleteWithSystem: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// CompleteWithStreaming sends a prompt with streaming enabled.
// Returns channels of incremental content deltas.
func (c *OpenRouterClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	logging.PerceptionDebug("[OpenRouter] CompleteWithStreaming: starting streaming model=%s", c.model)

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
			logging.PerceptionError("[OpenRouter] CompleteWithStreaming: API key not configured")
			errorChan <- fmt.Errorf("API key not configured")
			return
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

		messages := []OpenRouterMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		}

		reqBody := OpenRouterRequest{
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
			reqBody.ResponseFormat = BuildOpenRouterPiggybackEnvelopeSchema()
		}

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
			req.Header.Set("HTTP-Referer", c.siteURL)
			req.Header.Set("X-Title", c.siteName)

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

					var chunk OpenRouterResponse
					if err := json.Unmarshal([]byte(data), &chunk); err != nil {
						continue
					}
					if chunk.Error != nil {
						scanErrChan <- fmt.Errorf("API error: %s", chunk.Error.Message)
						return
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

			select {
			case <-scanDone:
				select {
				case err := <-scanErrChan:
					logging.PerceptionError("[OpenRouter] CompleteWithStreaming: stream error after %v: %v", time.Since(startTime), err)
					errorChan <- fmt.Errorf("stream error: %w", err)
				default:
					logging.Perception("[OpenRouter] CompleteWithStreaming: completed in %v", time.Since(startTime))
				}
			case <-ctx.Done():
				resp.Body.Close()
				<-scanDone
				logging.PerceptionWarn("[OpenRouter] CompleteWithStreaming: cancelled after %v", time.Since(startTime))
				errorChan <- ctx.Err()
			}
			return
		}

		logging.PerceptionError("[OpenRouter] CompleteWithStreaming: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
		errorChan <- fmt.Errorf("max retries exceeded: %w", lastErr)
	}()

	return contentChan, errorChan
}

// SetModel changes the model used for completions.
func (c *OpenRouterClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *OpenRouterClient) GetModel() string {
	return c.model
}

// CompleteWithTools sends a prompt with tool definitions.
// CompleteWithTools sends a prompt with tool definitions.
func (c *OpenRouterClient) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	openAITools := MapToolDefinitionsToOpenAI(tools)

	reqBody := OpenAIRequest{
		Model: c.model,
		Messages: []OpenAIMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Tools:      openAITools,
		ToolChoice: "auto",
		Stream:     false,
	}

	resp, err := ExecuteOpenAIRequest(ctx, c.httpClient, c.baseURL, c.apiKey, reqBody)
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

	stopReason := choice.FinishReason
	if stopReason == "tool_calls" {
		stopReason = "tool_use"
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
