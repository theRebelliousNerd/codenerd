package broker

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/types"
)

type countServer struct {
	*httptest.Server
	hits     atomic.Int64
	lastBody atomic.Value // []byte
}

func newCountServer(t *testing.T, tokens int, status int) *countServer {
	t.Helper()
	cs := &countServer{}
	cs.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cs.hits.Add(1)
		body, _ := io.ReadAll(r.Body)
		cs.lastBody.Store(body)

		if r.URL.Path != "/messages/count_tokens" {
			t.Errorf("counter hit the wrong path: %s", r.URL.Path)
		}
		if got := r.Header.Get("x-api-key"); got != "test-key" {
			t.Errorf("x-api-key = %q", got)
		}
		if got := r.Header.Get("anthropic-version"); got == "" {
			t.Error("anthropic-version header missing")
		}

		w.WriteHeader(status)
		if status == http.StatusOK {
			_ = json.NewEncoder(w).Encode(map[string]int{"input_tokens": tokens})
		} else {
			_, _ = w.Write([]byte(`{"error":"nope"}`))
		}
	}))
	t.Cleanup(cs.Close)
	return cs
}

func (cs *countServer) body(t *testing.T) map[string]any {
	t.Helper()
	raw, _ := cs.lastBody.Load().([]byte)
	var decoded map[string]any
	if err := json.Unmarshal(raw, &decoded); err != nil {
		t.Fatalf("count body was not valid JSON: %v (%s)", err, raw)
	}
	return decoded
}

func TestAnthropicCounterUsesProviderEndpoint(t *testing.T) {
	srv := newCountServer(t, 4242, http.StatusOK)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))

	count, err := counter.Count(context.Background(), &Request{
		Model:  "claude-opus-5",
		System: "you are a reviewer",
		User:   "review this",
	})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}

	if count.Tokens != 4242 {
		t.Errorf("tokens = %d, want the provider's 4242", count.Tokens)
	}
	if count.Confidence != ConfidenceExact {
		t.Errorf("confidence = %q, want exact — the provider counted this", count.Confidence)
	}
	if count.Source != "anthropic.count_tokens" {
		t.Errorf("source = %q", count.Source)
	}
	if count.Segments.Total() != count.Tokens {
		t.Errorf("segments (%d) do not sum to the authoritative total (%d)",
			count.Segments.Total(), count.Tokens)
	}
}

func TestAnthropicCounterSendsSystemMessagesAndTools(t *testing.T) {
	// The count is only as good as the body it counts. If this drifts from what
	// the client actually sends, the number is quietly wrong.
	srv := newCountServer(t, 100, http.StatusOK)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))

	_, err := counter.Count(context.Background(), &Request{
		Model:  "claude-opus-5",
		System: "SYSTEM_MARKER",
		Messages: []types.Message{
			{Role: "user", Text: "USER_MARKER"},
			{Role: "assistant", ToolCalls: []types.ToolCall{{ID: "t1", Name: "run", Input: map[string]any{"a": 1}}}},
			{Role: "user", ToolResults: []types.ToolResult{{ToolUseID: "t1", Content: "RESULT_MARKER"}}},
		},
		Tools: []types.ToolDefinition{{Name: "run", Description: "TOOL_MARKER", InputSchema: map[string]any{"type": "object"}}},
	})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}

	raw, _ := srv.lastBody.Load().([]byte)
	for _, marker := range []string{"SYSTEM_MARKER", "USER_MARKER", "RESULT_MARKER", "TOOL_MARKER", "claude-opus-5"} {
		if !strings.Contains(string(raw), marker) {
			t.Errorf("count body omitted %s; the counted request is not the request being sent", marker)
		}
	}

	body := srv.body(t)
	msgs, _ := body["messages"].([]any)
	if len(msgs) == 0 {
		t.Fatal("count body carried no messages; the endpoint rejects that")
	}
	first, _ := msgs[0].(map[string]any)
	if first["role"] != "user" {
		t.Errorf("first message role = %v, want user — the endpoint requires it", first["role"])
	}
}

func TestAnthropicCounterSynthesizesLeadingUserTurn(t *testing.T) {
	// An assistant-first history is legal internally but rejected by the
	// endpoint. Without the synthesized turn every such call would silently
	// fall back to the estimator.
	srv := newCountServer(t, 50, http.StatusOK)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))

	count, err := counter.Count(context.Background(), &Request{
		Model:    "claude-opus-5",
		Messages: []types.Message{{Role: "assistant", Text: "I already said something"}},
	})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count.Confidence != ConfidenceExact {
		t.Errorf("assistant-first history fell back to the estimator: %q", count.Confidence)
	}

	msgs, _ := srv.body(t)["messages"].([]any)
	first, _ := msgs[0].(map[string]any)
	if first["role"] != "user" {
		t.Errorf("leading user turn was not synthesized: %v", first["role"])
	}
}

