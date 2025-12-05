package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const defaultSystemPrompt = "You are codeNERD. Respond in English. Be concise. When summarizing code, ground answers only in provided text. Do not claim to browse the filesystem or network; only use supplied content."

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
	APIKey       string
	BaseURL      string
	Model        string
	Timeout      time.Duration
	SystemPrompt string
}

// DefaultZAIConfig returns sensible defaults.
func DefaultZAIConfig(apiKey string) ZAIConfig {
	return ZAIConfig{
		APIKey:       apiKey,
		BaseURL:      "https://api.z.ai/api/coding/paas/v4",
		Model:        "GLM-4.6",
		Timeout:      120 * time.Second,
		SystemPrompt: defaultSystemPrompt,
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

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	} else {
		systemPrompt = defaultSystemPrompt + "\n" + systemPrompt
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
		Model:   "claude-sonnet-4-5-20250514",
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

// ============================================================================
// OpenAI Client
// ============================================================================

// OpenAIClient implements LLMClient for OpenAI API.
type OpenAIClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	mu         sync.Mutex
	lastRequest time.Time
}

// OpenAIConfig holds configuration for OpenAI client.
type OpenAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
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
		Timeout: 120 * time.Second,
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

// OpenAIRequest represents the OpenAI API request.
type OpenAIRequest struct {
	Model       string          `json:"model"`
	Messages    []OpenAIMessage `json:"messages"`
	MaxTokens   int             `json:"max_tokens,omitempty"`
	Temperature float64         `json:"temperature,omitempty"`
}

// OpenAIMessage represents a message.
type OpenAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// OpenAIResponse represents the API response.
type OpenAIResponse struct {
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
func (c *OpenAIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *OpenAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.apiKey == "" {
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

		var openaiResp OpenAIResponse
		if err := json.Unmarshal(body, &openaiResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if openaiResp.Error != nil {
			return "", fmt.Errorf("API error: %s", openaiResp.Error.Message)
		}

		if len(openaiResp.Choices) == 0 {
			return "", fmt.Errorf("no completion returned")
		}

		return strings.TrimSpace(openaiResp.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SetModel changes the model used for completions.
func (c *OpenAIClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *OpenAIClient) GetModel() string {
	return c.model
}

// ============================================================================
// Google Gemini Client
// ============================================================================

// GeminiClient implements LLMClient for Google Gemini API.
type GeminiClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	mu         sync.Mutex
	lastRequest time.Time
}

// GeminiConfig holds configuration for Gemini client.
type GeminiConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// DefaultGeminiConfig returns sensible defaults.
func DefaultGeminiConfig(apiKey string) GeminiConfig {
	return GeminiConfig{
		APIKey:  apiKey,
		BaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Model:   "gemini-3-pro-preview",
		Timeout: 120 * time.Second,
	}
}

// NewGeminiClient creates a new Gemini client.
func NewGeminiClient(apiKey string) *GeminiClient {
	config := DefaultGeminiConfig(apiKey)
	return NewGeminiClientWithConfig(config)
}

// NewGeminiClientWithConfig creates a new Gemini client with custom config.
func NewGeminiClientWithConfig(config GeminiConfig) *GeminiClient {
	return &GeminiClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
	}
}

// GeminiRequest represents the Gemini API request.
type GeminiRequest struct {
	Contents         []GeminiContent        `json:"contents"`
	SystemInstruction *GeminiContent        `json:"systemInstruction,omitempty"`
	GenerationConfig  GeminiGenerationConfig `json:"generationConfig,omitempty"`
}

// GeminiContent represents content in the request.
type GeminiContent struct {
	Role  string       `json:"role,omitempty"`
	Parts []GeminiPart `json:"parts"`
}

// GeminiPart represents a part of the content.
type GeminiPart struct {
	Text string `json:"text"`
}

// GeminiGenerationConfig represents generation parameters.
type GeminiGenerationConfig struct {
	Temperature     float64 `json:"temperature,omitempty"`
	MaxOutputTokens int     `json:"maxOutputTokens,omitempty"`
}

// GeminiResponse represents the API response.
type GeminiResponse struct {
	Candidates []struct {
		Content struct {
			Parts []struct {
				Text string `json:"text"`
			} `json:"parts"`
			Role string `json:"role"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
	UsageMetadata struct {
		PromptTokenCount     int `json:"promptTokenCount"`
		CandidatesTokenCount int `json:"candidatesTokenCount"`
		TotalTokenCount      int `json:"totalTokenCount"`
	} `json:"usageMetadata"`
	Error *struct {
		Code    int    `json:"code"`
		Message string `json:"message"`
		Status  string `json:"status"`
	} `json:"error,omitempty"`
}

// Complete sends a prompt and returns the completion.
func (c *GeminiClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *GeminiClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.apiKey == "" {
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

	reqBody := GeminiRequest{
		Contents: []GeminiContent{
			{
				Role:  "user",
				Parts: []GeminiPart{{Text: userPrompt}},
			},
		},
		SystemInstruction: &GeminiContent{
			Parts: []GeminiPart{{Text: systemPrompt}},
		},
		GenerationConfig: GeminiGenerationConfig{
			Temperature:     0.1,
			MaxOutputTokens: 4096,
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Construct URL with API key
	url := fmt.Sprintf("%s/models/%s:generateContent?key=%s", c.baseURL, c.model, c.apiKey)

	// Retry loop for rate limits
	maxRetries := 3
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			time.Sleep(time.Duration(1<<uint(i-1)) * time.Second)
		}

		req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
		if err != nil {
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")

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

		var geminiResp GeminiResponse
		if err := json.Unmarshal(body, &geminiResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if geminiResp.Error != nil {
			return "", fmt.Errorf("API error: %s", geminiResp.Error.Message)
		}

		if len(geminiResp.Candidates) == 0 || len(geminiResp.Candidates[0].Content.Parts) == 0 {
			return "", fmt.Errorf("no completion returned")
		}

		var result strings.Builder
		for _, part := range geminiResp.Candidates[0].Content.Parts {
			result.WriteString(part.Text)
		}

		return strings.TrimSpace(result.String()), nil
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SetModel changes the model used for completions.
func (c *GeminiClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *GeminiClient) GetModel() string {
	return c.model
}

// ============================================================================
// xAI (Grok) Client
// ============================================================================

// XAIClient implements LLMClient for xAI (Grok) API.
type XAIClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
	mu         sync.Mutex
	lastRequest time.Time
}

// XAIConfig holds configuration for xAI client.
type XAIConfig struct {
	APIKey  string
	BaseURL string
	Model   string
	Timeout time.Duration
}

// DefaultXAIConfig returns sensible defaults.
func DefaultXAIConfig(apiKey string) XAIConfig {
	return XAIConfig{
		APIKey:  apiKey,
		BaseURL: "https://api.x.ai/v1",
		Model:   "grok-2-latest",
		Timeout: 120 * time.Second,
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

// XAI uses OpenAI-compatible API format
type XAIRequest = OpenAIRequest
type XAIMessage = OpenAIMessage
type XAIResponse = OpenAIResponse

// Complete sends a prompt and returns the completion.
func (c *XAIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *XAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	if c.apiKey == "" {
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
			return "", fmt.Errorf("no completion returned")
		}

		return strings.TrimSpace(xaiResp.Choices[0].Message.Content), nil
	}

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

// ============================================================================
// Provider Selection
// ============================================================================

// Provider represents an LLM provider.
type Provider string

const (
	ProviderZAI       Provider = "zai"
	ProviderAnthropic Provider = "anthropic"
	ProviderOpenAI    Provider = "openai"
	ProviderGemini    Provider = "gemini"
	ProviderXAI       Provider = "xai"
)

// ProviderConfig holds the resolved provider and API key.
type ProviderConfig struct {
	Provider       Provider
	APIKey         string
	Model          string // Optional model override
	Context7APIKey string // Context7 API key for research
}

// DefaultConfigPath returns the default path to .nerd/config.json.
func DefaultConfigPath() string {
	cwd, err := os.Getwd()
	if err != nil {
		return filepath.Join(".nerd", "config.json")
	}
	return filepath.Join(cwd, ".nerd", "config.json")
}

// LoadConfigJSON loads provider configuration from a JSON config file.
func LoadConfigJSON(path string) (*ProviderConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var cfg struct {
		Provider        string `json:"provider"`
		APIKey          string `json:"api_key"`
		AnthropicAPIKey string `json:"anthropic_api_key"`
		OpenAIAPIKey    string `json:"openai_api_key"`
		GeminiAPIKey    string `json:"gemini_api_key"`
		XAIAPIKey       string `json:"xai_api_key"`
		ZAIAPIKey       string `json:"zai_api_key"`
		Model           string `json:"model"`
		Context7APIKey  string `json:"context7_api_key"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	// Determine provider and key
	var provider Provider
	var apiKey string

	// If provider is explicitly set, use that
	if cfg.Provider != "" {
		provider = Provider(cfg.Provider)
		switch provider {
		case ProviderAnthropic:
			apiKey = cfg.AnthropicAPIKey
		case ProviderOpenAI:
			apiKey = cfg.OpenAIAPIKey
		case ProviderGemini:
			apiKey = cfg.GeminiAPIKey
		case ProviderXAI:
			apiKey = cfg.XAIAPIKey
		case ProviderZAI:
			apiKey = cfg.ZAIAPIKey
		}
	}

	// If no key found for explicit provider, check all keys in priority order
	if apiKey == "" {
		if cfg.AnthropicAPIKey != "" {
			provider = ProviderAnthropic
			apiKey = cfg.AnthropicAPIKey
		} else if cfg.OpenAIAPIKey != "" {
			provider = ProviderOpenAI
			apiKey = cfg.OpenAIAPIKey
		} else if cfg.GeminiAPIKey != "" {
			provider = ProviderGemini
			apiKey = cfg.GeminiAPIKey
		} else if cfg.XAIAPIKey != "" {
			provider = ProviderXAI
			apiKey = cfg.XAIAPIKey
		} else if cfg.ZAIAPIKey != "" {
			provider = ProviderZAI
			apiKey = cfg.ZAIAPIKey
		} else if cfg.APIKey != "" {
			// Legacy: single api_key field (assume zai)
			provider = ProviderZAI
			apiKey = cfg.APIKey
		}
	}

	if apiKey == "" {
		return nil, fmt.Errorf("no API key found in config")
	}

	// Context7 API key: check config first, then env var
	context7Key := cfg.Context7APIKey
	if context7Key == "" {
		context7Key = os.Getenv("CONTEXT7_API_KEY")
	}

	return &ProviderConfig{
		Provider:       provider,
		APIKey:         apiKey,
		Model:          cfg.Model,
		Context7APIKey: context7Key,
	}, nil
}

// DetectProvider checks .nerd/config.json first, then environment variables.
// Priority: config.json > env vars (ANTHROPIC > OPENAI > GEMINI > XAI > ZAI)
func DetectProvider() (*ProviderConfig, error) {
	// First, try to load from .nerd/config.json
	configPath := DefaultConfigPath()
	if cfg, err := LoadConfigJSON(configPath); err == nil && cfg.APIKey != "" {
		return cfg, nil
	}

	// Fall back to environment variables
	providers := []struct {
		envVar   string
		provider Provider
	}{
		{"ANTHROPIC_API_KEY", ProviderAnthropic},
		{"OPENAI_API_KEY", ProviderOpenAI},
		{"GEMINI_API_KEY", ProviderGemini},
		{"XAI_API_KEY", ProviderXAI},
		{"ZAI_API_KEY", ProviderZAI},
	}

	for _, p := range providers {
		if key := os.Getenv(p.envVar); key != "" {
			return &ProviderConfig{
				Provider: p.provider,
				APIKey:   key,
			}, nil
		}
	}

	return nil, fmt.Errorf("no API key found; configure .nerd/config.json or set one of: ANTHROPIC_API_KEY, OPENAI_API_KEY, GEMINI_API_KEY, XAI_API_KEY, ZAI_API_KEY")
}

// NewClientFromEnv creates an LLM client based on config file or environment variables.
func NewClientFromEnv() (LLMClient, error) {
	config, err := DetectProvider()
	if err != nil {
		return nil, err
	}
	return NewClientFromConfig(config)
}

// NewClientFromConfig creates an LLM client from a provider config.
func NewClientFromConfig(config *ProviderConfig) (LLMClient, error) {
	switch config.Provider {
	case ProviderAnthropic:
		client := NewAnthropicClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderOpenAI:
		client := NewOpenAIClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderGemini:
		client := NewGeminiClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderXAI:
		client := NewXAIClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	case ProviderZAI:
		client := NewZAIClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", config.Provider)
	}
}
