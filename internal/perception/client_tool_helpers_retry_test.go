package perception

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
	"time"

	"codenerd/internal/config"
)

// TestOpenAIRetryBackoff pins the delay computation for ExecuteOpenAIRequest:
// ordinary transients double exponentially from base capped at max, and after
// a 429 the response's Retry-After governs (falling back to max when absent,
// capped at max when max > 0). max <= 0 means uncapped for ordinary transients.
func TestOpenAIRetryBackoff(t *testing.T) {
	base := time.Second
	max := 30 * time.Second
	tests := []struct {
		name             string
		attempt          int
		max              time.Duration
		lastWasRateLimit bool
		retryAfter       time.Duration
		want             time.Duration
	}{
		{"transient attempt 1", 1, max, false, 0, 1 * time.Second},
		{"transient attempt 2", 2, max, false, 0, 2 * time.Second},
		{"transient attempt 3", 3, max, false, 0, 4 * time.Second},
		{"transient attempt 7 caps at max", 7, max, false, 0, 30 * time.Second},
		{"429 no retry-after uses max", 2, max, true, 0, 30 * time.Second},
		{"429 honors retry-after 2s", 2, max, true, 2 * time.Second, 2 * time.Second},
		{"429 retry-after capped at max", 2, max, true, 90 * time.Second, 30 * time.Second},
		{"max 0 uncapped transient", 3, 0, false, 0, 4 * time.Second},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := openAIRetryBackoff(tc.attempt, base, tc.max, tc.lastWasRateLimit, tc.retryAfter)
			if got != tc.want {
				t.Fatalf("openAIRetryBackoff(%d, %v, %v, %v, %v) = %v, want %v",
					tc.attempt, base, tc.max, tc.lastWasRateLimit, tc.retryAfter, got, tc.want)
			}
		})
	}
}

// TestExecuteOpenAIRequestRetryAfterHeader pins the HTTP 429 path: the first
// response carries "Retry-After: 1", so the retry waits exactly that long and
// the second attempt's completion is returned after exactly 2 requests.
// (A Retry-After of 0 cannot be tested this way: the backoff then becomes the
// configured max, which would stall the test.)
func TestExecuteOpenAIRequestRetryAfterHeader(t *testing.T) {
	prev := config.GetLLMTimeouts()
	config.SetLLMTimeouts(config.LLMTimeouts{
		MaxRetries:       3,
		RetryBackoffBase: time.Second,
		RetryBackoffMax:  30 * time.Second,
	})
	defer config.SetLLMTimeouts(prev)

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Retry-After", "1")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`rate limited`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	start := time.Now()
	resp, err := ExecuteOpenAIRequest(context.Background(), srv.Client(), srv.URL, "key", OpenAIRequest{})
	if err != nil {
		t.Fatalf("ExecuteOpenAIRequest() error = %v, want nil", err)
	}
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp = %+v, want a completion with nil Error", resp)
	}
	if len(resp.Choices) != 1 || resp.Choices[0].Message.Content != "ok" {
		t.Fatalf("unexpected choices: %+v", resp.Choices)
	}
	if requests != 2 {
		t.Fatalf("server saw %d requests, want 2", requests)
	}
	if elapsed := time.Since(start); elapsed < time.Second {
		t.Fatalf("elapsed %v, want at least the 1s Retry-After honored", elapsed)
	}
}

// TestExecuteOpenAIRequestQuotaExhaustedFailsFast pins the F-RL-2a fix: an
// HTTP 429 carrying an exhausted daily quota (Remaining 0 with a Reset far
// beyond the remaining retry budget) returns a *QuotaExhaustedError after
// exactly ONE request instead of burning the whole retry budget.
func TestExecuteOpenAIRequestQuotaExhaustedFailsFast(t *testing.T) {
	prev := config.GetLLMTimeouts()
	config.SetLLMTimeouts(config.LLMTimeouts{
		MaxRetries:       3,
		RetryBackoffBase: time.Millisecond,
		RetryBackoffMax:  10 * time.Millisecond,
	})
	defer config.SetLLMTimeouts(prev)

	// Live OpenRouter body from the 2026-09-17 incident (free-tier daily
	// quota exhausted; reset moved relative to now so the test does not expire).
	reset := strconv.FormatInt(time.Now().Add(24*time.Hour).UnixMilli(), 10)
	liveBody := fmt.Sprintf(`{"error":{"message":"Rate limit exceeded: free-models-per-day-stealth. ","code":429,"metadata":{"headers":{"X-RateLimit-Limit":"1000","X-RateLimit-Remaining":"0","X-RateLimit-Reset":"%s"},"limit_source":"openrouter_free_tier_daily","remedy_hint":"Wait for the daily reset (see X-RateLimit-Reset), or purchase credits to raise your free-model daily limit.","provider_name":null}},"user_id":"user_x"}`, reset)

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(liveBody))
	}))
	defer srv.Close()

	_, err := ExecuteOpenAIRequest(context.Background(), srv.Client(), srv.URL, "key", OpenAIRequest{})
	if err == nil {
		t.Fatalf("ExecuteOpenAIRequest() error = nil, want *QuotaExhaustedError")
	}
	var quotaErr *QuotaExhaustedError
	if !errors.As(err, &quotaErr) {
		t.Fatalf("ExecuteOpenAIRequest() error = %v (%T), want *QuotaExhaustedError", err, err)
	}
	if requests != 1 {
		t.Fatalf("server saw %d requests, want exactly 1 (fail fast, no retries)", requests)
	}
}

// TestExecuteOpenAIRequestBurstRateLimitStillRetries pins that a 429 with
// quota remaining (a short burst, not an exhausted quota) keeps the
// pre-existing retry behavior: the second attempt's completion is returned.
func TestExecuteOpenAIRequestBurstRateLimitStillRetries(t *testing.T) {
	prev := config.GetLLMTimeouts()
	config.SetLLMTimeouts(config.LLMTimeouts{
		MaxRetries:       3,
		RetryBackoffBase: time.Millisecond,
		RetryBackoffMax:  10 * time.Millisecond,
	})
	defer config.SetLLMTimeouts(prev)

	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		if requests == 1 {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"error":{"message":"Rate limit exceeded: burst.","code":429,"metadata":{"headers":{"X-RateLimit-Limit":"1000","X-RateLimit-Remaining":"5","X-RateLimit-Reset":"1789689600000"},"limit_source":"openrouter_free_tier_daily","provider_name":null}}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"id":"chatcmpl-1","object":"chat.completion","created":1,"model":"m","choices":[{"index":0,"message":{"role":"assistant","content":"ok"},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	resp, err := ExecuteOpenAIRequest(context.Background(), srv.Client(), srv.URL, "key", OpenAIRequest{})
	if err != nil {
		t.Fatalf("ExecuteOpenAIRequest() error = %v, want nil", err)
	}
	if resp == nil || resp.Error != nil {
		t.Fatalf("resp = %+v, want a completion with nil Error", resp)
	}
	if requests != 2 {
		t.Fatalf("server saw %d requests, want 2 (burst 429 retried)", requests)
	}
}
