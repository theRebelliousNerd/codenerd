package perception

import (
	"bufio"
	"bytes"
	"codenerd/internal/config"
	"codenerd/internal/logging"
	"codenerd/internal/usage"
	"context"
	crand "crypto/rand"
	"crypto/tls"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	mathrand "math/rand"
	"net/http"
	"net/http/httptrace"
	"strconv"
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
	_, _ = crand.Read(randBytes)
	return fmt.Sprintf("zai-%d-%s", count, hex.EncodeToString(randBytes))
}

// zaiLogger returns the API logger for ZAI client instrumentation
func zaiLogger() *logging.Logger {
	return logging.Get(logging.CategoryAPI)
}

func sleepWithContext(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
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
	rateLimitDelay   time.Duration
	retryBackoffBase time.Duration
	retryBackoffMax  time.Duration
	maxRetries       int
	streamingTimeout time.Duration
	cooldownUntil    time.Time
	randMu           sync.Mutex
	rng              *mathrand.Rand
}

// DefaultZAIConfig returns sensible defaults.
func DefaultZAIConfig(apiKey string) ZAIConfig {
	timeouts := config.GetLLMTimeouts()
	timeout := timeouts.HTTPClientTimeout
	if timeout <= 0 {
		timeout = 10 * time.Minute
	}
	rateLimitDelay := timeouts.RateLimitDelay
	if rateLimitDelay <= 0 {
		rateLimitDelay = 600 * time.Millisecond
	}
	retryBase := timeouts.RetryBackoffBase
	if retryBase <= 0 {
		retryBase = 1 * time.Second
	}
	retryMax := timeouts.RetryBackoffMax
	if retryMax <= 0 {
		retryMax = 30 * time.Second
	}
	maxRetries := timeouts.MaxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	streamingTimeout := timeouts.StreamingTimeout
	if streamingTimeout <= 0 {
		streamingTimeout = timeout
	}
	return ZAIConfig{
		APIKey:           apiKey,
		BaseURL:          "https://api.z.ai/api/coding/paas/v4", // Coding-optimized endpoint
		Model:            "glm-4.7",
		Timeout:          timeout,
		SystemPrompt:     defaultSystemPrompt,
		MaxRetries:       maxRetries,
		RetryBackoffBase: retryBase,
		RetryBackoffMax:  retryMax,
		RateLimitDelay:   rateLimitDelay,
		StreamingTimeout: streamingTimeout,
	}
}

// NewZAIClient creates a new ZAI client with default config.
func NewZAIClient(apiKey string) *ZAIClient {
	config := DefaultZAIConfig(apiKey)
	return NewZAIClientWithConfig(config)
}

// NewZAIClientWithConfig creates a new ZAI client with custom config.
func NewZAIClientWithConfig(config ZAIConfig) *ZAIClient {
	defaults := DefaultZAIConfig(config.APIKey)
	if config.BaseURL == "" {
		config.BaseURL = defaults.BaseURL
	}
	if config.Model == "" {
		config.Model = defaults.Model
	}
	if config.Timeout <= 0 {
		config.Timeout = defaults.Timeout
	}
	if config.SystemPrompt == "" {
		config.SystemPrompt = defaults.SystemPrompt
	}
	if config.RateLimitDelay <= 0 {
		config.RateLimitDelay = defaults.RateLimitDelay
	}
	if config.RetryBackoffBase <= 0 {
		config.RetryBackoffBase = defaults.RetryBackoffBase
	}
	if config.RetryBackoffMax <= 0 {
		config.RetryBackoffMax = defaults.RetryBackoffMax
	}
	if config.MaxRetries <= 0 {
		config.MaxRetries = defaults.MaxRetries
	}
	if config.StreamingTimeout <= 0 {
		config.StreamingTimeout = defaults.StreamingTimeout
	}
	client := &ZAIClient{
		apiKey:  config.APIKey,
		baseURL: config.BaseURL,
		model:   config.Model,
		httpClient: &http.Client{
			Timeout: config.Timeout,
		},
		semDisabled: config.DisableSemaphore,
		rateLimitDelay:   config.RateLimitDelay,
		retryBackoffBase: config.RetryBackoffBase,
		retryBackoffMax:  config.RetryBackoffMax,
		maxRetries:       config.MaxRetries,
		streamingTimeout: config.StreamingTimeout,
		rng:              mathrand.New(mathrand.NewSource(time.Now().UnixNano())),
	}
	// Only create semaphore if not disabled (external scheduler handles concurrency)
	if !config.DisableSemaphore {
		client.sem = make(chan struct{}, 5) // Z.AI API allows max 5 concurrent requests
	}
	return client
}

