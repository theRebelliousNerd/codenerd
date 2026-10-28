package perception

import (
	"bytes"
	"codenerd/internal/types"
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
func (c *ZAIClient) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan types.StreamResult) {
	w, contentChan, doneChan := types.NewStream()

	go func() {
		defer w.Close()

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
			w.Fail(fmt.Errorf("API key not configured"))
			return
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
					// ctx.Err() is nil here — the deadline has NOT fired yet, the
					// backoff just would not fit before it. Sending ctx.Err()
					// delivered a nil error the consumer reads as clean success
					// with zero chunks. Name the give-up explicitly.
					w.Fail(fmt.Errorf("retry backoff of %v would exceed the context deadline in %v; giving up: %w",
						backoffDuration, remainingBeforeBackoff, context.DeadlineExceeded))
					return
				}

				if err := sleepWithContext(ctx, backoffDuration); err != nil {
					w.Fail(err)
					return
				}
			}

			if ctx.Err() != nil {
				w.Fail(ctx.Err())
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
					w.Fail(ctx.Err())
					return
				}
			}

			if err := c.waitForRateLimit(ctx, reqID, log, contextDeadline); err != nil {
				if acquired {
					<-c.sem
				}
				w.Fail(err)
				return
			}

			messages := make([]ZAIMessage, 0)
			// The caller owns the system prompt: an empty one sends none.
			if strings.TrimSpace(systemPrompt) != "" {
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
				Temperature:    types.TemperatureFor(ctx, 0.1),
				TopP:           types.TopPFor(ctx, 0.9),
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
				w.Fail(fmt.Errorf("failed to marshal request: %w", err))
				return
			}

			req, err := http.NewRequestWithContext(ctx, "POST", c.baseURL+"/chat/completions", bytes.NewReader(jsonData))
			if err != nil {
				if acquired {
					<-c.sem
				}
				w.Fail(fmt.Errorf("failed to create request: %w", err))
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
				w.Fail(fmt.Errorf("API request failed with status %d: %s", resp.StatusCode, string(body)))
				return
			}

			if acquired {
				defer func() { <-c.sem }()
			}
			defer resp.Body.Close()

			res := readChatStream(ctx, resp.Body, chatStreamProfile{provider: ProviderZAI, model: c.model, limit: reqBody.MaxTokens}, w)
			reportUsage(ctx, c.model, ProviderZAI, usageOpChat, res.Usage)
			logStreamEnd("ZAI", startTime, res)
			w.Finish(res)
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
		w.Fail(fmt.Errorf("streaming retries exhausted: %w", lastErr))
	}()

	return contentChan, doneChan
}
