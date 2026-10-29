package xaioauth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// doJSON posts JSON to the API with a Bearer token and classifies common errors.
func doJSON(ctx context.Context, httpClient *http.Client, method, urlStr, accessToken string, payload any, maxBody int64) (status int, headers http.Header, body []byte, err error) {
	if maxBody <= 0 {
		maxBody = 10 << 20
	}
	var reader io.Reader
	if payload != nil {
		data, mErr := json.Marshal(payload)
		if mErr != nil {
			return 0, nil, nil, fmt.Errorf("marshal request: %w", mErr)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, reader)
	if err != nil {
		return 0, nil, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp.StatusCode, resp.Header, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, resp.Header, body, nil
}

// classifyHTTPError maps status codes to typed package errors.
func classifyHTTPError(status int, body []byte, headers http.Header) error {
	text := string(body)
	lower := strings.ToLower(text)

	switch status {
	case http.StatusUnauthorized:
		return &AuthRequiredError{Detail: truncate(text, 200)}
	case http.StatusTooManyRequests:
		var retryAfter time.Duration
		if headers != nil {
			if ra := headers.Get("Retry-After"); ra != "" {
				if sec, err := strconv.Atoi(ra); err == nil {
					retryAfter = time.Duration(sec) * time.Second
				}
			}
		}
		return &RateLimitedError{RetryAfter: retryAfter, Body: truncate(text, 300)}
	case http.StatusForbidden:
		// xAI uses 403 for tier/entitlement and sometimes quota messaging.
		// Only the entitlement wording is a tier gate: a quota-403
		// misclassified as tier-forbidden would send the probe and the
		// factory down the reauth path instead of treating it as
		// transient. Both arms used to return TierForbiddenError, which
		// made this check dead code.
		if strings.Contains(lower, "subscription") ||
			strings.Contains(lower, "permission") ||
			strings.Contains(lower, "not have") ||
			strings.Contains(lower, "entitlement") ||
			strings.Contains(lower, "resources") {
			return &TierForbiddenError{StatusCode: status, Body: truncate(text, 300)}
		}
		return fmt.Errorf("xai-oauth: API status %d: %s", status, truncate(text, 400))
	default:
		return fmt.Errorf("xai-oauth: API status %d: %s", status, truncate(text, 400))
	}
}

// chatURL builds the chat completions endpoint.
func chatURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/chat/completions"
}

// maxChatAttempts bounds transient retries on the chat surface: the initial
// attempt plus three retries, mirroring the sibling API-key clients.
const maxChatAttempts = 4

// isTransientChatStatus reports whether a chat status is worth retrying: rate
// limits and the vendor-failing 5xx range. Anything else is the request being
// wrong and surfaces immediately.
func isTransientChatStatus(status int) bool {
	switch status {
	case http.StatusTooManyRequests,
		http.StatusInternalServerError,
		http.StatusBadGateway,
		http.StatusServiceUnavailable,
		http.StatusGatewayTimeout:
		return true
	}
	return false
}

// chatRetryDelay honors Retry-After when the vendor sends one, capped so a
// hostile header cannot stall a turn; otherwise exponential backoff.
// (Local to this package, which cannot import perception's helper without an
// import cycle.)
func chatRetryDelay(headers http.Header, attempt int) time.Duration {
	backoff := time.Duration(1<<uint(attempt)) * time.Second
	if headers == nil {
		return backoff
	}
	if ra := strings.TrimSpace(headers.Get("Retry-After")); ra != "" {
		if secs, err := strconv.Atoi(ra); err == nil && secs >= 0 {
			if d := time.Duration(secs) * time.Second; d <= 60*time.Second {
				return d
			}
			return 60 * time.Second
		}
		if t, err := http.ParseTime(ra); err == nil {
			if d := time.Until(t); d > 0 && d <= 60*time.Second {
				return d
			}
		}
	}
	return backoff
}

// sleepChatCtx waits out a backoff, aborting early on cancellation.
func sleepChatCtx(ctx context.Context, d time.Duration) error {
	if d <= 0 {
		return nil
	}
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// accessTokenWithReload resolves a valid access token, allowing one store
// reload for credentials written after the client was constructed.
func (c *Client) accessTokenWithReload(ctx context.Context) (string, error) {
	token, err := c.tokens.AccessToken(ctx)
	if err != nil {
		if loadErr := c.tokens.Load(); loadErr == nil {
			token, err = c.tokens.AccessToken(ctx)
		}
	}
	return token, err
}

// acquireChatSlot enforces MaxConcurrentCalls. A nil semaphore (see
// DisableSemaphore) means an external scheduler owns concurrency.
func (c *Client) acquireChatSlot(ctx context.Context) (release func(), err error) {
	if c.sem == nil {
		return func() {}, nil
	}
	select {
	case c.sem <- struct{}{}:
		return func() { <-c.sem }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

// doChatRequest is the single transport for every chat-surface method: token
// acquisition, one 401-forced refresh, and bounded transient retry honoring
// Retry-After. Previously each of the three request methods hand-rolled the
// 401 block and none retried anything, so one 503 killed the turn.
func (c *Client) doChatRequest(ctx context.Context, reqBody chatRequest) ([]byte, error) {
	release, err := c.acquireChatSlot(ctx)
	if err != nil {
		return nil, err
	}
	defer release()

	token, err := c.accessTokenWithReload(ctx)
	if err != nil {
		return nil, err
	}

	endpoint := chatURL(c.cfg.BaseURL)
	var lastErr error
	refreshed := false
	for attempt := 0; attempt < maxChatAttempts; attempt++ {
		status, headers, body, err := doJSON(ctx, c.httpClient, http.MethodPost, endpoint, token, reqBody, 10<<20)
		if err != nil {
			if ctx.Err() != nil {
				return nil, ctx.Err()
			}
			lastErr = err
			if sleepErr := sleepChatCtx(ctx, chatRetryDelay(nil, attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}
		if status == http.StatusOK {
			return body, nil
		}
		if status == http.StatusUnauthorized && !refreshed {
			// Force refresh and retry once, inside the attempt budget.
			refreshed = true
			c.tokens.InvalidateAccess()
			token, err = c.accessTokenWithReload(ctx)
			if err != nil {
				return nil, err
			}
			continue
		}
		if isTransientChatStatus(status) {
			lastErr = classifyHTTPError(status, body, headers)
			if sleepErr := sleepChatCtx(ctx, chatRetryDelay(headers, attempt)); sleepErr != nil {
				return nil, sleepErr
			}
			continue
		}
		return nil, classifyHTTPError(status, body, headers)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("xai-oauth: chat request failed without details")
	}
	return nil, fmt.Errorf("%w (after %d attempts)", lastErr, maxChatAttempts)
}
