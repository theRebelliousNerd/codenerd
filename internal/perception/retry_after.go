package perception

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// retryAfterHeader parses a Retry-After header as delta-seconds or an
// HTTP-date. ok is false when there is no usable header. A zero or past
// value is present (ok) with d == 0: the vendor said retry now.
func retryAfterHeader(resp *http.Response) (d time.Duration, ok bool) {
	if resp == nil {
		return 0, false
	}
	raw := strings.TrimSpace(resp.Header.Get("Retry-After"))
	if raw == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(raw); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if when, err := http.ParseTime(raw); err == nil {
		delay := time.Until(when)
		if delay < 0 {
			delay = 0
		}
		return delay, true
	}
	return 0, false
}

// parseRetryAfter returns the Retry-After delay, or 0 when there is no
// usable header. Callers treat 0 as "no header"; that is unchanged.
func parseRetryAfter(resp *http.Response) time.Duration {
	d, ok := retryAfterHeader(resp)
	if !ok {
		return 0
	}
	return d
}

// retryAfterCap bounds honoured Retry-After values so a hostile or buggy
// header cannot stall a shard.
const retryAfterCap = 60 * time.Second

// retryDelay computes how long to wait before the next attempt, honouring a
// Retry-After header when the vendor sends one. Meta's contributor tier is
// limited to 60 requests/minute and does send it, so obeying beats guessing.
//
// "Retry-After: 0" and a past date mean retry now (zero wait); a missing or
// unparseable header falls back to backoff; a large delta or far date is capped.
func retryDelay(resp *http.Response, attempt int) time.Duration {
	backoff := llmRetryBackoff(attempt + 1)
	d, ok := retryAfterHeader(resp)
	if !ok {
		return backoff
	}
	if d > retryAfterCap {
		return retryAfterCap
	}
	return d
}
