package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

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

				log.StructuredLog("debug", "Retry backoff starting", map[string]any{
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
				body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
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
			// Buffer is pooled (64 KiB initial, 1 MiB max line) to match the other
			// streaming clients and reduce GC pressure on long-running sessions.
			scanner, releaseScanner := newPooledScanner(resp.Body, 1024*1024)
			defer releaseScanner()

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
		log.StructuredLog("error", "ZAI streaming request failed after all retries", map[string]any{
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
