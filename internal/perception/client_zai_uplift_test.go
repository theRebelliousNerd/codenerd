package perception

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
)

func zaiTestClient(baseURL string) *ZAIClient {
	return NewZAIClientWithConfig(ZAIConfig{
		APIKey:           "test-key",
		BaseURL:          baseURL,
		Model:            "glm-test",
		Timeout:          time.Minute,
		MaxRetries:       3,
		RetryBackoffBase: time.Second,
		RetryBackoffMax:  30 * time.Second,
	})
}

func always429(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
}

// When the retry backoff does not fit inside the context deadline, the
// client must fail loudly. The old code returned ctx.Err() — which is nil
// when the deadline has not fired yet — reporting ("", nil): an empty
// SUCCESS. Backoff here is jittered 0.5-1.5s against a 300ms deadline,
// so the pre-check always triggers deterministically.
func TestZAIChat_BackoffExceedingDeadlineFailsLoud(t *testing.T) {
	srv := always429(t)
	defer srv.Close()
	c := zaiTestClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	start := time.Now()
	resp, err := c.CompleteWithSystem(ctx, "sys", "hi")
	elapsed := time.Since(start)

	if err == nil {
		t.Fatalf("resp = %q, err = nil: want a loud deadline error, not empty success", resp)
	}
	if !strings.Contains(err.Error(), "would exceed context deadline") {
		t.Errorf("err = %v, want the deadline pre-check message", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want wrapped context.DeadlineExceeded", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("elapsed = %v, want fail-fast with no backoff sleep", elapsed)
	}
}

// Same pre-check bug lived in the structured path; same loud failure.
func TestZAIStructured_BackoffExceedingDeadlineFailsLoud(t *testing.T) {
	srv := always429(t)
	defer srv.Close()
	c := zaiTestClient(srv.URL)
	ctx, cancel := context.WithTimeout(context.Background(), 300*time.Millisecond)
	defer cancel()

	resp, err := c.CompleteWithStructuredOutput(ctx, "sys", "hi", false)
	if err == nil {
		t.Fatalf("resp = %q, err = nil: want a loud deadline error, not empty success", resp)
	}
	if !strings.Contains(err.Error(), "would exceed context deadline") {
		t.Errorf("err = %v, want the deadline pre-check message", err)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("err = %v, want wrapped context.DeadlineExceeded", err)
	}
}

// The tools path shares the retryable-status policy with the chat paths:
// a transient 503 is retried rather than failing the turn. The old tools
// loop failed immediately on any 5xx.
func TestZAITools_RetriesTransientThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id": "zai-1", "object": "chat.completion", "created": 1, "model": "glm-test",
		  "choices": [{
		    "index": 0,
		    "message": {
		      "role": "assistant", "content": "",
		      "tool_calls": [{
		        "id": "call_1", "type": "function",
		        "function": {"name": "get_time", "arguments": "{\"zone\":\"UTC\"}"}
		      }]
		    },
		    "finish_reason": "tool_calls"
		  }],
		  "usage": {"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15}
		}`))
	}))
	defer srv.Close()

	c := zaiTestClient(srv.URL)
	tools := []ToolDefinition{{
		Name:        "get_time",
		Description: "Get the time",
		InputSchema: map[string]any{"type": "object"},
	}}
	resp, err := c.CompleteWithTools(context.Background(), "sys", "time?", tools)
	if err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one transient retried)", got)
	}
	if resp.StopReason != "tool_use" {
		t.Errorf("StopReason = %q, want standardized tool_use", resp.StopReason)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_time" {
		t.Fatalf("ToolCalls = %+v, want one get_time call", resp.ToolCalls)
	}
	if zone, _ := resp.ToolCalls[0].Input["zone"].(string); zone != "UTC" {
		t.Errorf("tool input zone = %q, want UTC", zone)
	}
}

// The tools loop must honor the configured retry budget like the chat
// paths do. The old loop hardcoded 3 retries regardless of config.
func TestZAITools_HonorsConfiguredMaxRetries(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"rate limited"}}`))
	}))
	defer srv.Close()

	c := zaiTestClient(srv.URL)
	c.maxRetries = 1
	_, err := c.CompleteWithTools(context.Background(), "sys", "hi", nil)
	if err == nil || !strings.Contains(err.Error(), "max retries exceeded") {
		t.Fatalf("err = %v, want max-retries exhaustion", err)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (initial + 1 retry)", got)
	}
}

// A cancelled turn must exit during the tools backoff via the file's own
// sleepWithContext, not after sleeping through it.
func TestZAITools_CancelledBackoffExitsFast(t *testing.T) {
	srv := always429(t)
	defer srv.Close()
	c := zaiTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := c.CompleteWithTools(ctx, "sys", "hi", nil)
	elapsed := time.Since(start)

	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want bare context.Canceled (file convention)", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("cancelled turn took %v, want fast exit (old code slept 7s)", elapsed)
	}
}

// A length-stop on the chat path must report the real completion ceiling
// in the typed error. The old code passed 0, so the broker's compression
// message dropped the limit it exists to quote.
func TestZAIChat_TruncationReportsRealCeiling(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "cut…"}, "finish_reason": "length"}],
		  "usage": {"prompt_tokens": 10, "completion_tokens": 4096, "total_tokens": 4106}
		}`))
	}))
	defer srv.Close()

	c := zaiTestClient(srv.URL)
	_, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	var trunc *types.OutputTruncated
	if !errors.As(err, &trunc) {
		t.Fatalf("err = %v (%T), want *types.OutputTruncated", err, err)
	}
	if trunc.LimitTokens != c.maxOutputTokens || trunc.LimitTokens <= 0 {
		t.Errorf("LimitTokens = %d, want the real ceiling %d", trunc.LimitTokens, c.maxOutputTokens)
	}
	if trunc.OutputTokens != 4096 {
		t.Errorf("OutputTokens = %d, want 4096", trunc.OutputTokens)
	}
}
