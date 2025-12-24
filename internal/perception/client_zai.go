package perception

import (
	"bufio"
	"bytes"
	"codenerd/internal/logging"
	"codenerd/internal/usage"
	"context"
	"crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptrace"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// zaiRequestCounter provides unique request IDs for tracing
var zaiRequestCounter uint64

// generateRequestID creates a unique request ID for tracing
func generateRequestID() string {
	count := atomic.AddUint64(&zaiRequestCounter, 1)
	randBytes := make([]byte, 4)
	rand.Read(randBytes)
	return fmt.Sprintf("zai-%d-%s", count, hex.EncodeToString(randBytes))
}

// zaiLogger returns the API logger for ZAI client instrumentation
func zaiLogger() *logging.Logger {
	return logging.Get(logging.CategoryAPI)
}

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
	// Generate request ID for tracing
	reqID := generateRequestID()
	log := zaiLogger()
	startTime := time.Now()

	// Log context deadline for timeout investigation
	var contextDeadline time.Time
	var contextRemaining time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		contextDeadline = deadline
		contextRemaining = time.Until(deadline)
	}

	log.StructuredLog("debug", "ZAI request started", map[string]interface{}{
		"request_id":           reqID,
		"method":               "CompleteWithSystem",
		"context_deadline":     contextDeadline.Format(time.RFC3339),
		"context_remaining_ms": contextRemaining.Milliseconds(),
		"semaphore_disabled":   c.semDisabled,
		"prompt_length":        len(userPrompt),
		"system_prompt_length": len(systemPrompt),
	})

	// Detect if this is a Piggyback Protocol request (contains articulation instructions)
	isPiggyback := strings.Contains(systemPrompt, "control_packet") ||
		strings.Contains(systemPrompt, "surface_response") ||
		strings.Contains(userPrompt, "PiggybackEnvelope")

	// Use enhanced method for Piggyback Protocol
	if isPiggyback {
		log.Debug("[%s] Delegating to CompleteWithStructuredOutput (Piggyback detected)", reqID)
		return c.CompleteWithStructuredOutput(ctx, systemPrompt, userPrompt, false) // Disable thinking to prevent blocking timeouts
	}

	// Fallback to basic completion for other requests
	if c.apiKey == "" {
		log.Error("[%s] API key not configured", reqID)
		return "", fmt.Errorf("API key not configured")
	}

	// Acquire concurrency semaphore (max 5 concurrent requests)
	// Skip if disabled (external APIScheduler manages concurrency)
	if !c.semDisabled {
		semWaitStart := time.Now()
		log.Debug("[%s] Waiting for semaphore slot (in_use=%d/5)", reqID, len(c.sem))

		select {
		case c.sem <- struct{}{}:
			semWaitDuration := time.Since(semWaitStart)
			log.StructuredLog("debug", "Semaphore acquired", map[string]interface{}{
				"request_id":      reqID,
				"wait_ms":         semWaitDuration.Milliseconds(),
				"slots_in_use":    len(c.sem),
				"context_remaining_after_wait_ms": time.Until(contextDeadline).Milliseconds(),
			})
			defer func() { <-c.sem }()
		case <-ctx.Done():
			semWaitDuration := time.Since(semWaitStart)
			log.StructuredLog("error", "Context cancelled waiting for semaphore", map[string]interface{}{
				"request_id":   reqID,
				"wait_ms":      semWaitDuration.Milliseconds(),
				"error":        ctx.Err().Error(),
				"slots_in_use": len(c.sem),
			})
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
		sleepDuration := 600*time.Millisecond - elapsed
		remainingBeforeSleep := time.Until(contextDeadline)

		log.StructuredLog("debug", "Rate limit sleep starting", map[string]interface{}{
			"request_id":                 reqID,
			"sleep_ms":                   sleepDuration.Milliseconds(),
			"context_remaining_ms":       remainingBeforeSleep.Milliseconds(),
			"would_exceed_deadline":      sleepDuration > remainingBeforeSleep,
		})

		// WARNING: This sleep is NOT context-aware (investigation point)
		time.Sleep(sleepDuration)

		log.Debug("[%s] Rate limit sleep completed, context_remaining_ms=%d",
			reqID, time.Until(contextDeadline).Milliseconds())
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
		log.Error("[%s] Failed to marshal request: %v", reqID, err)
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop for 429 errors
	maxRetries := 3
	var lastErr error
	var cumulativeBackoffMs int64

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoffDuration := time.Duration(1<<uint(i-1)) * time.Second
			cumulativeBackoffMs += backoffDuration.Milliseconds()
			remainingBeforeBackoff := time.Until(contextDeadline)

			log.StructuredLog("debug", "Retry backoff starting", map[string]interface{}{
				"request_id":                 reqID,
				"attempt":                    i + 1,
				"backoff_ms":                 backoffDuration.Milliseconds(),
				"cumulative_backoff_ms":      cumulativeBackoffMs,
				"context_remaining_ms":       remainingBeforeBackoff.Milliseconds(),
				"would_exceed_deadline":      backoffDuration > remainingBeforeBackoff,
			})

			// WARNING: This sleep is NOT context-aware (investigation point)
			time.Sleep(backoffDuration)
		}

		// Check context before making HTTP request
		if ctx.Err() != nil {
			log.StructuredLog("error", "Context cancelled before HTTP request", map[string]interface{}{
				"request_id":            reqID,
				"attempt":               i + 1,
				"total_elapsed_ms":      time.Since(startTime).Milliseconds(),
				"cumulative_backoff_ms": cumulativeBackoffMs,
				"error":                 ctx.Err().Error(),
			})
			return "", ctx.Err()
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
		if err != nil {
			log.Error("[%s] Failed to create request: %v", reqID, err)
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		// Add httptrace for connection debugging
		var connReused bool
		var connIdleTime time.Duration
		var dnsStart, dnsDone, connectStart, connectDone, tlsStart, tlsDone, gotFirstByte time.Time

		trace := &httptrace.ClientTrace{
			DNSStart:     func(info httptrace.DNSStartInfo) { dnsStart = time.Now() },
			DNSDone:      func(info httptrace.DNSDoneInfo) { dnsDone = time.Now() },
			ConnectStart: func(network, addr string) { connectStart = time.Now() },
			ConnectDone:  func(network, addr string, err error) { connectDone = time.Now() },
			TLSHandshakeStart: func() { tlsStart = time.Now() },
			TLSHandshakeDone:  func(_ tls.ConnectionState, _ error) { tlsDone = time.Now() },
			GotConn: func(info httptrace.GotConnInfo) {
				connReused = info.Reused
				connIdleTime = info.IdleTime
				log.StructuredLog("debug", "Connection acquired", map[string]interface{}{
					"request_id": reqID,
					"reused":     info.Reused,
					"was_idle":   info.WasIdle,
					"idle_ms":    info.IdleTime.Milliseconds(),
				})
			},
			GotFirstResponseByte:  func() { gotFirstByte = time.Now() },
		}
		req = req.WithContext(httptrace.WithClientTrace(ctx, trace))

		httpStart := time.Now()
		log.StructuredLog("debug", "HTTP request starting", map[string]interface{}{
			"request_id":           reqID,
			"attempt":              i + 1,
			"context_remaining_ms": time.Until(contextDeadline).Milliseconds(),
			"url":                  c.baseURL + "/chat/completions",
		})

		resp, err := c.httpClient.Do(req)
		httpDuration := time.Since(httpStart)

		if err != nil {
			log.StructuredLog("error", "HTTP request failed", map[string]interface{}{
				"request_id":      reqID,
				"attempt":         i + 1,
				"http_duration_ms": httpDuration.Milliseconds(),
				"error":           err.Error(),
				"conn_reused":     connReused,
				"conn_idle_ms":    connIdleTime.Milliseconds(),
				"dns_ms":          dnsDone.Sub(dnsStart).Milliseconds(),
				"connect_ms":      connectDone.Sub(connectStart).Milliseconds(),
				"tls_ms":          tlsDone.Sub(tlsStart).Milliseconds(),
			})
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		readDuration := time.Since(httpStart) - httpDuration

		if err != nil {
			log.StructuredLog("error", "Response body read failed", map[string]interface{}{
				"request_id":      reqID,
				"attempt":         i + 1,
				"read_duration_ms": readDuration.Milliseconds(),
				"error":           err.Error(),
			})
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		log.StructuredLog("debug", "HTTP request completed", map[string]interface{}{
			"request_id":        reqID,
			"attempt":           i + 1,
			"status_code":       resp.StatusCode,
			"http_duration_ms":  httpDuration.Milliseconds(),
			"body_read_ms":      readDuration.Milliseconds(),
			"body_bytes":        len(body),
			"conn_reused":       connReused,
			"conn_idle_ms":      connIdleTime.Milliseconds(),
			"ttfb_ms":           gotFirstByte.Sub(httpStart).Milliseconds(),
		})

		if resp.StatusCode == http.StatusTooManyRequests {
			log.Warn("[%s] Rate limit exceeded (429), will retry", reqID)
			lastErr = fmt.Errorf("rate limit exceeded (429)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Error("[%s] API request failed with status %d: %s", reqID, resp.StatusCode, string(body))
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var zaiResp ZAIResponse
		if err := json.Unmarshal(body, &zaiResp); err != nil {
			log.Error("[%s] Failed to parse response: %v", reqID, err)
			return "", fmt.Errorf("failed to parse response: %w", err)
		}

		if zaiResp.Error != nil {
			log.Error("[%s] API error: %s", reqID, zaiResp.Error.Message)
			return "", fmt.Errorf("API error: %s", zaiResp.Error.Message)
		}

		if len(zaiResp.Choices) == 0 {
			log.Error("[%s] No completion returned", reqID)
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

		totalDuration := time.Since(startTime)
		log.StructuredLog("info", "ZAI request completed successfully", map[string]interface{}{
			"request_id":            reqID,
			"total_duration_ms":     totalDuration.Milliseconds(),
			"attempts":              i + 1,
			"cumulative_backoff_ms": cumulativeBackoffMs,
			"prompt_tokens":         zaiResp.Usage.PromptTokens,
			"completion_tokens":     zaiResp.Usage.CompletionTokens,
		})

		return strings.TrimSpace(zaiResp.Choices[0].Message.Content), nil
	}

	totalDuration := time.Since(startTime)
	log.StructuredLog("error", "ZAI request failed after all retries", map[string]interface{}{
		"request_id":            reqID,
		"total_duration_ms":     totalDuration.Milliseconds(),
		"attempts":              maxRetries + 1,
		"cumulative_backoff_ms": cumulativeBackoffMs,
		"last_error":            lastErr.Error(),
	})

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
	// Generate request ID for tracing
	reqID := generateRequestID()
	log := zaiLogger()
	startTime := time.Now()

	// Log context deadline for timeout investigation
	var contextDeadline time.Time
	var contextRemaining time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		contextDeadline = deadline
		contextRemaining = time.Until(deadline)
	}

	log.StructuredLog("debug", "ZAI structured request started", map[string]interface{}{
		"request_id":           reqID,
		"method":               "CompleteWithStructuredOutput",
		"context_deadline":     contextDeadline.Format(time.RFC3339),
		"context_remaining_ms": contextRemaining.Milliseconds(),
		"semaphore_disabled":   c.semDisabled,
		"prompt_length":        len(userPrompt),
		"system_prompt_length": len(systemPrompt),
		"thinking_enabled":     enableThinking,
	})

	if c.apiKey == "" {
		log.Error("[%s] API key not configured", reqID)
		return "", fmt.Errorf("API key not configured")
	}

	// Acquire concurrency semaphore (max 5 concurrent requests)
	// Skip if disabled (external APIScheduler manages concurrency)
	if !c.semDisabled {
		semWaitStart := time.Now()
		log.Debug("[%s] Waiting for semaphore slot (in_use=%d/5)", reqID, len(c.sem))

		select {
		case c.sem <- struct{}{}:
			semWaitDuration := time.Since(semWaitStart)
			log.StructuredLog("debug", "Semaphore acquired", map[string]interface{}{
				"request_id":                      reqID,
				"wait_ms":                         semWaitDuration.Milliseconds(),
				"slots_in_use":                    len(c.sem),
				"context_remaining_after_wait_ms": time.Until(contextDeadline).Milliseconds(),
			})
			defer func() { <-c.sem }()
		case <-ctx.Done():
			semWaitDuration := time.Since(semWaitStart)
			log.StructuredLog("error", "Context cancelled waiting for semaphore", map[string]interface{}{
				"request_id":   reqID,
				"wait_ms":      semWaitDuration.Milliseconds(),
				"error":        ctx.Err().Error(),
				"slots_in_use": len(c.sem),
			})
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
		sleepDuration := 600*time.Millisecond - elapsed
		remainingBeforeSleep := time.Until(contextDeadline)

		log.StructuredLog("debug", "Rate limit sleep starting", map[string]interface{}{
			"request_id":            reqID,
			"sleep_ms":              sleepDuration.Milliseconds(),
			"context_remaining_ms":  remainingBeforeSleep.Milliseconds(),
			"would_exceed_deadline": sleepDuration > remainingBeforeSleep,
		})

		// WARNING: This sleep is NOT context-aware (investigation point)
		time.Sleep(sleepDuration)

		log.Debug("[%s] Rate limit sleep completed, context_remaining_ms=%d",
			reqID, time.Until(contextDeadline).Milliseconds())
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
		ResponseFormat: BuildZAIPiggybackEnvelopeSchema(), // Z.AI: json_object only
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
		log.Error("[%s] Failed to marshal request: %v", reqID, err)
		return "", fmt.Errorf("failed to marshal request: %w", err)
	}

	// Retry loop
	maxRetries := 3
	var lastErr error
	var cumulativeBackoffMs int64

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			// Exponential backoff: 1s, 2s, 4s
			backoffDuration := time.Duration(1<<uint(i-1)) * time.Second
			cumulativeBackoffMs += backoffDuration.Milliseconds()
			remainingBeforeBackoff := time.Until(contextDeadline)

			log.StructuredLog("debug", "Retry backoff starting", map[string]interface{}{
				"request_id":            reqID,
				"attempt":               i + 1,
				"backoff_ms":            backoffDuration.Milliseconds(),
				"cumulative_backoff_ms": cumulativeBackoffMs,
				"context_remaining_ms":  remainingBeforeBackoff.Milliseconds(),
				"would_exceed_deadline": backoffDuration > remainingBeforeBackoff,
			})

			// WARNING: This sleep is NOT context-aware (investigation point)
			time.Sleep(backoffDuration)
		}

		// Check context before making HTTP request
		if ctx.Err() != nil {
			log.StructuredLog("error", "Context cancelled before HTTP request", map[string]interface{}{
				"request_id":            reqID,
				"attempt":               i + 1,
				"total_elapsed_ms":      time.Since(startTime).Milliseconds(),
				"cumulative_backoff_ms": cumulativeBackoffMs,
				"error":                 ctx.Err().Error(),
			})
			return "", ctx.Err()
		}

		req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
		if err != nil {
			log.Error("[%s] Failed to create request: %v", reqID, err)
			return "", fmt.Errorf("failed to create request: %w", err)
		}

		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		// Add httptrace for connection debugging
		var connReused bool
		var connIdleTime time.Duration
		var dnsStart, dnsDone, connectStart, connectDone, tlsStart, tlsDone, gotFirstByte time.Time

		trace := &httptrace.ClientTrace{
			DNSStart:          func(info httptrace.DNSStartInfo) { dnsStart = time.Now() },
			DNSDone:           func(info httptrace.DNSDoneInfo) { dnsDone = time.Now() },
			ConnectStart:      func(network, addr string) { connectStart = time.Now() },
			ConnectDone:       func(network, addr string, err error) { connectDone = time.Now() },
			TLSHandshakeStart: func() { tlsStart = time.Now() },
			TLSHandshakeDone:  func(_ tls.ConnectionState, _ error) { tlsDone = time.Now() },
			GotConn: func(info httptrace.GotConnInfo) {
				connReused = info.Reused
				connIdleTime = info.IdleTime
				log.StructuredLog("debug", "Connection acquired", map[string]interface{}{
					"request_id": reqID,
					"reused":     info.Reused,
					"was_idle":   info.WasIdle,
					"idle_ms":    info.IdleTime.Milliseconds(),
				})
			},
			GotFirstResponseByte: func() { gotFirstByte = time.Now() },
		}
		req = req.WithContext(httptrace.WithClientTrace(ctx, trace))

		httpStart := time.Now()
		log.StructuredLog("debug", "HTTP request starting", map[string]interface{}{
			"request_id":           reqID,
			"attempt":              i + 1,
			"context_remaining_ms": time.Until(contextDeadline).Milliseconds(),
			"url":                  c.baseURL + "/chat/completions",
		})

		resp, err := c.httpClient.Do(req)
		httpDuration := time.Since(httpStart)

		if err != nil {
			log.StructuredLog("error", "HTTP request failed", map[string]interface{}{
				"request_id":       reqID,
				"attempt":          i + 1,
				"http_duration_ms": httpDuration.Milliseconds(),
				"error":            err.Error(),
				"conn_reused":      connReused,
				"conn_idle_ms":     connIdleTime.Milliseconds(),
				"dns_ms":           dnsDone.Sub(dnsStart).Milliseconds(),
				"connect_ms":       connectDone.Sub(connectStart).Milliseconds(),
				"tls_ms":           tlsDone.Sub(tlsStart).Milliseconds(),
			})
			lastErr = fmt.Errorf("request failed: %w", err)
			continue
		}
		defer resp.Body.Close()

		body, err := io.ReadAll(resp.Body)
		readDuration := time.Since(httpStart) - httpDuration

		if err != nil {
			log.StructuredLog("error", "Response body read failed", map[string]interface{}{
				"request_id":       reqID,
				"attempt":          i + 1,
				"read_duration_ms": readDuration.Milliseconds(),
				"error":            err.Error(),
			})
			lastErr = fmt.Errorf("failed to read response: %w", err)
			continue
		}

		log.StructuredLog("debug", "HTTP request completed", map[string]interface{}{
			"request_id":       reqID,
			"attempt":          i + 1,
			"status_code":      resp.StatusCode,
			"http_duration_ms": httpDuration.Milliseconds(),
			"body_read_ms":     readDuration.Milliseconds(),
			"body_bytes":       len(body),
			"conn_reused":      connReused,
			"conn_idle_ms":     connIdleTime.Milliseconds(),
			"ttfb_ms":          gotFirstByte.Sub(httpStart).Milliseconds(),
		})

		if resp.StatusCode == http.StatusTooManyRequests {
			log.Warn("[%s] Rate limit exceeded (429), will retry", reqID)
			lastErr = fmt.Errorf("rate limit exceeded (429)")
			continue
		}

		if resp.StatusCode != http.StatusOK {
			log.Error("[%s] API request failed with status %d: %s", reqID, resp.StatusCode, string(body))
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		var zaiResp ZAIResponse
		if err := json.Unmarshal(body, &zaiResp); err != nil {
			log.Error("[%s] Failed to parse response: %v", reqID, err)
			// Try to handle potentially malformed response or double-encoding
			return "", fmt.Errorf("failed to parse response: %w (body excerpt: %s)", err, string(body[:min(len(body), 100)]))
		}

		if zaiResp.Error != nil {
			log.Error("[%s] API error: %s", reqID, zaiResp.Error.Message)
			return "", fmt.Errorf("API error: %s", zaiResp.Error.Message)
		}

		if len(zaiResp.Choices) == 0 {
			log.Error("[%s] No choices in response", reqID)
			return "", fmt.Errorf("no choices in response")
		}

		totalDuration := time.Since(startTime)
		log.StructuredLog("info", "ZAI structured request completed successfully", map[string]interface{}{
			"request_id":            reqID,
			"total_duration_ms":     totalDuration.Milliseconds(),
			"attempts":              i + 1,
			"cumulative_backoff_ms": cumulativeBackoffMs,
		})

		return strings.TrimSpace(zaiResp.Choices[0].Message.Content), nil
	}

	totalDuration := time.Since(startTime)
	log.StructuredLog("error", "ZAI structured request failed after all retries", map[string]interface{}{
		"request_id":            reqID,
		"total_duration_ms":     totalDuration.Milliseconds(),
		"attempts":              maxRetries + 1,
		"cumulative_backoff_ms": cumulativeBackoffMs,
		"last_error":            lastErr.Error(),
	})

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
			ResponseFormat: BuildZAIPiggybackEnvelopeSchema(), // Z.AI: json_object only with streaming
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
