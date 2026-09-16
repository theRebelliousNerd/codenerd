package perception

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
)

func anthropicTestClient(baseURL string) *AnthropicClient {
	return NewAnthropicClientWithConfig(AnthropicConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
		Model:   "claude-test",
		Timeout: time.Minute,
	})
}

const anthropicChatOK = `{
  "id": "msg_1", "type": "message", "role": "assistant", "model": "claude-test",
  "content": [{"type": "text", "text": "  hello  "}],
  "stop_reason": "end_turn",
  "usage": {"input_tokens": 3, "output_tokens": 7}
}`

// Anthropic's 529 "overloaded" is transient and must be retried. The old
// policy failed instantly on every 5xx while pointlessly retrying 400s.
func TestAnthropicChat_RetriesOverloadedThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(529)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(anthropicChatOK))
	}))
	defer srv.Close()

	c := anthropicTestClient(srv.URL)
	start := time.Now()
	resp, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	elapsed := time.Since(start)
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "hello" {
		t.Errorf("response = %q, want trimmed hello", resp)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one overload retried)", got)
	}
	if elapsed < 900*time.Millisecond {
		t.Errorf("elapsed = %v, want the 1s retry backoff to have been honored", elapsed)
	}
}

// A 400 is deterministic: identical bytes will never succeed, so it must
// fail fast on the first hit — even with a piggyback-flavored prompt and a
// schema/json-flavored body, the exact combination the old code retried
// three times over 7s while claiming to "retry without Piggyback".
func TestAnthropicChat_BadRequestFailsFast(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"type":"error","error":{"type":"invalid_request_error","message":"bad schema: json invalid"}}`))
	}))
	defer srv.Close()

	c := anthropicTestClient(srv.URL)
	start := time.Now()
	_, err := c.CompleteWithSystem(context.Background(), "sys control_packet", "PiggybackEnvelope ping")
	elapsed := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "status 400") {
		t.Fatalf("err = %v, want immediate 400 failure", err)
	}
	if got := hits.Load(); got != 1 {
		t.Errorf("server hits = %d, want 1 (no identical retry on 400)", got)
	}
	if elapsed > 3*time.Second {
		t.Errorf("elapsed = %v, want fast failure (old code slept 7s)", elapsed)
	}
}

// A cancelled turn must exit during the retry backoff, not after sleeping
// through 1+2+4s of it. Same guarantee as ExecuteOpenAIRequest.
func TestAnthropicChat_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := anthropicTestClient(srv.URL)
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

// The tools path shares postMessages, so a transient 503 is retried rather
// than failing the turn. The old tools path was single-attempt.
func TestAnthropicTools_RetriesTransientThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"api_error","message":"brb"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "id": "msg_2", "type": "message", "role": "assistant", "model": "claude-test",
		  "content": [
		    {"type": "text", "text": "checking "},
		    {"type": "tool_use", "id": "toolu_1", "name": "get_time", "input": {"zone": "UTC"}}
		  ],
		  "stop_reason": "tool_use",
		  "usage": {"input_tokens": 10, "output_tokens": 5}
		}`))
	}))
	defer srv.Close()

	c := anthropicTestClient(srv.URL)
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
		t.Errorf("StopReason = %q, want tool_use", resp.StopReason)
	}
	if resp.Text != "checking" {
		t.Errorf("Text = %q, want trimmed text block", resp.Text)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "get_time" {
		t.Fatalf("ToolCalls = %+v, want one get_time call", resp.ToolCalls)
	}
	if zone, _ := resp.ToolCalls[0].Input["zone"].(string); zone != "UTC" {
		t.Errorf("tool input zone = %q, want UTC", zone)
	}
	if resp.Usage.TotalTokens != 15 {
		t.Errorf("Usage.TotalTokens = %d, want 15", resp.Usage.TotalTokens)
	}
}

// The tool-results path shares postMessages too: a 429 is retried, and the
// provider-neutral history must reach the wire in Anthropic block form with
// tool_use_id pairing intact.
func TestAnthropicToolResults_Retries429AndMapsHistory(t *testing.T) {
	var hits atomic.Int32
	captured := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusTooManyRequests)
			_, _ = w.Write([]byte(`{"type":"error","error":{"type":"rate_limit_error","message":"slow down"}}`))
			return
		}
		body, _ := io.ReadAll(r.Body)
		captured <- string(body)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(anthropicChatOK))
	}))
	defer srv.Close()

	c := anthropicTestClient(srv.URL)
	history := []types.Message{
		{Role: "user", Text: "what time is it?"},
		{Role: "assistant", Text: "checking", ToolCalls: []types.ToolCall{
			{ID: "toolu_1", Name: "get_time", Input: map[string]any{}},
		}},
		{Role: "user", ToolResults: []types.ToolResult{
			{ToolUseID: "toolu_1", Content: "noon"},
		}},
	}
	tools := []types.ToolDefinition{{
		Name:        "get_time",
		Description: "Get the time",
		InputSchema: map[string]any{"type": "object"},
	}}
	resp, err := c.CompleteWithToolResults(context.Background(), "sys", history, tools)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one 429 retried)", got)
	}
	if resp.Text != "hello" {
		t.Errorf("response = %q, want hello", resp.Text)
	}
	wire := <-captured
	for _, want := range []string{`"tool_use_id":"toolu_1"`, `"type":"tool_result"`, `"type":"tool_use"`, `"content":"noon"`} {
		if !strings.Contains(wire, want) {
			t.Errorf("wire request missing %s:\n%s", want, wire)
		}
	}
}

// The extracted rateLimit must enforce the 100ms inter-request spacing.
func TestAnthropicRateLimit_EnforcesSpacing(t *testing.T) {
	c := anthropicTestClient("http://127.0.0.1:1")
	c.rateLimit()
	start := time.Now()
	c.rateLimit()
	if gap := time.Since(start); gap < 90*time.Millisecond {
		t.Errorf("rateLimit gap = %v, want >= 90ms", gap)
	}
}
