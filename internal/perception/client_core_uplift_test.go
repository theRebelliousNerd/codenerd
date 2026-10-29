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

// TestMapOpenAIToolCallsToInternal_BlankArguments pins the zero-arg variance:
// providers that serialize empty args as "" decode to an empty object
// instead of failing the whole turn.
func TestMapOpenAIToolCallsToInternal_BlankArguments(t *testing.T) {
	for _, args := range []string{"", "   "} {
		got, err := MapOpenAIToolCallsToInternal([]OpenAIToolCall{
			{ID: "c", Type: "function", Function: OpenAIFunctionCall{Name: "noargs", Arguments: args}},
		})
		if err != nil {
			t.Errorf("args %q: unexpected error %v", args, err)
			continue
		}
		if len(got) != 1 || got[0].Input == nil || len(got[0].Input) != 0 {
			t.Errorf("args %q: got %+v, want one call with empty object", args, got)
		}
	}
}

const toolHelperOKBody = `{"id":"x","object":"chat.completion","model":"m",
"choices":[{"index":0,"message":{"role":"assistant","content":"hi"},"finish_reason":"stop"}],
"usage":{"prompt_tokens":1,"completion_tokens":1,"total_tokens":2}}`

func toolHelperServer(t *testing.T, script []int) (*httptest.Server, *int32) {
	t.Helper()
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&hits, 1)
		status := script[0]
		if int(n) <= len(script) {
			status = script[n-1]
		}
		if status != http.StatusOK {
			w.WriteHeader(status)
			_, _ = w.Write([]byte(`{"error":"boom"}`))
			return
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(toolHelperOKBody))
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

// TestExecuteOpenAIRequest_RetriesTransient pins that 429/5xx/408 ride the
// backoff and a later 200 wins — while 400 fails on the first attempt.
func TestExecuteOpenAIRequest_RetriesTransient(t *testing.T) {
	srv, hits := toolHelperServer(t, []int{500, 429, 200})
	resp, err := ExecuteOpenAIRequest(context.Background(), srv.Client(), srv.URL, "k", OpenAIRequest{Model: "m"})
	if err != nil {
		t.Fatalf("expected recovery, got %v", err)
	}
	if resp.Choices[0].Message.Content != "hi" {
		t.Errorf("content=%q, want hi", resp.Choices[0].Message.Content)
	}
	if atomic.LoadInt32(hits) != 3 {
		t.Errorf("hits=%d, want 3 (two retries then success)", atomic.LoadInt32(hits))
	}

	srv400, hits400 := toolHelperServer(t, []int{400})
	_, err = ExecuteOpenAIRequest(context.Background(), srv400.Client(), srv400.URL, "k", OpenAIRequest{Model: "m"})
	if err == nil || !strings.Contains(err.Error(), "400") {
		t.Errorf("400: err=%v, want immediate status failure", err)
	}
	if atomic.LoadInt32(hits400) != 1 {
		t.Errorf("400: hits=%d, want 1 (no retry on client errors)", atomic.LoadInt32(hits400))
	}
}

// TestExecuteOpenAIRequest_CancelledBackoff pins that a cancelled context
// aborts the retry sleep instead of riding up to 7s of backoff: the error
// names cancellation and the call returns far below the full schedule.
func TestExecuteOpenAIRequest_CancelledBackoff(t *testing.T) {
	srv, _ := toolHelperServer(t, []int{500})
	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()
	start := time.Now()
	_, err := ExecuteOpenAIRequest(ctx, srv.Client(), srv.URL, "k", OpenAIRequest{Model: "m"})
	elapsed := time.Since(start)
	if err == nil || !strings.Contains(err.Error(), "cancelled") {
		t.Errorf("err=%v, want cancellation during backoff", err)
	}
	if elapsed > 5*time.Second {
		t.Errorf("elapsed=%v, want well under the 7s full backoff", elapsed)
	}
}

// TestBuildGeminiPiggybackEnvelopeSchema_IndependentCopies pins that the
// builder hands out independent maps: mutating one response schema must not
// corrupt the cached canonical schema other requests draw from.
func TestBuildGeminiPiggybackEnvelopeSchema_IndependentCopies(t *testing.T) {
	a := BuildGeminiPiggybackEnvelopeSchema()
	props, ok := a["properties"].(map[string]any)
	if !ok {
		t.Skip("canonical schema has no properties object to mutate")
	}
	props["injected_by_test"] = true
	if cp, ok := props["control_packet"].(map[string]any); ok {
		cp["injected_nested"] = true
	}
	b := BuildGeminiPiggybackEnvelopeSchema()
	bprops, _ := b["properties"].(map[string]any)
	if _, found := bprops["injected_by_test"]; found {
		t.Error("top-level mutation leaked into the next built schema")
	}
	if cp, ok := bprops["control_packet"].(map[string]any); ok {
		if _, found := cp["injected_nested"]; found {
			t.Error("nested mutation leaked into the next built schema")
		}
	}
}
