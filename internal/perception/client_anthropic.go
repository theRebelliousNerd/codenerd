package perception

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// AnthropicClient implements LLMClient for direct Anthropic API.
type AnthropicClient struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client
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

// CompleteWithStreaming sends a prompt with streaming enabled.
// Returns channels of incremental content deltas.
func (c *AnthropicClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	go func() {
		defer close(contentChan)
		defer close(errorChan)

		if c.apiKey == "" {
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
				errorChan <- fmt.Errorf("stream error: %w", err)
			default:
			}
		case <-ctx.Done():
			resp.Body.Close()
			<-scanDone
			errorChan <- ctx.Err()
		}
	}()

	return contentChan, errorChan
}

// SetModel changes the model used for completions.
func (c *AnthropicClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *AnthropicClient) GetModel() string {
	return c.model
}
