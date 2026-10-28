package typesafe

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// RetryPolicy bounds the retries of a 429 or 529. The wait before retry n is
// Base*2^n up to Cap, or the reply's retry-after when it has one. A wait over
// MaxWait is not slept: the error is returned instead of stalling the caller.
type RetryPolicy struct {
	Max     int // retries after the first attempt
	Base    time.Duration
	Cap     time.Duration
	MaxWait time.Duration
}

// DefaultRetryPolicy is five attempts, one to eight seconds apart.
var DefaultRetryPolicy = RetryPolicy{Max: 4, Base: time.Second, Cap: 8 * time.Second, MaxWait: time.Minute}

func (p RetryPolicy) backoff(retry int) time.Duration {
	d := p.Base
	for i := 0; i < retry && d < p.Cap; i++ {
		d *= 2
	}
	if d > p.Cap {
		d = p.Cap
	}
	return d
}

// wait is how long to sleep before retry number retry (from zero).
func (p RetryPolicy) wait(retry int, e *APIError) time.Duration {
	if e.RetryAfter > 0 {
		return e.RetryAfter
	}
	return p.backoff(retry)
}

// parseRetryAfter reads delta-seconds or an HTTP date; anything else is zero.
func parseRetryAfter(h http.Header, now time.Time) time.Duration {
	v := strings.TrimSpace(h.Get("Retry-After"))
	if v == "" {
		return 0
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0
		}
		return time.Duration(secs) * time.Second
	}
	if t, err := http.ParseTime(v); err == nil {
		if d := t.Sub(now); d > 0 {
			return d
		}
	}
	return 0
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-t.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
