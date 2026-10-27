package perception

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"strconv"
	"time"

	"codenerd/internal/config"
)

const anthropicResponseLimit = 10 * 1024 * 1024

// AnthropicAPIError retains the status/type/request ID for diagnostics without
// putting a provider response body (which can echo prompts or secrets) in logs.
type AnthropicAPIError struct {
	StatusCode int
	Type       string
	RequestID  string
}

func (e *AnthropicAPIError) Error() string {
	return fmt.Sprintf("Anthropic API request failed with status %d (type=%s, request_id=%s)", e.StatusCode, e.Type, e.RequestID)
}

func anthropicError(status int, body []byte, requestID string) *AnthropicAPIError {
	var envelope struct {
		RequestID string `json:"request_id"`
		Error     struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	_ = json.Unmarshal(body, &envelope)
	if requestID == "" {
		requestID = envelope.RequestID
	}
	return &AnthropicAPIError{StatusCode: status, Type: envelope.Error.Type, RequestID: requestID}
}

func waitAnthropic(ctx context.Context, delay time.Duration) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if delay <= 0 {
		return nil
	}
	timer := time.NewTimer(delay)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-timer.C:
		return nil
	}
}

func (c *AnthropicClient) rateLimit(ctx context.Context) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	next := c.lastRequest.Add(100 * time.Millisecond)
	if now := time.Now(); now.After(next) {
		next = now
	}
	c.lastRequest = next
	c.mu.Unlock()
	return waitAnthropic(ctx, time.Until(next))
}

func anthropicRetryAfter(value string, now time.Time) time.Duration {
	if seconds, err := strconv.ParseFloat(value, 64); err == nil && seconds >= 0 {
		if math.IsInf(seconds, 0) || math.IsNaN(seconds) {
			return 0
		}
		if seconds >= float64(math.MaxInt64)/float64(time.Second) {
			return time.Duration(math.MaxInt64)
		}
		return time.Duration(seconds * float64(time.Second))
	}
	if date, err := http.ParseTime(value); err == nil && date.After(now) {
		return date.Sub(now)
	}
	return 0
}

func readAnthropicBody(body io.Reader) ([]byte, error) {
	data, err := io.ReadAll(io.LimitReader(body, anthropicResponseLimit+1))
	if err != nil {
		return nil, fmt.Errorf("read Anthropic response: %w", err)
	}
	if len(data) > anthropicResponseLimit {
		return nil, fmt.Errorf("Anthropic response exceeds %d bytes", anthropicResponseLimit)
	}
	return data, nil
}

// sendMessages retries only transport failures and retryable HTTP statuses.
// Once a stream has started it is never replayed: the harness may already have
// displayed output. Deterministic 4xx errors retain their request ID and fail fast.
func (c *AnthropicClient) sendMessages(ctx context.Context, reqBody AnthropicRequest, plan anthropicCachePlan) (*http.Response, error) {
	data, err := marshalAnthropicRequest(reqBody, plan)
	if err != nil {
		return nil, fmt.Errorf("marshal Anthropic request: %w", err)
	}
	var lastErr error
	var retryAfter time.Duration
	for attempt := 0; attempt <= llmMaxRetries(); attempt++ {
		if attempt > 0 {
			delay := llmRetryBackoff(attempt)
			if retryAfter > delay {
				delay = retryAfter
			}
			if err := waitAnthropic(ctx, delay); err != nil {
				return nil, fmt.Errorf("request cancelled during retry backoff: %w", err)
			}
		}
		if err := c.rateLimit(ctx); err != nil {
			return nil, fmt.Errorf("Anthropic request canceled: %w", err)
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/messages", bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("create Anthropic request: %w", err)
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", config.AnthropicAPIVersion)
		if c.options.WorkspaceID != "" {
			req.Header.Set("anthropic-workspace-id", c.options.WorkspaceID)
		}
		if reqBody.Stream {
			req.Header.Set("Accept", "text/event-stream")
		}
		resp, err := c.httpClient.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return nil, fmt.Errorf("Anthropic request canceled: %w", ctx.Err())
			}
			lastErr = fmt.Errorf("Anthropic request failed: %w", err)
			retryAfter = 0
			continue
		}
		if resp.StatusCode == http.StatusOK {
			return resp, nil
		}
		data, readErr := readAnthropicBody(resp.Body)
		resp.Body.Close()
		if readErr != nil {
			return nil, readErr
		}
		lastErr = anthropicError(resp.StatusCode, data, resp.Header.Get("request-id"))
		if resp.StatusCode != http.StatusTooManyRequests && !isTransientHTTPStatus(resp.StatusCode) {
			return nil, lastErr
		}
		retryAfter = anthropicRetryAfter(resp.Header.Get("Retry-After"), time.Now())
	}
	return nil, fmt.Errorf("Anthropic max retries exceeded: %w", lastErr)
}

