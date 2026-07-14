package perception

import (
	"codenerd/internal/logging"
	"context"
	crand "crypto/rand"
	"math/big"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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
	n, err := crand.Int(crand.Reader, big.NewInt(1000))
	var factor float64
	if err != nil {
		factor = 0.5
	} else {
		factor = 0.5 + (float64(n.Int64()) / 1000.0)
	}
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
	delay := min(base*time.Duration(1<<uint(attempt-1)), maxDelay)
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
	log.StructuredLog("debug", "Rate limit sleep starting", map[string]any{
		"request_id":            reqID,
		"sleep_ms":              delay.Milliseconds(),
		"context_remaining_ms":  remaining.Milliseconds(),
		"has_deadline":          hasDeadline,
		"would_exceed_deadline": hasDeadline && delay > remaining,
	})

	if err := sleepWithContext(ctx, delay); err != nil {
		log.StructuredLog("error", "Rate limit sleep cancelled", map[string]any{
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
