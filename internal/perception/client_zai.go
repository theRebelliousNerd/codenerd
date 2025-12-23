package perception

import (
	"bufio"
	"bytes"
	"codenerd/internal/usage"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

// ZAIClient implements LLMClient for Z.AI API.
type ZAIClient struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
	lastRequest time.Time
	sem         chan struct{} // Concurrency semaphore: Z.AI allows max 5 concurrent requests
	semDisabled bool          // When true, skip semaphore (external scheduler manages concurrency)
}

// DefaultZAIConfig returns sensible defaults.
func DefaultZAIConfig(apiKey string) ZAIConfig {
	return ZAIConfig{
		APIKey:       apiKey,
		BaseURL:      "https://api.z.ai/api/coding/paas/v4", // Coding-optimized endpoint
		Model:        "glm-4.7",
		Timeout:      10 * time.Minute, // GLM-4.7 with 160K+ context needs extended timeout
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
	client := &ZAIClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		semDisabled: config.DisableSemaphore,
	}
	// Only create semaphore if not disabled (external scheduler handles concurrency)
	if !config.DisableSemaphore {
		client.sem = make(chan struct{}, 5) // Z.AI API allows max 5 concurrent requests
	}
	return client
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
		return c.CompleteWithStructuredOutput(ctx, systemPrompt, userPrompt, false) // Disable thinking to prevent blocking timeouts
	}

	// Fallback to basic completion for other requests
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}

	// Acquire concurrency semaphore (max 5 concurrent requests)
	// Skip if disabled (external APIScheduler manages concurrency)
	if !c.semDisabled {
		select {
		case c.sem <- struct{}{}:
			defer func() { <-c.sem }()
		case <-ctx.Done():
			return "", ctx.Err()
		}
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

// CompleteWithStructuredOutput sends a request with JSON schema enforcement.
// This is the preferred method for Piggyback Protocol interactions.
func (c *ZAIClient) CompleteWithStructuredOutput(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (string, error) {
	if c.apiKey == "" {
		return "", fmt.Errorf("API key not configured")
	}

	// Acquire concurrency semaphore (max 5 concurrent requests)
	// Skip if disabled (external APIScheduler manages concurrency)
	if !c.semDisabled {
		select {
		case c.sem <- struct{}{}:
			defer func() { <-c.sem }()
		case <-ctx.Done():
			return "", ctx.Err()
		}
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
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   4096,
		Temperature: 0.1,
		TopP:        0.9,
		// Stream: false (default)
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

		fmt.Printf("DEBUG: Sending ZAI Request (Attempt %d)\n", i+1)
		resp, err := c.httpClient.Do(req)
		if err != nil {
			fmt.Printf("DEBUG: Request failed: %v\n", err)
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		fmt.Printf("DEBUG: Response received (Status %d), reading body...\n", resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			fmt.Printf("DEBUG: Body read failed: %v\n", err)
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}
		fmt.Printf("DEBUG: Body read complete (%d bytes)\n", len(body))

		if resp.StatusCode == http.StatusTooManyRequests {
			lastErr = fmt.Errorf("rate limit exceeded (429)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var zaiResp ZAIResponse
		if err := json.Unmarshal(body, &zaiResp); err != nil {
			// Try to handle potentially malformed response or double-encoding
			return "", fmt.Errorf("failed to parse response: %w (body excerpt: %s)", err, string(body[:min(len(body), 100)]))
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

		// Acquire concurrency semaphore (max 5 concurrent requests)
		// Skip if disabled (external APIScheduler manages concurrency)
		if !c.semDisabled {
			select {
			case c.sem <- struct{}{}:
				defer func() { <-c.sem }()
			case <-ctx.Done():
				errorChan <- ctx.Err()
				return
			}
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

		// Read SSE stream with context cancellation support.
		// The scanner runs in a separate goroutine so we can monitor ctx.Done()
		// and force-close the response body to unblock scanner.Scan() on timeout.
		scanner := bufio.NewScanner(resp.Body)

		// Channel to signal scanner goroutine completion
		scanDone := make(chan struct{})
		// Channel to capture scanner error (buffered to avoid goroutine leak)
		scanErrChan := make(chan error, 1)

		go func() {
			defer close(scanDone)
			for scanner.Scan() {
				line := scanner.Text()

				// SSE format: "data: {...}"
				if !strings.HasPrefix(line, "data: ") {
					continue
				}

				data := strings.TrimPrefix(line, "data: ")
				if data == "[DONE]" {
					return
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
							// Context cancelled while trying to send
							return
						}
					}
				}
			}
			// Capture scanner error for the main goroutine to handle
			if err := scanner.Err(); err != nil {
				scanErrChan <- err
			}
		}()

		// Wait for either scanner completion or context cancellation
		select {
		case <-scanDone:
			// Normal completion - check for scanner errors
			select {
			case err := <-scanErrChan:
				errorChan <- fmt.Errorf("stream error: %w", err)
			default:
				// No error, clean completion
			}
		case <-ctx.Done():
			// Context cancelled - force close response body to unblock scanner.Scan()
			// This is safe because we're in the goroutine that owns resp.Body,
			// and the defer resp.Body.Close() will be a no-op after this.
			resp.Body.Close()
			// Wait briefly for scanner to notice the closed body and exit
			<-scanDone
			errorChan <- ctx.Err()
		}
	}()

	return contentChan, errorChan
}