func TestAnthropicCounterCachesRepeatedRequests(t *testing.T) {
	// System prompts and tool schemas repeat almost verbatim across the turns
	// of a session. Paying a round trip for each one puts provider latency on
	// the hot path of every turn.
	srv := newCountServer(t, 777, http.StatusOK)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))

	req := &Request{Model: "claude-opus-5", System: "stable system prompt", User: "same question"}
	for i := 0; i < 5; i++ {
		count, err := counter.Count(context.Background(), req)
		if err != nil {
			t.Fatalf("Count %d: %v", i, err)
		}
		if count.Tokens != 777 {
			t.Errorf("call %d returned %d tokens", i, count.Tokens)
		}
	}

	if hits := srv.hits.Load(); hits != 1 {
		t.Errorf("identical requests hit the endpoint %d times, want 1", hits)
	}

	// Different content must miss the cache.
	if _, err := counter.Count(context.Background(), &Request{
		Model: "claude-opus-5", System: "stable system prompt", User: "a different question",
	}); err != nil {
		t.Fatalf("Count: %v", err)
	}
	if hits := srv.hits.Load(); hits != 2 {
		t.Errorf("different content did not miss the cache: %d hits", hits)
	}
}

func TestAnthropicCounterDegradesToEstimatorOnServerError(t *testing.T) {
	srv := newCountServer(t, 0, http.StatusInternalServerError)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))

	count, err := counter.Count(context.Background(), &Request{
		Model: "claude-opus-5", System: strings.Repeat("s", 4000),
	})
	if err != nil {
		t.Fatalf("a failing endpoint must degrade, not error: %v", err)
	}
	if count.Confidence == ConfidenceExact {
		t.Error("a failed count was labelled exact; the confidence label is what every admission decision trusts")
	}
	if count.Tokens <= 0 {
		t.Error("fallback produced no count")
	}
}

func TestAnthropicCounterDegradesWithoutCredentials(t *testing.T) {
	counter := NewAnthropicCounter("", "https://unused.invalid", nil, NewEstimatingCounter(NewCalibrator()))

	count, err := counter.Count(context.Background(), &Request{Model: "m", System: strings.Repeat("s", 2000)})
	if err != nil {
		t.Fatalf("no credential must degrade, not error: %v", err)
	}
	if count.Confidence == ConfidenceExact {
		t.Errorf("confidence = %q with no credential", count.Confidence)
	}
}

func TestAnthropicCounterDegradesOnSlowEndpoint(t *testing.T) {
	// An exact number that arrives after the user has given up is worse than a
	// calibrated estimate that arrives now.
	slow := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		time.Sleep(300 * time.Millisecond)
		_ = json.NewEncoder(w).Encode(map[string]int{"input_tokens": 1})
	}))
	defer slow.Close()

	client := &http.Client{Timeout: 20 * time.Millisecond}
	counter := NewAnthropicCounter("test-key", slow.URL, client, NewEstimatingCounter(NewCalibrator()))

	start := time.Now()
	count, err := counter.Count(context.Background(), &Request{Model: "m", System: strings.Repeat("s", 2000)})
	elapsed := time.Since(start)

	if err != nil {
		t.Fatalf("a slow endpoint must degrade, not error: %v", err)
	}
	if count.Confidence == ConfidenceExact {
		t.Error("a timed-out count was labelled exact")
	}
	if elapsed > 250*time.Millisecond {
		t.Errorf("counting blocked for %v; the timeout did not bound the hot path", elapsed)
	}
}

func TestAnthropicCounterRejectsNonsenseTokenCounts(t *testing.T) {
	// A 200 carrying zero tokens is not a count. Trusting it would admit an
	// arbitrarily large request against a budget that thought it was empty.
	srv := newCountServer(t, 0, http.StatusOK)
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), NewEstimatingCounter(NewCalibrator()))

	count, err := counter.Count(context.Background(), &Request{Model: "m", System: strings.Repeat("s", 2000)})
	if err != nil {
		t.Fatalf("Count: %v", err)
	}
	if count.Confidence == ConfidenceExact {
		t.Error("a zero-token response was accepted as an exact count")
	}
	if count.Tokens <= 0 {
		t.Error("fallback did not produce a usable count")
	}
}

func TestAnthropicCounterTrainsItsFallback(t *testing.T) {
	// The exact path needs no calibration, but the fallback serves every
	// request made while the endpoint is unreachable. Keeping it trained during
	// the good times is what makes it useful during the bad ones.
	srv := newCountServer(t, 1000, http.StatusOK)
	estimator := NewEstimatingCounter(NewCalibrator())
	counter := NewAnthropicCounter("test-key", srv.URL, srv.Client(), estimator)

	counter.Observe(Observation{Model: "claude-opus-5", Chars: 8000, ActualInputTokens: 2000})

	if n := estimator.Calibrator().Observations("claude-opus-5"); n != 1 {
		t.Errorf("observation did not reach the fallback estimator: n=%d", n)
	}
}
