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

// OpenAIClient implements LLMClient for OpenAI API.
type OpenAIClient struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
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
		Timeout: 10 * time.Minute,   // Large context models need extended timeout
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
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
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
				body, _ := io.ReadAll(resp.Body)
				resp.Body.Close()
				lastErr = fmt.Errorf("rate limit exceeded (429): %s", strings.TrimSpace(string(body)))
				continue
			}

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
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

					var chunk OpenAIResponse
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