// DisableSemaphore disables the internal concurrency semaphore when an external scheduler is used.
// This is safe to call multiple times and should be invoked before issuing requests.
func (c *ZAIClient) DisableSemaphore() {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.semDisabled = true
	c.sem = nil
}

func contextRemaining(deadline time.Time) (time.Duration, bool) {
	if deadline.IsZero() {
		return 0, false
	}
	return time.Until(deadline), true
}

func maxDuration(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func (c *ZAIClient) jitterDuration(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	c.randMu.Lock()
	factor := 0.5 + c.rng.Float64()
	c.randMu.Unlock()
	return time.Duration(float64(d) * factor)
}

func (c *ZAIClient) nextRetryDelay(attempt int) time.Duration {
	base := c.retryBackoffBase
	if base <= 0 {
		base = 1 * time.Second
	}
	maxDelay := c.retryBackoffMax
	if maxDelay <= 0 {
		maxDelay = 30 * time.Second
	}
	if attempt < 1 {
		attempt = 1
	}
	delay := base * time.Duration(1<<uint(attempt-1))
	if delay > maxDelay {
		delay = maxDelay
	}
	return c.jitterDuration(delay)
}

func (c *ZAIClient) waitForRateLimit(ctx context.Context, reqID string, log *logging.Logger, deadline time.Time) error {
	delay := time.Duration(0)
	minDelay := c.rateLimitDelay
	if minDelay <= 0 {
		minDelay = 600 * time.Millisecond
	}

	c.mu.Lock()
	now := time.Now()
	if now.Before(c.cooldownUntil) {
		delay = c.cooldownUntil.Sub(now)
	}
	if gap := minDelay - now.Sub(c.lastRequest); gap > delay {
		delay = gap
	}
	if delay < 0 {
		delay = 0
	}
	c.lastRequest = now.Add(delay)
	c.mu.Unlock()

	if delay <= 0 {
		return nil
	}

	remaining, hasDeadline := contextRemaining(deadline)
	log.StructuredLog("debug", "Rate limit sleep starting", map[string]interface{}{
		"request_id":            reqID,
		"sleep_ms":              delay.Milliseconds(),
		"context_remaining_ms":  remaining.Milliseconds(),
		"has_deadline":          hasDeadline,
		"would_exceed_deadline": hasDeadline && delay > remaining,
	})

	if err := sleepWithContext(ctx, delay); err != nil {
		log.StructuredLog("error", "Rate limit sleep cancelled", map[string]interface{}{
			"request_id": reqID,
			"error":      err.Error(),
		})
		return err
	}

	log.Debug("[%s] Rate limit sleep completed, context_remaining_ms=%d",
		reqID, remaining.Milliseconds())
	return nil
}

func parseRetryAfter(resp *http.Response) time.Duration {
	if resp == nil {
		return 0
	}
	raw := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if raw == "" {
		return 0
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		if secs <= 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if when, err := http.ParseTime(raw); err == nil {
		delay := time.Until(when)
		if delay < 0 {
			return 0
		}
		return delay
	}
	return 0
}

func shouldRetryStatus(code int) bool {
	switch code {
	case http.StatusTooManyRequests,
		http.StatusRequestTimeout,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	default:
		return false
	}
}

// Complete sends a prompt and returns the completion.
func (c *ZAIClient) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}

// CompleteWithSystem sends a prompt with a system message.
// ENHANCED (v1.2.0): Automatically uses structured output + thinking mode for Piggyback Protocol.
func (c *ZAIClient) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	// Generate request ID for tracing
	reqID := generateRequestID()
	log := zaiLogger()
	startTime := time.Now()

	// Log context deadline for timeout investigation
	var contextDeadline time.Time
	var deadlineRemaining time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		contextDeadline = deadline
		deadlineRemaining = time.Until(deadline)
	}

	log.StructuredLog("debug", "ZAI request started", map[string]interface{}{
		"request_id":           reqID,
		"method":               "CompleteWithSystem",
		"context_deadline":     contextDeadline.Format(time.RFC3339),
		"context_remaining_ms": deadlineRemaining.Milliseconds(),
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

	if err := c.waitForRateLimit(ctx, reqID, log, contextDeadline); err != nil {
		return "", err
	}

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
	maxRetries := c.maxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	var lastErr error
	var cumulativeBackoffMs int64
	var retryDelayOverride time.Duration

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			backoffDuration := retryDelayOverride
			if backoffDuration <= 0 {
				backoffDuration = c.nextRetryDelay(i)
			}
			retryDelayOverride = 0
			cumulativeBackoffMs += backoffDuration.Milliseconds()
			remainingBeforeBackoff, hasDeadline := contextRemaining(contextDeadline)

			log.StructuredLog("debug", "Retry backoff starting", map[string]interface{}{
				"request_id":                 reqID,
				"attempt":                    i + 1,
				"backoff_ms":                 backoffDuration.Milliseconds(),
				"cumulative_backoff_ms":      cumulativeBackoffMs,
				"context_remaining_ms":       remainingBeforeBackoff.Milliseconds(),
				"would_exceed_deadline":      hasDeadline && backoffDuration > remainingBeforeBackoff,
			})

			if hasDeadline && backoffDuration > remainingBeforeBackoff {
				log.StructuredLog("error", "Retry backoff would exceed deadline", map[string]interface{}{
					"request_id":            reqID,
					"attempt":               i + 1,
					"backoff_ms":            backoffDuration.Milliseconds(),
					"context_remaining_ms":  remainingBeforeBackoff.Milliseconds(),
				})
				return "", ctx.Err()
			}

			if err := sleepWithContext(ctx, backoffDuration); err != nil {
				log.StructuredLog("error", "Retry backoff cancelled", map[string]interface{}{
					"request_id": reqID,
					"attempt":    i + 1,
					"error":      err.Error(),
				})
				return "", err
			}
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
			retryAfter := parseRetryAfter(resp)
			retryDelay := maxDuration(retryAfter, c.nextRetryDelay(i))
			c.mu.Lock()
			cooldownUntil := time.Now().Add(retryDelay)
			if cooldownUntil.After(c.cooldownUntil) {
				c.cooldownUntil = cooldownUntil
			}
			c.mu.Unlock()

			log.StructuredLog("warn", "Rate limit exceeded (429), will retry", map[string]interface{}{
				"request_id":            reqID,
				"retry_after_ms":        retryAfter.Milliseconds(),
				"backoff_ms":            retryDelay.Milliseconds(),
				"cumulative_backoff_ms": cumulativeBackoffMs,
			})
			lastErr = fmt.Errorf("rate limit exceeded (429)")
			retryDelayOverride = retryDelay
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if shouldRetryStatus(resp.StatusCode) {
				retryDelay := c.nextRetryDelay(i)
				lastErr = fmt.Errorf("retryable status %d: %s", resp.StatusCode, string(body))
				retryDelayOverride = retryDelay
				log.StructuredLog("warn", "Retryable HTTP status received", map[string]interface{}{
					"request_id":     reqID,
					"status_code":    resp.StatusCode,
					"backoff_ms":     retryDelay.Milliseconds(),
					"response_bytes": len(body),
				})
				continue
			}
			log.Error("[%s] API request failed with status %d: %s", reqID, resp.StatusCode, string(body))
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		// Bug #6 fix: Check for empty response (safety filter or API failure)
		trimmedBody := bytes.TrimSpace(body)
		if len(trimmedBody) == 0 {
			retryDelay := c.nextRetryDelay(i)
			log.StructuredLog("warn", "Empty response from API (possible safety filter), will retry", map[string]interface{}{
				"request_id": reqID,
				"attempt":    i + 1,
				"backoff_ms": retryDelay.Milliseconds(),
			})
			lastErr = fmt.Errorf("empty response from API")
			retryDelayOverride = retryDelay
			continue
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
	lastErrMsg := ""
	if lastErr != nil {
		lastErrMsg = lastErr.Error()
	}
	log.StructuredLog("error", "ZAI request failed after all retries", map[string]interface{}{
		"request_id":            reqID,
		"total_duration_ms":     totalDuration.Milliseconds(),
		"attempts":              maxRetries + 1,
		"cumulative_backoff_ms": cumulativeBackoffMs,
		"last_error":            lastErrMsg,
	})

	if lastErr == nil {
		lastErr = fmt.Errorf("request failed without error details")
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
	// Auto-apply timeout if context has no deadline (centralized timeout handling)
	if _, hasDeadline := ctx.Deadline(); !hasDeadline {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, c.httpClient.Timeout)
		defer cancel()
	}

	// Generate request ID for tracing
	reqID := generateRequestID()
	log := zaiLogger()
	startTime := time.Now()

	// Log context deadline for timeout investigation
	var contextDeadline time.Time
	var deadlineRemaining time.Duration
	if deadline, ok := ctx.Deadline(); ok {
		contextDeadline = deadline
		deadlineRemaining = time.Until(deadline)
	}

	log.StructuredLog("debug", "ZAI structured request started", map[string]interface{}{
		"request_id":           reqID,
		"method":               "CompleteWithStructuredOutput",
		"context_deadline":     contextDeadline.Format(time.RFC3339),
		"context_remaining_ms": deadlineRemaining.Milliseconds(),
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

	if err := c.waitForRateLimit(ctx, reqID, log, contextDeadline); err != nil {
		return "", err
	}

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
	maxRetries := c.maxRetries
	if maxRetries <= 0 {
		maxRetries = 3
	}
	var lastErr error
	var cumulativeBackoffMs int64
	var retryDelayOverride time.Duration

	for i := 0; i <= maxRetries; i++ {
		if i > 0 {
			backoffDuration := retryDelayOverride
			if backoffDuration <= 0 {
				backoffDuration = c.nextRetryDelay(i)
			}
			retryDelayOverride = 0
			cumulativeBackoffMs += backoffDuration.Milliseconds()
			remainingBeforeBackoff, hasDeadline := contextRemaining(contextDeadline)

			log.StructuredLog("debug", "Retry backoff starting", map[string]interface{}{
				"request_id":            reqID,
				"attempt":               i + 1,
				"backoff_ms":            backoffDuration.Milliseconds(),
				"cumulative_backoff_ms": cumulativeBackoffMs,
				"context_remaining_ms":  remainingBeforeBackoff.Milliseconds(),
				"would_exceed_deadline": hasDeadline && backoffDuration > remainingBeforeBackoff,
			})

			if hasDeadline && backoffDuration > remainingBeforeBackoff {
				log.StructuredLog("error", "Retry backoff would exceed deadline", map[string]interface{}{
					"request_id":           reqID,
					"attempt":              i + 1,
					"backoff_ms":           backoffDuration.Milliseconds(),
					"context_remaining_ms": remainingBeforeBackoff.Milliseconds(),
				})
				return "", ctx.Err()
			}

			if err := sleepWithContext(ctx, backoffDuration); err != nil {
				log.StructuredLog("error", "Retry backoff cancelled", map[string]interface{}{
					"request_id": reqID,
					"attempt":    i + 1,
					"error":      err.Error(),
				})
				return "", err
			}
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
			retryAfter := parseRetryAfter(resp)
			retryDelay := maxDuration(retryAfter, c.nextRetryDelay(i))
			c.mu.Lock()
			cooldownUntil := time.Now().Add(retryDelay)
			if cooldownUntil.After(c.cooldownUntil) {
				c.cooldownUntil = cooldownUntil
			}
			c.mu.Unlock()

			log.StructuredLog("warn", "Rate limit exceeded (429), will retry", map[string]interface{}{
				"request_id":            reqID,
				"retry_after_ms":        retryAfter.Milliseconds(),
				"backoff_ms":            retryDelay.Milliseconds(),
				"cumulative_backoff_ms": cumulativeBackoffMs,
			})
			lastErr = fmt.Errorf("rate limit exceeded (429)")
			retryDelayOverride = retryDelay
			continue
		}

		if resp.StatusCode != http.StatusOK {
			if shouldRetryStatus(resp.StatusCode) {
				retryDelay := c.nextRetryDelay(i)
				lastErr = fmt.Errorf("retryable status %d: %s", resp.StatusCode, string(body))
				retryDelayOverride = retryDelay
				log.StructuredLog("warn", "Retryable HTTP status received", map[string]interface{}{
					"request_id":     reqID,
					"status_code":    resp.StatusCode,
					"backoff_ms":     retryDelay.Milliseconds(),
					"response_bytes": len(body),
				})
				continue
			}
			log.Error("[%s] API request failed with status %d: %s", reqID, resp.StatusCode, string(body))
			return "", fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
		}

		// Bug #6 fix: Check for empty response (safety filter or API failure)
		trimmedBody := bytes.TrimSpace(body)
		if len(trimmedBody) == 0 {
			retryDelay := c.nextRetryDelay(i)
			log.StructuredLog("warn", "Empty response from API (possible safety filter), will retry", map[string]interface{}{
				"request_id": reqID,
				"attempt":    i + 1,
				"backoff_ms": retryDelay.Milliseconds(),
			})
			lastErr = fmt.Errorf("empty response from API")
			retryDelayOverride = retryDelay
			continue
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
	lastErrMsg := ""
	if lastErr != nil {
		lastErrMsg = lastErr.Error()
	}
	log.StructuredLog("error", "ZAI structured request failed after all retries", map[string]interface{}{
		"request_id":            reqID,
		"total_duration_ms":     totalDuration.Milliseconds(),
		"attempts":              maxRetries + 1,
		"cumulative_backoff_ms": cumulativeBackoffMs,
		"last_error":            lastErrMsg,
	})

	if lastErr == nil {
		lastErr = fmt.Errorf("request failed without error details")
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

		// Auto-apply timeout if context has no deadline (centralized timeout handling)
		// For streaming, this applies a streaming-specific timeout as a maximum duration
		if _, hasDeadline := ctx.Deadline(); !hasDeadline {
			var cancel context.CancelFunc
			timeout := c.streamingTimeout
			if timeout <= 0 {
				timeout = c.httpClient.Timeout
			}
			ctx, cancel = context.WithTimeout(ctx, timeout)
			defer cancel()
		}

		reqID := generateRequestID()
		log := zaiLogger()
		startTime := time.Now()

		var contextDeadline time.Time
		if deadline, ok := ctx.Deadline(); ok {
			contextDeadline = deadline
		}

		if c.apiKey == "" {
			errorChan <- fmt.Errorf("API key not configured")
			return
		}

		if strings.TrimSpace(systemPrompt) == "" {
			systemPrompt = defaultSystemPrompt
		} else {
			systemPrompt = defaultSystemPrompt + "\n" + systemPrompt
		}

		maxRetries := c.maxRetries
		if maxRetries <= 0 {
			maxRetries = 3
		}
		var lastErr error
		var cumulativeBackoffMs int64
		var retryDelayOverride time.Duration

		for i := 0; i <= maxRetries; i++ {
			if i > 0 {
				backoffDuration := retryDelayOverride
				if backoffDuration <= 0 {
					backoffDuration = c.nextRetryDelay(i)
				}
				retryDelayOverride = 0
				cumulativeBackoffMs += backoffDuration.Milliseconds()
				remainingBeforeBackoff, hasDeadline := contextRemaining(contextDeadline)

				log.StructuredLog("debug", "Retry backoff starting", map[string]interface{}{
					"request_id":            reqID,
					"attempt":               i + 1,
					"backoff_ms":            backoffDuration.Milliseconds(),
					"cumulative_backoff_ms": cumulativeBackoffMs,
					"context_remaining_ms":  remainingBeforeBackoff.Milliseconds(),
					"would_exceed_deadline": hasDeadline && backoffDuration > remainingBeforeBackoff,
				})

				if hasDeadline && backoffDuration > remainingBeforeBackoff {
					errorChan <- ctx.Err()
					return
				}

				if err := sleepWithContext(ctx, backoffDuration); err != nil {
					errorChan <- err
					return
				}
			}

			if ctx.Err() != nil {
				errorChan <- ctx.Err()
				return
			}

			// Acquire concurrency semaphore (max 5 concurrent requests)
			// Skip if disabled (external APIScheduler manages concurrency)
			acquired := false
			if !c.semDisabled {
				select {
				case c.sem <- struct{}{}:
					acquired = true
				case <-ctx.Done():
					errorChan <- ctx.Err()
					return
				}
			}

			if err := c.waitForRateLimit(ctx, reqID, log, contextDeadline); err != nil {
				if acquired {
					<-c.sem
				}
				errorChan <- err
				return
			}

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
				if acquired {
					<-c.sem
				}
				errorChan <- fmt.Errorf("failed to marshal request: %w", err)
				return
			}

			req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
			if err != nil {
				if acquired {
					<-c.sem
				}
				errorChan <- fmt.Errorf("failed to create request: %w", err)
				return
			}

			req.Header.Set("Content-Type", "application/json")
			req.Header.Set("Authorization", "Bearer "+c.apiKey)

			resp, err := c.httpClient.Do(req)
			if err != nil {
				if acquired {
					<-c.sem
				}
				lastErr = fmt.Errorf("request failed: %w", err)
				continue
			}

			if resp.StatusCode != http.StatusOK {
				body, _ := io.ReadAll(resp.Body)
				_ = resp.Body.Close()
				if acquired {
					<-c.sem
				}
				if resp.StatusCode == http.StatusTooManyRequests {
					retryAfter := parseRetryAfter(resp)
					retryDelay := maxDuration(retryAfter, c.nextRetryDelay(i))
					c.mu.Lock()
					cooldownUntil := time.Now().Add(retryDelay)
					if cooldownUntil.After(c.cooldownUntil) {
						c.cooldownUntil = cooldownUntil
					}
					c.mu.Unlock()
					lastErr = fmt.Errorf("rate limit exceeded (429)")
					retryDelayOverride = retryDelay
					continue
				}
				if shouldRetryStatus(resp.StatusCode) {
					retryDelay := c.nextRetryDelay(i)
					lastErr = fmt.Errorf("retryable status %d: %s", resp.StatusCode, string(body))
					retryDelayOverride = retryDelay
					continue
				}
				errorChan <- fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body))
				return
			}

			if acquired {
				defer func() { <-c.sem }()
			}
			defer resp.Body.Close()

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
			return
		}

		totalDuration := time.Since(startTime)
		lastErrMsg := ""
		if lastErr != nil {
			lastErrMsg = lastErr.Error()
		}
		log.StructuredLog("error", "ZAI streaming request failed after all retries", map[string]interface{}{
			"request_id":            reqID,
			"total_duration_ms":     totalDuration.Milliseconds(),
			"attempts":              maxRetries + 1,
			"cumulative_backoff_ms": cumulativeBackoffMs,
			"last_error":            lastErrMsg,
		})

		if lastErr == nil {
			lastErr = fmt.Errorf("request failed without error details")
		}
		errorChan <- fmt.Errorf("streaming retries exhausted: %w", lastErr)
	}()

	return contentChan, errorChan
}
