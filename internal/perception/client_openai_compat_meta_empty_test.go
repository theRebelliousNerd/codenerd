package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

// Meta returns no reasoning text on the chat surface, so a turn spent on
// thinking looks like: content empty, finish_reason "stop", reasoning absent,
// completion tokens billed. Measured 2026-09-17 21:20 on the coder step
// planner: output_tokens 2177 and then 2668, both empty, ~40 s, no recovery.
// The client must retry once asking for minimal reasoning.
func TestCompleteWithSystem_Meta_RetriesWithMinimalReasoningWhenTokensBoughtNothing(t *testing.T) {
	var calls int32
	var secondEffort string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&calls, 1)
		var body OpenAIRequest
		_ = json.NewDecoder(r.Body).Decode(&body)
		w.Header().Set("Content-Type", "application/json")
		if n == 1 {
			_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"completion_tokens":2177}}`))
			return
		}
		secondEffort = body.ReasoningEffort
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":"STEP a.go :: do it"},"finish_reason":"stop"}],"usage":{"completion_tokens":12}}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderMeta, srv.URL)
	got, err := c.CompleteWithSystem(context.Background(), "sys", "user")
	if err != nil {
		t.Fatalf("CompleteWithSystem returned an error instead of recovering: %v", err)
	}
	if got != "STEP a.go :: do it" {
		t.Errorf("got %q; want the recovered answer", got)
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("expected exactly 2 calls (original + one retry), got %d", n)
	}
	if secondEffort != "minimal" {
		t.Errorf("the retry asked for reasoning_effort %q; want minimal, otherwise it replays the request that produced nothing", secondEffort)
	}
}

// The retry is bounded: a Meta reply that is empty twice yields the original
// diagnostic after exactly two calls.
func TestCompleteWithSystem_Meta_EmptyRetryIsBounded(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"completion_tokens":2668}}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderMeta, srv.URL)
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "user"); err == nil {
		t.Fatal("expected an error when both attempts come back empty")
	}
	if n := atomic.LoadInt32(&calls); n != 2 {
		t.Errorf("expected exactly 2 calls, got %d", n)
	}
}
