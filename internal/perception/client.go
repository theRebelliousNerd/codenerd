package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// LLMClient defines the interface for LLM providers.
type LLMClient interface {
	Complete(ctx context.Context, prompt string) (string, error)
	CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error)
}

// ZAIClient implements LLMClient for Z.AI API.
type ZAIClient struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

// ZAIConfig holds configuration for ZAI client.
type ZAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// DefaultZAIConfig returns sensible defaults.
func DefaultZAIConfig(apiKey string) ZAIConfig {
	return ZAIConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.zukijourney.com/v1",
		Model:   "claude-sonnet-4-20250514",
		Timeout: 120 * time.Second,
	}
}

// NewZAIClient creates a new ZAI client with default config.
func NewZAIClient(apiKey string) *ZAIClient {
	config := DefaultZAIConfig(apiKey)
	return NewZAIClientWithConfig(config)
}

// NewZAIClientWithConfig creates a new ZAI client with custom config.
func NewZAIClientWithConfig(config ZAIConfig) *ZAIClient {
	return &ZAIClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// ZAIRequest represents the API request structure.
type ZAIRequest struct {
	Model       string       `json:"model"`
	Messages    []ZAIMessage `json:"messages"`
	MaxTokens   int          `json:"max_tokens,omitempty"`
	Temperature float64      `json:"temperature,omitempty"`
}

// ZAIMessage represents a message in the conversation.
type ZAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ZAIResponse represents the API response structure.
type ZAIResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
		Code    string `json:"code"`
	} `json:"error,omitempty"`
}

// Complete sends a prompt and returns the completion.
func (c *ZAIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *ZAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}

	// Rate limiting: Ensure at least 500ms between requests
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < 600*time.Millisecond {
		time.Sleep(600*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()

	messages := make([]ZAIMessage, 0)

	if systemPrompt != "" {
		messages = append(messages, ZAIMessage{
			Role:    "system",
			Content: systemPrompt,
		})
	}

	messages = append(messages, ZAIMessage{
		Role:    "user",
		Content: userPrompt,
	})

	reqBody := ZAIRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   4096,
		Temperature: 0.1, // Low temperature for structured output
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop for 429 errors
	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Exponential backoff: 1s, 2s, 4s
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

		var zaiResp ZAIResponse
		if err := json.Unmarshal(body, &zaiResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if zaiResp.Error != nil {
			return "", fmt.Errorf("API error: %s", zaiResp.Error.Message)
		}

		if len(zaiResp.Choices) == 0 {
			return "", fmt.Errorf("no completion returned")
		}

		return strings.TrimSpace(zaiResp.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SetModel changes the model used for completions.
func (c *ZAIClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *ZAIClient) GetModel() string {
	return c.model
}

// AnthropicClient implements LLMClient for direct Anthropic API.
type AnthropicClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
}

// AnthropicConfig holds configuration for Anthropic client.
type AnthropicConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// DefaultAnthropicConfig returns sensible defaults.
func DefaultAnthropicConfig(apiKey string) AnthropicConfig {
	return AnthropicConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.anthropic.com/v1",
		Model:   "claude-sonnet-4-20250514",
		Timeout: 120 * time.Second,
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

// AnthropicRequest represents the Anthropic API request.
type AnthropicRequest struct {
	Model       string             `json:"model"`
	MaxTokens   int                `json:"max_tokens"`
	System      string             `json:"system,omitempty"`
	Messages    []AnthropicMessage `json:"messages"`
	Temperature float64            `json:"temperature,omitempty"`
}

// AnthropicMessage represents a message.
type AnthropicMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// AnthropicResponse represents the API response.
type AnthropicResponse struct {
	ID      string `json:"id"`
	Type    string `json:"type"`
	Role    string `json:"role"`
	Content []struct {
		Type string `json:"type"`
		Text string `json:"text"`
	} `json:"content"`
	Model        string `json:"model"`
	StopReason   string `json:"stop_reason"`
	StopSequence string `json:"stop_sequence"`
	Usage        struct {
		InputTokens  int `json:"input_tokens"`
		OutputTokens int `json:"output_tokens"`
	} `json:"usage"`
	Error *struct {
		Type    string `json:"type"`
		Message string `json:"message"`
	} `json:"error,omitempty"`
}

// Complete sends a prompt and returns the completion.
func (c *AnthropicClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *AnthropicClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}

	reqBody := AnthropicRequest{
		Model:     c.model,
		MaxTokens: 4096,
		System:    systemPrompt,
		Messages: []AnthropicMessage{
			{Role: "user", Content: userPrompt},
		},
		Temperature: 0.1,
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/messages", bytes.NewReader(jsonData))
	if err != nil {
		return "", fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
	}

	var anthropicResp AnthropicResponse
	if err := json.Unmarshal(body, &anthropicResp); err != nil {
		return "", fmt.Errorf("failed to parse response: %w", err)
	}

	if anthropicResp.Error != nil {
		return "", fmt.Errorf("API error: %s", anthropicResp.Error.Message)
	}

	if len(anthropicResp.Content) == 0 {
		return "", fmt.Errorf("no completion returned")
	}

	var result strings.Builder
	for _, content := range anthropicResp.Content {
		if content.Type == "text" {
			result.WriteString(content.Text)
		}
	}

	return strings.TrimSpace(result.String()), nil
}

// SetModel changes the model used for completions.
func (c *AnthropicClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *AnthropicClient) GetModel() string {
	return c.model
}
