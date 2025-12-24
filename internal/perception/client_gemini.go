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

// GeminiClient implements LLMClient for Google Gemini API.
type GeminiClient struct {
	apiKey      string
	baseURL     string
	model       string
	httpClient  *http.Client
	mu          sync.Mutex
	lastRequest time.Time
}

// DefaultGeminiConfig returns sensible defaults.
func DefaultGeminiConfig(apiKey string) GeminiConfig {
	return GeminiConfig{
		APIKey:  apiKey,
		BaseURL: "https://generativelanguage.googleapis.com/v1beta",
		Model:   "gemini-3-pro-preview",
		Timeout: 10 * time.Minute, // Large context models need extended timeout
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

// Complete sends a prompt and returns the completion.
func (c *GeminiClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *GeminiClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	startTime := time.Now()
	logging.PerceptionDebug("[Gemini] CompleteWithSystem: model=%s system_len=%d user_len=%d", c.model, len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		logging.PerceptionError("[Gemini] CompleteWithSystem: API key not configured")
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
	if isPiggyback {
		reqBody.GenerationConfig.ResponseMimeType = "application/json"
		reqBody.GenerationConfig.ResponseJsonSchema = BuildGeminiPiggybackEnvelopeSchema()
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

		jsonData, err := json.Marshal(reqBody)
		if err != nil {
			return "", fmt.Errorf("failed to marshal request: %w", err)
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
			// Some models may reject responseJsonSchema; retry once without it.
			if isPiggyback && reqBody.GenerationConfig.ResponseJsonSchema != nil && resp.StatusCode == http.StatusBadRequest {
				bodyStr := string(body)
				if strings.Contains(bodyStr, "responseJsonSchema") || strings.Contains(bodyStr, "responseMimeType") {
					reqBody.GenerationConfig.ResponseJsonSchema = nil
					reqBody.GenerationConfig.ResponseMimeType = ""
					lastErr = fmt.Errorf("request rejected structured output, retrying without responseJsonSchema: %s", bodyStr)
					continue
				}
			}
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

		response := strings.TrimSpace(result.String())
		logging.Perception("[Gemini] CompleteWithSystem: completed in %v response_len=%d", time.Since(startTime), len(response))
		return response, nil
	}

	logging.PerceptionError("[Gemini] CompleteWithSystem: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// CompleteWithStreaming sends a prompt with streaming enabled.
// Returns channels of incremental content deltas.
func (c *GeminiClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan error) {
	contentChan := make(chan string, 100)
	errorChan := make(chan error, 1)

	logging.PerceptionDebug("[Gemini] CompleteWithStreaming: starting streaming model=%s", c.model)

	go func() {
		defer close(contentChan)
		defer close(errorChan)
		startTime := time.Now()

		if c.apiKey == "" {
			logging.PerceptionError("[Gemini] CompleteWithStreaming: API key not configured")
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
		if isPiggyback {
			reqBody.GenerationConfig.ResponseMimeType = "application/json"
			reqBody.GenerationConfig.ResponseJsonSchema = BuildGeminiPiggybackEnvelopeSchema()
		}

		url := fmt.Sprintf("%s/models/%s:streamGenerateContent?alt=sse&key=%s", c.baseURL, c.model, c.apiKey)

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

			req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewReader(jsonData))
			if err != nil {
				errorChan <- fmt.Errorf("failed to create request: %w", err)
				return
			}

			req.Header.Set("Content-Type", "application/json")
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

				// Some models may reject responseJsonSchema; retry once without it.
				if isPiggyback && reqBody.GenerationConfig.ResponseJsonSchema != nil && resp.StatusCode == http.StatusBadRequest {
					bodyStr := string(body)
					if strings.Contains(bodyStr, "responseJsonSchema") || strings.Contains(bodyStr, "responseMimeType") {
						reqBody.GenerationConfig.ResponseJsonSchema = nil
						reqBody.GenerationConfig.ResponseMimeType = ""
						lastErr = fmt.Errorf("request rejected structured output, retrying without responseJsonSchema: %s", bodyStr)
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

					var chunk GeminiResponse
					if err := json.Unmarshal([]byte(data), &chunk); err != nil {
						continue
					}
					if chunk.Error != nil {
						scanErrChan <- fmt.Errorf("API error: %s", chunk.Error.Message)
						return
					}
					if len(chunk.Candidates) == 0 {
						continue
					}
					for _, part := range chunk.Candidates[0].Content.Parts {
						if part.Text == "" {
							continue
						}
						select {
						case contentChan <- part.Text:
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
					logging.PerceptionError("[Gemini] CompleteWithStreaming: stream error after %v: %v", time.Since(startTime), err)
					errorChan <- fmt.Errorf("stream error: %w", err)
				default:
					logging.Perception("[Gemini] CompleteWithStreaming: completed in %v", time.Since(startTime))
				}
			case <-ctx.Done():
				resp.Body.Close()
				<-scanDone
				logging.PerceptionWarn("[Gemini] CompleteWithStreaming: cancelled after %v", time.Since(startTime))
				errorChan <- ctx.Err()
			}
			return
		}

		logging.PerceptionError("[Gemini] CompleteWithStreaming: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
		errorChan <- fmt.Errorf("max retries exceeded: %w", lastErr)
	}()

	return contentChan, errorChan
}

// SetModel changes the model used for completions.
func (c *GeminiClient) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *GeminiClient) GetModel() string {
	return c.model
}
