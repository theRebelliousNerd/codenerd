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

// ollamaTransport is Ollama's Chat Completions transport: POST
// {endpoint}/v1/chat/completions, the OpenAI-shaped surface Ollama serves.
// Only OllamaClient builds one. OpenAI itself is on the Responses API
// (client_openai.go), so this is the last first-party Chat Completions
// transport of its kind; xAI and OpenRouter share ExecuteOpenAIRequest.
type ollamaTransport struct {
	apiKey     string
	baseURL    string
	model      string
	httpClient *http.Client

	// provider is the id usage accounting records calls under: always
	// ProviderOllama, so local tokens never inflate a cloud vendor's row.
	provider Provider

	mu          sync.Mutex
	lastRequest time.Time
	// maxOutputTokens is the completion ceiling sent on every request.
	maxOutputTokens int
}

func newOllamaTransport(baseURL, model string, timeout time.Duration, maxOutputTokens int) *ollamaTransport {
	return &ollamaTransport{
		// Ollama ignores auth; the header is a placeholder.
		apiKey:          "ollama",
		baseURL:         baseURL,
		model:           model,
		provider:        ProviderOllama,
		httpClient:      NewSharedHTTPClient(timeout),
		maxOutputTokens: orDefaultTokens(maxOutputTokens, 4096),
	}
}

// rateLimit enforces minimum inter-request spacing to avoid 429 responses.
// Must be called before each API request.
func (c *ollamaTransport) rateLimit() {
	c.mu.Lock()
	elapsed := time.Since(c.lastRequest)
	if elapsed < 100*time.Millisecond {
		time.Sleep(100*time.Millisecond - elapsed)
	}
	c.lastRequest = time.Now()
	c.mu.Unlock()
}

// Complete sends a prompt and returns the completion.
func (c *ollamaTransport) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
func (c *ollamaTransport) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	startTime := time.Now()
	logging.PerceptionDebug("[Ollama] CompleteWithSystem: model=%s system_len=%d user_len=%d", c.model, len(systemPrompt), len(userPrompt))

	if c.apiKey == "" {
		logging.PerceptionError("[Ollama] CompleteWithSystem: API key not configured")
		return "", fmt.Errorf("API key not configured")
	}

	isPiggyback := !types.IsStructuredOutputOnlyCtx(ctx) &&
		(strings.Contains(systemPrompt, "control_packet") ||
			strings.Contains(systemPrompt, "surface_response") ||
			strings.Contains(userPrompt, "PiggybackEnvelope") ||
			strings.Contains(userPrompt, "control_packet"))

	// Rate limiting
	c.rateLimit()

	messages := systemAndUserMessages(systemPrompt, userPrompt)

	reqBody := OpenAIRequest{
		Model:       c.model,
		Messages:    messages,
		MaxTokens:   c.maxOutputTokens,
		Temperature: types.TemperatureFor(ctx, 0.1),
	}
	if isPiggyback {
		reqBody.ResponseFormat = BuildOpenAIPiggybackEnvelopeSchema()
	}

	// Retry loop for rate limits
	maxRetries := llmMaxRetries()
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Context-aware backoff: a cancelled turn must exit during
			// the sleep, not after it (matches ExecuteOpenAIRequest).
			backoff := llmRetryBackoff(i)
			select {
			case <-ctx.Done():
				return "", fmt.Errorf("request cancelled during retry backoff: %w", ctx.Err())
			case <-time.After(backoff):
			}
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

		if isTransientHTTPStatus(resp.StatusCode) {
			lastErr = fmt.Errorf("transient server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
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
			logging.PerceptionError("[Ollama] CompleteWithSystem: no completion returned")
			return "", fmt.Errorf("no completion returned")
		}

		reportUsage(ctx, c.model, c.provider, usageOpChat, openaiResp.Usage.metadata())

		response := strings.TrimSpace(openaiResp.Choices[0].Message.Content)
		if finish := openaiResp.Choices[0].FinishReason; types.LengthStop(finish) {
			return "", outputTruncated(c.provider, c.model, "CompleteWithSystem", finish, response,
				c.maxOutputTokens, openaiResp.Usage.CompletionTokens)
		}
		logging.Perception("[Ollama] CompleteWithSystem: completed in %v response_len=%d", time.Since(startTime), len(response))
		return response, nil
	}

	logging.PerceptionError("[Ollama] CompleteWithSystem: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
	return "", fmt.Errorf("max retries exceeded: %w", lastErr)
}

