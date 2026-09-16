package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func xaiTestClient(baseURL string) *XAIClient {
	return NewXAIClientWithConfig(XAIConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
		Model:   "grok-test",
		Timeout: time.Minute,
	})
}

// The chat path shares the transient policy with the tools paths (which
// already retried 5xx via the helper): a 503 is retried, not fatal.
func TestXAIChat_RetriesTransientThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "  grok says hi  "}, "finish_reason": "stop"}],
		  "usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5}
		}`))
	}))
	defer srv.Close()

	c := xaiTestClient(srv.URL)
	resp, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "grok says hi" {
		t.Errorf("response = %q, want trimmed content", resp)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one transient retried)", got)
	}
}

// A cancelled turn must exit during the retry backoff, not after sleeping
// through 1+2+4s of it. Same guarantee as ExecuteOpenAIRequest.
func TestXAIChat_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := xaiTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := c.CompleteWithSystem(ctx, "sys", "hi")
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "cancelled during retry backoff") {
		t.Fatalf("err = %v, want cancellation during backoff", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("cancelled turn took %v, want fast exit (old code slept 7s)", elapsed)
	}
}

// The extracted rateLimit must enforce the 100ms inter-request spacing.
func TestXAIRateLimit_EnforcesSpacing(t *testing.T) {
	c := xaiTestClient("http://127.0.0.1:1")
	c.rateLimit()
	start := time.Now()
	c.rateLimit()
	if gap := time.Since(start); gap < 90*time.Millisecond {
		t.Errorf("rateLimit gap = %v, want >= 90ms", gap)
	}
}
