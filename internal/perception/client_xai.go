package perception

import (
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

// XAIClient implements LLMClient for xAI (Grok) API.
type XAIClient struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

// DefaultXAIConfig returns sensible defaults.
func DefaultXAIConfig(apiKey string) XAIConfig {
	return XAIConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.x.ai/v1",
		Model:   "grok-2-latest",
		Timeout: 10 * time.Minute, // Large context models need extended timeout
	}
}

// NewXAIClient creates a new xAI client.
func NewXAIClient(apiKey string) *XAIClient {
	config := DefaultXAIConfig(apiKey)
	return NewXAIClientWithConfig(config)
}

// NewXAIClientWithConfig creates a new xAI client with custom config.
func NewXAIClientWithConfig(config XAIConfig) *XAIClient {
	return &XAIClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// Complete sends a prompt and returns the completion.
func (c *XAIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *XAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[XAI] CompleteWithSystem: model=%s system_len=%d user_len=%d", c.model, len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		logging.PerceptionError("[XAI] CompleteWithSystem: API key not configured")
		return "", fmt.Errorf("API key not configured")
	}

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	}

	// Rate limiting
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	messages := []XAIMessage{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	reqBody := XAIRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   4096,
		Temperature: 0.1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop for rate limits
	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(1<<uint(i-1)) * time.Second)
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
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var xaiResp XAIResponse
		if err := json.Unmarshal(body, &xaiResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if xaiResp.Error != nil {
			return "", fmt.Errorf("API error: %s", xaiResp.Error.Message)
		}

		if len(xaiResp.Choices) == 0 {
			logging.PerceptionError("[XAI] CompleteWithSystem: no completion returned")
			return "", fmt.Errorf("no completion returned")
		}

		response := strings.TrimSpace(xaiResp.Choices[0].Message.Content)
		logging.Perception("[XAI] CompleteWithSystem: completed in %v response_len=%d", time.Since(startTime), len(response))
		return response, nil
	}

	logging.PerceptionError("[XAI] CompleteWithSystem: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SetModel changes the model used for completions.
func (c *XAIClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *XAIClient) GetModel() string {
	return c.model
}
