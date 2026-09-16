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

func openaiTestClient(baseURL string) *OpenAIClient {
	return NewOpenAIClientWithConfig(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
		Model:   "gpt-test",
		Timeout: time.Minute,
	})
}

// A cancelled turn must exit during the retry backoff, not after sleeping
// through 1+2+4s of it. Same guarantee as ExecuteOpenAIRequest.
func TestOpenAICompleteWithSystem_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := openaiTestClient(srv.URL)
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

// The streaming retry loop lives in a goroutine and reports via errorChan;
// cancellation must surface there just as fast.
func TestOpenAIStreaming_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := openaiTestClient(srv.URL)
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

// The tools path delegates to the shared helper, so a transient 503 is
// retried rather than failing the turn. The old private loop failed
// immediately on any 5xx.
func TestOpenAICompleteWithTools_RetriesTransientThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= 2 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id": "chatcmpl-x", "object": "chat.completion", "created": 1, "model": "gpt-test",
		  "choices": [{
		    "index": 0,
		    "message": {
		      "role": "assistant", "content": "",
		      "tool_calls": [{
		        "id": "call_1", "type": "function",
		        "function": {"name": "get_weather", "arguments": "{\"city\":\"Oslo\"}"}
		      }]
		    },
		    "finish_reason": "tool_calls"
		  }],
		  "usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	}))
	defer srv.Close()

	c := openaiTestClient(srv.URL)
	tools := []ToolDefinition{{
		Name:        "get_weather",
		Description: "Get weather for a city",
		InputSchema: map[string]any{"type": "object"},
	}}
	resp, err := c.CompleteWithTools(context.Background(), "sys", "weather?", tools)
	if err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	if got := hits.Load(); got != 3 {
		t.Errorf("server hits = %d, want 3 (two transient failures retried)", got)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want standardized tool_use", resp.StopReason)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_weather" {
		t.Fatalf("ToolCalls = %+v, want one get_weather call", resp.ToolCalls)
	}
	if city, _ := resp.ToolCalls[0].Input["city"].(string); city != "Oslo" {
		t.Errorf("tool input city = %q, want Oslo", city)
	}
	if resp.Usage.OutputTokens != 5 || resp.Usage.InputTokens != 10 {
		t.Errorf("Usage = %+v, want 10 in / 5 out", resp.Usage)
	}
}

// An empty key must fail before any HTTP happens, matching the chat path.
func TestOpenAICompleteWithTools_EmptyKeyFailsFast(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{}`))
	}))
	defer srv.Close()

	c := openaiTestClient(srv.URL)
	c.apiKey = ""
	_, err := c.CompleteWithTools(context.Background(), "sys", "hi", nil)
	if err == nil || !strings.Contains(err.Error(), "API key not configured") {
		t.Fatalf("err = %v, want fail-fast key error", err)
	}
	if got := hits.Load(); got != 0 {
		t.Errorf("server hits = %d, want 0 (no request without a key)", got)
	}
}

// The extracted rateLimit must preserve the 100ms inter-request spacing
// the inline blocks enforced.
func TestOpenAIRateLimit_PreservesSpacing(t *testing.T) {
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

	c := openaiTestClient(srv.URL)
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

// The chat path shares the transient policy with the tools path (which
// already retried 5xx via the helper): a 503 is retried, not fatal.
func TestOpenAIChat_RetriesTransientThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "  recovered  "}, "finish_reason": "stop"}],
		  "usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5}
		}`))
	}))
	defer srv.Close()

	c := openaiTestClient(srv.URL)
	resp, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "recovered" {
		t.Errorf("response = %q, want trimmed content", resp)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one transient retried)", got)
	}
}

// A transient failure during stream setup is retried, and the recovered
// stream still delivers every delta in order.
func TestOpenAIStreaming_RetriesTransientSetup(t *testing.T) {
	var hits atomic.Int32
	srv := sseFlakyServer(t, 1, http.StatusServiceUnavailable, &hits)
	defer srv.Close()

	c := openaiTestClient(srv.URL)
	contentCh, errCh := c.CompleteWithStreaming(context.Background(), "sys", "hi", false)
	if got := drainStream(t, contentCh, errCh); got != "hello" {
		t.Errorf("streamed content = %q, want hello", got)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server hits = %d, want 2 (one setup retry)", n)
	}
}
