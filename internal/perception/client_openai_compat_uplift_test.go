package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"
)

func compatTestClient(t *testing.T, baseURL string) *OpenAICompatClient {
	t.Helper()
	c, err := NewOpenAICompatClient(OpenAICompatConfig{
		Vendor:  ProviderMoonshot,
		APIKey:  "test-key",
		BaseURL: baseURL,
		Model:   "kimi-test",
		Timeout: time.Minute,
	})
	if err != nil {
		t.Fatalf("NewOpenAICompatClient: %v", err)
	}
	return c
}

const compatChatOK = `{
  "choices": [{"index": 0, "message": {"role": "assistant", "content": "  compat ok  "}, "finish_reason": "stop"}],
  "usage": {"prompt_tokens": 2, "completion_tokens": 3, "total_tokens": 5}
}`

// A 408 is transient by definition and retries on the shared policy. The
// old compat predicate only knew the 5xx set and failed a 408 outright.
func TestCompatChat_Retries408ThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusRequestTimeout)
			_, _ = w.Write([]byte(`{"error":{"message":"timeout"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(compatChatOK))
	}))
	defer srv.Close()

	c := compatTestClient(t, srv.URL)
	resp, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "compat ok" {
		t.Errorf("response = %q, want trimmed content", resp)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one 408 retried)", got)
	}
}

// A spurious 404 model_not_found is retried end to end: observed live on
// Meta mid-session, where it killed a shard delegation outright.
func TestCompatChat_RetriesSpuriousModelNotFound(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(http.StatusNotFound)
			_, _ = w.Write([]byte(`{"error":{"code":"model_not_found","message":"The requested model was not found."}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(compatChatOK))
	}))
	defer srv.Close()

	c := compatTestClient(t, srv.URL)
	resp, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "compat ok" {
		t.Errorf("response = %q, want trimmed content", resp)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one spurious 404 retried)", got)
	}
}

// A transient failure during stream setup is retried like the chat path,
// and the recovered stream still delivers every delta in order.
func TestCompatStreaming_RetriesTransientSetup(t *testing.T) {
	var hits atomic.Int32
	srv := sseFlakyServer(t, 1, http.StatusServiceUnavailable, &hits)
	defer srv.Close()

	c := compatTestClient(t, srv.URL)
	contentCh, errCh := c.CompleteWithStreaming(context.Background(), "sys", "hi", false)
	if got := drainStream(t, contentCh, errCh); got != "hello" {
		t.Errorf("streamed content = %q, want hello", got)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server hits = %d, want 2 (one setup retry)", n)
	}
}