// CompleteWithStreaming sends a prompt with streaming enabled.
// Returns channels of incremental content deltas.
func (c *ollamaTransport) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, _ bool) (<-chan string, <-chan types.StreamResult) {
	w, contentChan, doneChan := types.NewStream()

	logging.PerceptionDebug("[Ollama] CompleteWithStreaming: starting streaming model=%s", c.model)

	go func() {
		defer w.Close()

		// Auto-apply timeout if context has no deadline (centralized timeout handling)
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
			defer cancel()
		}

		startTime := time.Now()

		if c.apiKey == "" {
			logging.PerceptionError("[Ollama] CompleteWithStreaming: API key not configured")
			w.Fail(fmt.Errorf("API key not configured"))
			return
		}

		isPiggyback := !types.IsStructuredOutputOnlyCtx(ctx) &&
			(strings.Contains(systemPrompt, "control_packet") ||
				strings.Contains(systemPrompt, "surface_response") ||
				strings.Contains(userPrompt, "PiggybackEnvelope") ||
				strings.Contains(userPrompt, "control_packet"))

		// Rate limiting
		c.rateLimit()

		messages := systemAndUserMessages(systemPrompt, userPrompt)

		reqBody := OpenAIRequest{
			Model:       c.model,
			Messages:    messages,
			MaxTokens:   c.maxOutputTokens,
			Temperature: types.TemperatureFor(ctx, 0.1),
			Stream:      true,
			StreamOptions: &OpenAIStreamOptions{
				IncludeUsage: true,
			},
		}
		if isPiggyback {
			reqBody.ResponseFormat = BuildOpenAIPiggybackEnvelopeSchema()
		}

		// Retry loop for initial request setup / rate limits (before streaming begins).
		maxRetries := llmMaxRetries()
		var lastErr error

		for attempt := 0; attempt <= maxRetries; attempt++ {
			if attempt > 0 {
				// Context-aware backoff: a cancelled turn must exit during
				// the sleep, not after it (matches ExecuteOpenAIRequest).
				backoff := llmRetryBackoff(attempt)
				select {
				case <-ctx.Done():
					w.Fail(fmt.Errorf("request cancelled during retry backoff: %w", ctx.Err()))
					return
				case <-time.After(backoff):
				}
			}

			jsonData, err := json.Marshal(reqBody)
			if err != nil {
				w.Fail(fmt.Errorf("failed to marshal request: %w", err))
				return
			}

			req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
			if err != nil {
				w.Fail(fmt.Errorf("failed to create request: %w", err))
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

			if isTransientHTTPStatus(resp.StatusCode) {
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
				resp.Body.Close()
				lastErr = fmt.Errorf("transient server error (%d): %s", resp.StatusCode, strings.TrimSpace(string(body)))
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

				w.Fail(fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body)))
				return
			}

			defer resp.Body.Close()
			res := readChatStream(ctx, resp.Body, chatStreamProfile{provider: c.provider, model: c.model, limit: c.maxOutputTokens}, w)
			reportUsage(ctx, c.model, c.provider, usageOpChat, res.Usage)
			logStreamEnd("Ollama", startTime, res)
			w.Finish(res)
			return
		}

		logging.PerceptionError("[Ollama] CompleteWithStreaming: max retries exceeded after %v: %v", time.Since(startTime), lastErr)
		w.Fail(fmt.Errorf("max retries exceeded: %w", lastErr))
	}()

	return contentChan, doneChan
}

// SetModel changes the model used for completions.
func (c *ollamaTransport) SetModel(model string) {
	c.model = model
}

// GetModel returns the current model.
func (c *ollamaTransport) GetModel() string {
	return c.model
}

// CompleteWithTools sends a prompt with tool definitions.
func (c *ollamaTransport) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []ToolDefinition) (*LLMToolResponse, error) {
	// Map generic tools to OpenAI tools
	openAITools := MapToolDefinitionsToOpenAI(tools)

	reqBody := OpenAIRequest{
		Model:       c.model,
		MaxTokens:   c.maxOutputTokens,
		Temperature: types.TemperatureFor(ctx, 0),
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
	if types.LengthStop(stopReason) {
		return nil, outputTruncated(c.provider, c.model, "CompleteWithTools", stopReason, choice.Message.Content,
			c.maxOutputTokens, resp.Usage.CompletionTokens)
	}
	if stopReason == "tool_calls" {
		stopReason = "tool_use" // Standardize on "tool_use"
	}

	return &LLMToolResponse{
		Text:       choice.Message.Content,
		ToolCalls:  toolCalls,
		StopReason: stopReason,
		Usage:      resp.Usage.metadata(),
	}, nil
}

// completeNonStreaming performs a non-streaming completion request through
// the shared ExecuteOpenAIRequest helper (retry, backoff, parse) and adds the
// usage accounting.
func (c *ollamaTransport) completeNonStreaming(ctx context.Context, reqBody OpenAIRequest) (*OpenAIResponse, error) {
	if c.apiKey == "" {
		return nil, fmt.Errorf("API key not configured")
	}
	openAIResp, err := ExecuteOpenAIRequest(ctx, c.httpClient, c.baseURL, c.apiKey, reqBody)
	if err != nil {
		return nil, err
	}

	// Tracked here rather than at each caller: this is the single
	// non-streaming HTTP path on this client, so every billed request is
	// counted exactly once even when a caller retries at a higher level.
	reportUsage(ctx, reqBody.Model, c.provider, usageOpFor(len(reqBody.Tools)), openAIResp.Usage.metadata())

	return openAIResp, nil
}
