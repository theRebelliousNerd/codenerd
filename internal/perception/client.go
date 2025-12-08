package perception

import (
	"bufio"
	"bytes"
	"codenerd/internal/config"
	"codenerd/internal/usage"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
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
		BaseURL:      "https://api.z.ai/api/coding/paas/v4", // Coding-optimized endpoint
		Model:        "glm-4.6",
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

// ZAIRequest represents the API request structure (Enhanced for v1.2.0).
type ZAIRequest struct {
	Model          string             `json:"model"`
	Messages       []ZAIMessage       `json:"messages"`
	MaxTokens      int                `json:"max_tokens,omitempty"`
	Temperature    float64            `json:"temperature,omitempty"`
	TopP           float64            `json:"top_p,omitempty"`
	Stream         bool               `json:"stream,omitempty"`
	StreamOptions  *ZAIStreamOptions  `json:"stream_options,omitempty"`
	ResponseFormat *ZAIResponseFormat `json:"response_format,omitempty"` // Structured output
	Thinking       *ZAIThinking       `json:"thinking,omitempty"`        // Extended reasoning
}

// ZAIStreamOptions configures streaming behavior.
type ZAIStreamOptions struct {
	IncludeUsage bool `json:"include_usage"`
}

// ZAIResponseFormat enforces structured output (JSON schema).
type ZAIResponseFormat struct {
	Type       string         `json:"type"` // "json_schema"
	JSONSchema *ZAIJSONSchema `json:"json_schema,omitempty"`
}

// ZAIJSONSchema defines the structured output schema.
type ZAIJSONSchema struct {
	Name   string                 `json:"name"`
	Strict bool                   `json:"strict"`
	Schema map[string]interface{} `json:"schema"`
}

// ZAIThinking enables extended reasoning mode.
type ZAIThinking struct {
	Type         string `json:"type"`                    // "enabled"
	BudgetTokens int    `json:"budget_tokens,omitempty"` // Optional token budget
}

// ZAIMessage represents a message in the conversation.
type ZAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ZAIResponse represents the API response structure (Enhanced for v1.2.0).
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
		Delta *struct { // For streaming
			Role    string `json:"role,omitempty"`
			Content string `json:"content,omitempty"`
		} `json:"delta,omitempty"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
		TotalTokens      int `json:"total_tokens"`
		// Thinking mode tokens
		ThinkingTokens int `json:"thinking_tokens,omitempty"`
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
// ENHANCED (v1.2.0): Automatically uses structured output + thinking mode for Piggyback Protocol.
func (c *ZAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Detect if this is a Piggyback Protocol request (contains articulation instructions)
	isPiggyback := strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(systemPrompt, "surface_response") ||
		strings.Contains(userPrompt, "PiggybackEnvelope")

	// Use enhanced method for Piggyback Protocol
	if isPiggyback {
		return c.CompleteWithStructuredOutput(ctx, systemPrompt, userPrompt, true) // Enable thinking
	}

	// Fallback to basic completion for other requests
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

		// Track usage if available
		if tracker := usage.FromContext(ctx); tracker != nil {
			tracker.Track(ctx,
				c.model,
				"zai",
				zaiResp.Usage.PromptTokens,
				zaiResp.Usage.CompletionTokens,
				"chat",
			)
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

// =============================================================================
// Z.AI ENHANCED FEATURES (v1.2.0)
// =============================================================================

// BuildPiggybackEnvelopeSchema creates the JSON schema for structured output.
// This enforces the PiggybackEnvelope format at the API level, eliminating
// JSON parsing errors and guaranteeing thought-first ordering (Bug #14 fix).
func BuildPiggybackEnvelopeSchema() *ZAIResponseFormat {
	return &ZAIResponseFormat{
		Type: "json_schema",
		JSONSchema: &ZAIJSONSchema{
			Name:   "PiggybackEnvelope",
			Strict: true,
			Schema: map[string]interface{}{
				"type": "object",
				"properties": map[string]interface{}{
					"control_packet": map[string]interface{}{
						"type": "object",
						"properties": map[string]interface{}{
							"intent_classification": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"category":   map[string]interface{}{"type": "string"},
									"verb":       map[string]interface{}{"type": "string"},
									"target":     map[string]interface{}{"type": "string"},
									"constraint": map[string]interface{}{"type": "string"},
									"confidence": map[string]interface{}{"type": "number"},
								},
								"required":             []string{"category", "verb", "target", "constraint", "confidence"},
								"additionalProperties": false,
							},
							"mangle_updates": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "string",
								},
							},
							"memory_operations": map[string]interface{}{
								"type": "array",
								"items": map[string]interface{}{
									"type": "object",
									"properties": map[string]interface{}{
										"op":    map[string]interface{}{"type": "string"},
										"key":   map[string]interface{}{"type": "string"},
										"value": map[string]interface{}{"type": "string"},
									},
									"required":             []string{"op", "key", "value"},
									"additionalProperties": false,
								},
							},
							"self_correction": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"triggered":  map[string]interface{}{"type": "boolean"},
									"hypothesis": map[string]interface{}{"type": "string"},
								},
								"required":             []string{"triggered", "hypothesis"},
								"additionalProperties": false,
							},
						},
						"required":             []string{"intent_classification", "mangle_updates", "memory_operations", "self_correction"},
						"additionalProperties": false,
					},
					"surface_response": map[string]interface{}{
						"type": "string",
					},
				},
				"required":             []string{"control_packet", "surface_response"},
				"additionalProperties": false,
			},
		},
	}
}

// CompleteWithStructuredOutput sends a request with JSON schema enforcement.
// This is the preferred method for Piggyback Protocol interactions.
func (c *ZAIClient) CompleteWithStructuredOutput(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}

	if strings.TrimSpace(systemPrompt) == "" {
		systemPrompt = defaultSystemPrompt
	} else {
		systemPrompt = defaultSystemPrompt + "\n" + systemPrompt
	}

	// Rate limiting
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
		Model:          c.model,
		Messages:       messages,
		MaxTokens:      4096,
		Temperature:    0.1,
		TopP:           0.9,
		ResponseFormat: BuildPiggybackEnvelopeSchema(), // Structured output
	}

	// Enable thinking mode if requested
	if enableThinking {
		reqBody.Thinking = &ZAIThinking{
			Type:         "enabled",
			BudgetTokens: 5000, // Allow up to 5K tokens for reasoning
		}
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop
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

		var zaiResp ZAIResponse
		if err := json.Unmarshal(body, &zaiResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if zaiResp.Error != nil {
			return "", fmt.Errorf("API error: %s", zaiResp.Error.Message)
		}

		if len(zaiResp.Choices) == 0 {
			return "", fmt.Errorf("no choices in response")
		}

		return strings.TrimSpace(zaiResp.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("all retries exhausted: %w", lastErr)
}

// CompleteWithStreaming sends a request with streaming enabled.
// Returns a channel that receives content chunks as they arrive.
// The control_packet MUST be buffered and extracted before streaming surface_response.
func (c *ZAIClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	go func() {
		defer close(contentChan)
		defer close(errorChan)

		if c.apiKey == "" {
			errorChan <- fmt.Errorf("API key not configured")
			return
		}

		if strings.TrimSpace(systemPrompt) == "" {
			systemPrompt = defaultSystemPrompt
		} else {
			systemPrompt = defaultSystemPrompt + "\n" + systemPrompt
		}

		// Rate limiting
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
			Model:          c.model,
			Messages:       messages,
			MaxTokens:      4096,
			Temperature:    0.1,
			TopP:           0.9,
			Stream:         true,
			StreamOptions:  &ZAIStreamOptions{IncludeUsage: true},
			ResponseFormat: BuildPiggybackEnvelopeSchema(), // Structured output with streaming
		}

		if enableThinking {
			reqBody.Thinking = &ZAIThinking{
				Type:         "enabled",
				BudgetTokens: 5000,
			}
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

		// Read SSE stream
		scanner := bufio.NewScanner(resp.Body)
		for scanner.Scan() {
			line := scanner.Text()

			// SSE format: "data: {...}"
			if !strings.HasPrefix(line, "data: ") {
				continue
			}

			data := strings.TrimPrefix(line, "data: ")
			if data == "[DONE]" {
				break
			}

			var chunk ZAIResponse
			if err := json.Unmarshal([]byte(data), &chunk); err != nil {
				continue // Skip malformed chunks
			}

			if len(chunk.Choices) > 0 && chunk.Choices[0].Delta != nil {
				content := chunk.Choices[0].Delta.Content
				if content != "" {
					select {
					case contentChan <- content:
					case <-ctx.Done():
						errorChan <- ctx.Err()
						return
					}
				}
			}
		}

		if err := scanner.Err(); err != nil {
			errorChan <- fmt.Errorf("stream error: %w", err)
		}
	}()

	return contentChan, errorChan
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
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
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
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
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
	Contents          []GeminiContent        `json:"contents"`
	SystemInstruction *GeminiContent         `json:"systemInstruction,omitempty"`
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
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
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
// OpenRouter Client (Multi-Provider Gateway)
// ============================================================================

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

// OpenRouterConfig holds configuration for OpenRouter client.
type OpenRouterConfig struct {
	APIKey   string
	BaseURL  string
	Model    string
	Timeout  time.Duration
	SiteURL  string // Optional
	SiteName string // Optional
}

// Popular OpenRouter models (use provider/model format)
// Full list at: https://openrouter.ai/models
var OpenRouterModels = []string{
	// Anthropic
	"anthropic/claude-3.5-sonnet",
	"anthropic/claude-3.5-haiku",
	"anthropic/claude-3-opus",
	// OpenAI
	"openai/gpt-4o",
	"openai/gpt-4o-mini",
	"openai/o1-preview",
	"openai/o1-mini",
	// Google
	"google/gemini-2.0-flash-exp:free",
	"google/gemini-pro-1.5",
	// Meta
	"meta-llama/llama-3.1-405b-instruct",
	"meta-llama/llama-3.1-70b-instruct",
	// Mistral
	"mistralai/mistral-large",
	"mistralai/codestral-latest",
	// DeepSeek
	"deepseek/deepseek-chat",
	"deepseek/deepseek-coder",
	// Qwen
	"qwen/qwen-2.5-72b-instruct",
	"qwen/qwen-2.5-coder-32b-instruct",
}

// DefaultOpenRouterConfig returns sensible defaults.
func DefaultOpenRouterConfig(apiKey string) OpenRouterConfig {
	return OpenRouterConfig{
		APIKey:   apiKey,
		BaseURL:  "https://openrouter.ai/api/v1",
		Model:    "anthropic/claude-3.5-sonnet", // Good default for coding
		Timeout:  120 * time.Second,
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

// OpenRouter uses OpenAI-compatible request/response format
type OpenRouterRequest = OpenAIRequest
type OpenRouterMessage = OpenAIMessage
type OpenRouterResponse = OpenAIResponse

// Complete sends a prompt and returns the completion.
func (c *OpenRouterClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *OpenRouterClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
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
		// OpenRouter-specific headers
		req.Header.Set("HTTP-Referer", c.siteURL)
		req.Header.Set("X-Title", c.siteName)

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

		var orResp OpenRouterResponse
		if err := json.Unmarshal(body, &orResp); err != nil {
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if orResp.Error != nil {
			return "", fmt.Errorf("API error: %s", orResp.Error.Message)
		}

		if len(orResp.Choices) == 0 {
			return "", fmt.Errorf("no completion returned")
		}

		return strings.TrimSpace(orResp.Choices[0].Message.Content), nil
	}

	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// SetModel changes the model used for completions.
func (c *OpenRouterClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *OpenRouterClient) GetModel() string {
	return c.model
}

// ============================================================================
// Provider Selection
// ============================================================================

// Provider represents an LLM provider.
type Provider string

const (
	ProviderZAI        Provider = "zai"
	ProviderAnthropic  Provider = "anthropic"
	ProviderOpenAI     Provider = "openai"
	ProviderGemini     Provider = "gemini"
	ProviderXAI        Provider = "xai"
	ProviderOpenRouter Provider = "openrouter"
)

// ProviderConfig holds the resolved provider and API key.
type ProviderConfig struct {
	Provider       Provider
	APIKey         string
	Model          string // Optional model override
	Context7APIKey string // Context7 API key for research
}

// DefaultConfigPath returns the default path to .nerd/config.json.
// Deprecated: Use config.DefaultUserConfigPath() instead.
func DefaultConfigPath() string {
	return config.DefaultUserConfigPath()
}

// LoadConfigJSON loads provider configuration from a JSON config file.
// This now delegates to the unified config.LoadUserConfig().
func LoadConfigJSON(path string) (*ProviderConfig, error) {
	userCfg, err := config.LoadUserConfig(path)
	if err != nil {
		return nil, err
	}

	// Use the unified config's provider detection
	providerStr, apiKey := userCfg.GetActiveProvider()
	if apiKey == "" {
		return nil, fmt.Errorf("no API key found in config")
	}

	// Context7 API key: check config first, then env var
	context7Key := userCfg.Context7APIKey
	if context7Key == "" {
		context7Key = os.Getenv("CONTEXT7_API_KEY")
	}

	return &ProviderConfig{
		Provider:       Provider(providerStr),
		APIKey:         apiKey,
		Model:          userCfg.Model,
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
		{"OPENROUTER_API_KEY", ProviderOpenRouter},
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

	case ProviderOpenRouter:
		client := NewOpenRouterClient(config.APIKey)
		if config.Model != "" {
			client.SetModel(config.Model)
		}
		return client, nil

	default:
		return nil, fmt.Errorf("unknown provider: %s", config.Provider)
	}
}
