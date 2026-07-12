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
func doJSON(ctx context.Context, httpClient *http.Client, method, urlStr, accessToken string, payload any, maxBody int64) (status int, body []byte, err error) {
	if maxBody <= 0 {
		maxBody = 10 << 20
	}
	var reader io.Reader
	if payload != nil {
		data, mErr := json.Marshal(payload)
		if mErr != nil {
			return 0, nil, fmt.Errorf("marshal request: %w", mErr)
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequestWithContext(ctx, method, urlStr, reader)
	if err != nil {
		return 0, nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := httpClient.Do(req)
	if err != nil {
		return 0, nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()
	body, err = io.ReadAll(io.LimitReader(resp.Body, maxBody))
	if err != nil {
		return resp.StatusCode, nil, fmt.Errorf("read response: %w", err)
	}
	return resp.StatusCode, body, nil
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
		if strings.Contains(lower, "subscription") ||
			strings.Contains(lower, "permission") ||
			strings.Contains(lower, "not have") ||
			strings.Contains(lower, "entitlement") ||
			strings.Contains(lower, "resources") {
			return &TierForbiddenError{StatusCode: status, Body: truncate(text, 300)}
		}
		return &TierForbiddenError{StatusCode: status, Body: truncate(text, 300)}
	default:
		return fmt.Errorf("xai-oauth: API status %d: %s", status, truncate(text, 400))
	}
}

// chatURL builds the chat completions endpoint.
func chatURL(baseURL string) string {
	return strings.TrimRight(baseURL, "/") + "/chat/completions"
}
