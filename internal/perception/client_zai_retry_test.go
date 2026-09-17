package perception

import (
	"testing"
	"time"
)

func TestContextRemaining(t *testing.T) {
	if d, ok := contextRemaining(time.Time{}); ok || d != 0 {
		t.Errorf("zero deadline: got (%v,%v), want (0,false)", d, ok)
	}
	d, ok := contextRemaining(time.Now().Add(time.Hour))
	if !ok || d <= 0 {
		t.Errorf("future deadline: got (%v,%v), want (positive,true)", d, ok)
	}
}

func TestMaxDuration(t *testing.T) {
	if maxDuration(3*time.Second, time.Second) != 3*time.Second {
		t.Error("maxDuration should return the larger value")
	}
	if maxDuration(time.Second, 3*time.Second) != 3*time.Second {
		t.Error("maxDuration should be order-independent")
	}
}

func TestShouldRetryStatus(t *testing.T) {
	for _, code := range []int{429, 408, 500, 502, 503, 504} {
		if !shouldRetryStatus(code) {
			t.Errorf("status %d should be retryable", code)
		}
	}
	for _, code := range []int{200, 201, 400, 401, 403, 404} {
		if shouldRetryStatus(code) {
			t.Errorf("status %d should NOT be retryable", code)
		}
	}
}

func TestJitterDuration(t *testing.T) {
	c := NewZAIClient("test-key")
	if c.jitterDuration(0) != 0 || c.jitterDuration(-time.Second) != 0 {
		t.Error("non-positive input should yield 0 jitter")
	}
	// Jitter factor is in [0.5, 1.5), so the result stays within those bounds.
	base := time.Second
	for i := 0; i < 20; i++ {
		got := c.jitterDuration(base)
		if got < time.Duration(0.5*float64(base)) || got >= time.Duration(1.5*float64(base)) {
			t.Fatalf("jittered %v out of [0.5x,1.5x) bounds", got)
		}
	}
}

func TestNextRetryDelay(t *testing.T) {
	c := NewZAIClient("test-key")
	if c.nextRetryDelay(1) <= 0 {
		t.Error("retry delay should be positive")
	}
	// attempt < 1 is treated as attempt 1.
	if c.nextRetryDelay(0) <= 0 {
		t.Error("attempt 0 should be clamped to a positive delay")
	}
	// A very large attempt is capped (then jittered by < 1.5x).
	capped := c.nextRetryDelay(50)
	if capped > time.Duration(1.5*float64(c.retryBackoffMax)) {
		t.Errorf("large-attempt delay %v exceeds 1.5x the cap %v", capped, c.retryBackoffMax)
	}
}
