package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestCompleteWithSystem_AllowEmptyCompletion(t *testing.T) {
	var calls int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&calls, 1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}],"usage":{"completion_tokens":1653}}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderMeta, srv.URL)

	got, err := c.CompleteWithSystem(WithAllowEmptyCompletion(context.Background()), "sys", "user")
	if err != nil {
		t.Fatalf("allowed empty completion returned error: %v", err)
	}
	if got != "" {
		t.Fatalf("got %q, want empty string", got)
	}
	if n := atomic.LoadInt32(&calls); n != 1 {
		t.Fatalf("expected 1 call, got %d", n)
	}
}

func TestCompleteWithSystem_EmptyStillErrorsWithoutOptIn(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[{"message":{"role":"assistant","content":""},"finish_reason":"stop"}]}`))
	}))
	defer srv.Close()

	c := newCompatClientAt(t, ProviderMeta, srv.URL)
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "user"); err == nil {
		t.Fatal("chat empty completion must still error without the opt-in")
	}
}
