package perception

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func openrouterTestClient(baseURL string) *OpenRouterClient {
	return NewOpenRouterClientWithConfig(OpenRouterConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
		Model:   "test/model",
		Timeout: time.Minute,
	})
}

// sseFlakyServer fails the first failCount requests with failStatus, then
// serves a two-chunk SSE stream ("hel"+"lo") with a trailing usage chunk.
// Shared by the OpenRouter and OpenAI streaming setup-retry pins.
func sseFlakyServer(t *testing.T, failCount int32, failStatus int, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= failCount {
			w.WriteHeader(failStatus)
			_, _ = w.Write([]byte(`{"error":{"message":"try again"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, content := range []string{"hel", "lo"} {
			fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n", content)
		}
		fmt.Fprint(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":5,\"completion_tokens\":2}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

func drainStream(t *testing.T, contentCh <-chan string, errCh <-chan error) string {
	t.Helper()
	var sb strings.Builder
	for contentCh != nil || errCh != nil {
		select {
		case delta, ok := <-contentCh:
			if !ok {
				contentCh = nil
				continue
			}
			sb.WriteString(delta)
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				t.Fatalf("stream error: %v", err)
			}
		case <-time.After(15 * time.Second):
			t.Fatal("timed out draining stream")
		}
	}
	return sb.String()
}

// A cancelled turn must exit during the retry backoff, not after sleeping
// through 1+2+4s of it. Same guarantee as ExecuteOpenAIRequest.
func TestOpenRouterChat_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := openrouterTestClient(srv.URL)
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

// The chat path shares the transient policy with the tools path (which
// already retried 5xx via the helper): a 503 is retried, not fatal.
func TestOpenRouterChat_RetriesTransientThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "  routed ok  "}, "finish_reason": "stop"}],
		  "usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5}
		}`))
	}))
	defer srv.Close()

	c := openrouterTestClient(srv.URL)
	resp, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "routed ok" {
		t.Errorf("response = %q, want trimmed content", resp)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one transient retried)", got)
	}
}

// The streaming setup loop reports via errorChan; cancellation must
// surface there just as fast.
func TestOpenRouterStreaming_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := openrouterTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, errCh := c.CompleteWithStreaming(ctx, "sys", "hi", false)
	select {
	case err := <-errCh:
		elapsed := time.Since(start)
		if err == nil || !strings.Contains(err.Error(), "cancelled during retry backoff") {
			t.Fatalf("err = %v, want cancellation during backoff", err)
		}
		if elapsed > 3*time.Second {
			t.Errorf("cancelled stream took %v, want fast exit (old code slept 7s)", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for stream error")
	}
}

// A transient failure during stream setup is retried, and the recovered
// stream still delivers every delta in order.
func TestOpenRouterStreaming_RetriesTransientSetup(t *testing.T) {
	var hits atomic.Int32
	srv := sseFlakyServer(t, 1, http.StatusServiceUnavailable, &hits)
	defer srv.Close()

	c := openrouterTestClient(srv.URL)
	contentCh, errCh := c.CompleteWithStreaming(context.Background(), "sys", "hi", false)
	if got := drainStream(t, contentCh, errCh); got != "hello" {
		t.Errorf("streamed content = %q, want hello", got)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server hits = %d, want 2 (one setup retry)", n)
	}
}

// The extracted rateLimit must preserve the 100ms inter-request spacing
// the inline blocks enforced.
func TestOpenRouterRateLimit_PreservesSpacing(t *testing.T) {
	arrivals := make(chan time.Time, 4)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		arrivals <- time.Now()
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
		  "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer srv.Close()

	c := openrouterTestClient(srv.URL)
	ctx := context.Background()
	if _, err := c.CompleteWithSystem(ctx, "sys", "one"); err != nil {
		t.Fatalf("first call: %v", err)
	}
	if _, err := c.CompleteWithSystem(ctx, "sys", "two"); err != nil {
		t.Fatalf("second call: %v", err)
	}
	first := <-arrivals
	second := <-arrivals
	if gap := second.Sub(first); gap < 90*time.Millisecond {
		t.Errorf("inter-request gap = %v, want >= 90ms", gap)
	}
}
