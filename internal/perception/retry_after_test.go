package perception

import (
	"net/http"
	"testing"
	"time"
)

// Meta's contributor tier is capped at 60 RPM and sends Retry-After; honouring
// it beats guessing with a fixed backoff.
func TestRetryDelay_HonorsRetryAfter(t *testing.T) {
	resp := &http.Response{Header: http.Header{}}
	resp.Header.Set("Retry-After", "7")
	if got := retryDelay(resp, 0); got != 7*time.Second {
		t.Errorf("retryDelay = %v, want 7s", got)
	}

	// A hostile or buggy header must not stall a shard indefinitely.
	resp.Header.Set("Retry-After", "99999")
	if got := retryDelay(resp, 0); got != 60*time.Second {
		t.Errorf("retryDelay = %v, want the 60s cap", got)
	}

	// No header falls back to exponential backoff.
	if got := retryDelay(&http.Response{Header: http.Header{}}, 2); got != 4*time.Second {
		t.Errorf("backoff = %v, want 4s", got)
	}
	if got := retryDelay(nil, 0); got != time.Second {
		t.Errorf("nil-response backoff = %v, want 1s", got)
	}

	// "Retry-After: 0" means retry now (zero wait).
	resp.Header.Set("Retry-After", "0")
	if got := retryDelay(resp, 1); got != 0 {
		t.Errorf("zero Retry-After = %v, want 0", got)
	}

	// An HTTP-date far in the future is capped like a large delta.
	future := &http.Response{Header: http.Header{}}
	future.Header.Set("Retry-After", time.Now().UTC().Add(10*time.Minute).Format(http.TimeFormat))
	if got := retryDelay(future, 0); got != 60*time.Second {
		t.Errorf("far-future date = %v, want the 60s cap", got)
	}

	// An HTTP-date in the past means retry now (zero wait).
	past := &http.Response{Header: http.Header{}}
	past.Header.Set("Retry-After", time.Now().UTC().Add(-time.Hour).Format(http.TimeFormat))
	if got := retryDelay(past, 1); got != 0 {
		t.Errorf("past-date = %v, want 0", got)
	}

	// An unparseable header falls back to backoff.
	bad := &http.Response{Header: http.Header{}}
	bad.Header.Set("Retry-After", "garbage")
	if got := retryDelay(bad, 2); got != 4*time.Second {
		t.Errorf("garbage backoff = %v, want 4s", got)
	}
}

func TestParseRetryAfter(t *testing.T) {
	if parseRetryAfter(nil) != 0 {
		t.Error("nil response should yield 0")
	}
	mk := func(v string) *http.Response {
		r := &http.Response{Header: http.Header{}}
		if v != "" {
			r.Header.Set("Retry-After", v)
		}
		return r
	}
	if parseRetryAfter(mk("")) != 0 {
		t.Error("missing header should yield 0")
	}
	if got := parseRetryAfter(mk("5")); got != 5*time.Second {
		t.Errorf("numeric Retry-After=5 -> %v, want 5s", got)
	}
	if got := parseRetryAfter(mk("0")); got != 0 {
		t.Errorf("non-positive Retry-After -> %v, want 0", got)
	}
	if got := parseRetryAfter(mk("garbage")); got != 0 {
		t.Errorf("unparseable Retry-After -> %v, want 0", got)
	}
	// HTTP-date in the future yields a positive delay.
	future := time.Now().UTC().Add(30 * time.Second).Format(http.TimeFormat)
	if got := parseRetryAfter(mk(future)); got <= 0 {
		t.Errorf("future HTTP-date -> %v, want positive", got)
	}
	// HTTP-date in the past clamps to 0.
	past := time.Now().UTC().Add(-time.Hour).Format(http.TimeFormat)
	if got := parseRetryAfter(mk(past)); got != 0 {
		t.Errorf("past HTTP-date -> %v, want 0", got)
	}
}

func TestRetryAfterHeader(t *testing.T) {
	mk := func(v string) *http.Response {
		r := &http.Response{Header: http.Header{}}
		if v != "" {
			r.Header.Set("Retry-After", v)
		}
		return r
	}

	if d, ok := retryAfterHeader(nil); ok || d != 0 {
		t.Errorf("nil response = (%v, %v), want (0, false)", d, ok)
	}
	if d, ok := retryAfterHeader(mk("")); ok || d != 0 {
		t.Errorf("missing header = (%v, %v), want (0, false)", d, ok)
	}
	if d, ok := retryAfterHeader(mk("garbage")); ok || d != 0 {
		t.Errorf("garbage header = (%v, %v), want (0, false)", d, ok)
	}
	if d, ok := retryAfterHeader(mk("-5")); ok || d != 0 {
		t.Errorf("negative delta = (%v, %v), want (0, false)", d, ok)
	}
	if d, ok := retryAfterHeader(mk("0")); !ok || d != 0 {
		t.Errorf("zero delta = (%v, %v), want (0, true)", d, ok)
	}
	if d, ok := retryAfterHeader(mk("7")); !ok || d != 7*time.Second {
		t.Errorf("delta 7 = (%v, %v), want (7s, true)", d, ok)
	}

	future := time.Now().UTC().Add(30 * time.Second).Format(http.TimeFormat)
	if d, ok := retryAfterHeader(mk(future)); !ok || d <= 0 {
		t.Errorf("future date = (%v, %v), want (positive, true)", d, ok)
	}
	past := time.Now().UTC().Add(-time.Hour).Format(http.TimeFormat)
	if d, ok := retryAfterHeader(mk(past)); !ok || d != 0 {
		t.Errorf("past date = (%v, %v), want (0, true)", d, ok)
	}
}