// anthropicIncompleteError is a syntactically valid HTTP 200 whose JSON is
// not a usable assistant message (wrong type, no content blocks, or no stop
// reason). It carries only envelope diagnostics — never response content.
type anthropicIncompleteError struct {
	Type          string
	ID            string
	ContentBlocks int
	StopReason    string
	RequestID     string
}

func (e *anthropicIncompleteError) Error() string {
	return fmt.Sprintf("Anthropic returned an incomplete message (type=%q id=%q content_blocks=%d stop_reason=%q request_id=%s)",
		e.Type, e.ID, e.ContentBlocks, e.StopReason, e.RequestID)
}

// postMessages retries an incomplete 200 like other transient provider
// failures. No tool has run yet — the response is unusable before any block
// is interpreted — so replaying the same request is safe. One such response
// used to fail an entire shard delegation outright.
func (c *AnthropicClient) postMessages(ctx context.Context, req AnthropicRequest, plan anthropicCachePlan) (*AnthropicResponse, error) {
	var lastErr error
	for attempt := 0; attempt <= llmMaxRetries(); attempt++ {
		if attempt > 0 {
			if err := waitAnthropic(ctx, llmRetryBackoff(attempt)); err != nil {
				return nil, fmt.Errorf("request cancelled during retry backoff: %w", err)
			}
		}
		message, err := c.postMessagesOnce(ctx, req, plan)
		if err == nil {
			return message, nil
		}
		var incomplete *anthropicIncompleteError
		if !errors.As(err, &incomplete) {
			return nil, err
		}
		lastErr = err
		if ctx.Err() != nil {
			return nil, fmt.Errorf("Anthropic request canceled: %w", ctx.Err())
		}
	}
	return nil, fmt.Errorf("Anthropic max retries exceeded: %w", lastErr)
}

func (c *AnthropicClient) postMessagesOnce(ctx context.Context, req AnthropicRequest, plan anthropicCachePlan) (*AnthropicResponse, error) {
	resp, err := c.sendMessages(ctx, req, plan)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, err := readAnthropicBody(resp.Body)
	if err != nil {
		return nil, err
	}
	var message AnthropicResponse
	if err := json.Unmarshal(body, &message); err != nil {
		return nil, fmt.Errorf("parse Anthropic response: %w", err)
	}
	if message.Error != nil {
		return nil, anthropicError(anthropicErrorStatus(message.Error.Type), body, resp.Header.Get("request-id"))
	}
	if message.Type != "message" || len(message.Content) == 0 || message.StopReason == "" {
		return nil, &anthropicIncompleteError{
			Type: message.Type, ID: message.ID, ContentBlocks: len(message.Content),
			StopReason: message.StopReason, RequestID: resp.Header.Get("request-id"),
		}
	}
	return &message, nil
}

func anthropicErrorStatus(kind string) int {
	switch kind {
	case "rate_limit_error":
		return 429
	case "overloaded_error":
		return 529
	case "api_error":
		return 500
	case "authentication_error":
		return 401
	case "permission_error":
		return 403
	case "not_found_error":
		return 404
	case "request_too_large":
		return 413
	case "invalid_request_error":
		return 400
	default:
		return 0
	}
}
