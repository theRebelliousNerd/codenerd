package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/types"
)

const metaResponsesOKReply = `{"id":"r1","status":"completed","model":"muse-spark-1.3-contributor",` +
	`"output":[{"id":"m1","type":"message","role":"assistant","status":"completed",` +
	`"content":[{"type":"output_text","text":"ok"}]}]}`

// newMetaResponsesServer answers the first len(statuses) requests with those
// HTTP statuses (Retry-After: 0 so the client's backoff is instant) and every
// later request with a minimal valid Responses reply. It returns the server
// and a counter of requests seen.
func newMetaResponsesServer(t *testing.T, statuses ...int) (*httptest.Server, *int32) {
	t.Helper()
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := int(atomic.AddInt32(&seen, 1))
		if n <= len(statuses) {
			w.Header().Set("Retry-After", "0")
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(statuses[n-1])
			_, _ = w.Write([]byte(`{"error":{"code":"service_overloaded","message":"The backend is temporarily overloaded. Please retry.","type":"server_error"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(metaResponsesOKReply))
	}))
	t.Cleanup(srv.Close)
	return srv, &seen
}

func metaResponsesHistory() []types.Message {
	return []types.Message{{Role: "user", Text: "hi"}}
}

// A single transient 503 on the Responses surface must not kill the turn:
// Meta's own error text says "Please retry", and the chat-completions path
// already retries the same class of failure.
func TestMetaResponses_Retries503ThenSucceeds(t *testing.T) {
	srv, seen := newMetaResponsesServer(t, http.StatusServiceUnavailable)
	c := newTestCompatClient(t, ProviderMeta, srv.URL)

	resp, err := c.CompleteWithToolResults(context.Background(), "sys", metaResponsesHistory(), nil)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if resp == nil || resp.Text != "ok" {
		t.Fatalf("response = %+v, want text ok", resp)
	}
	if got := atomic.LoadInt32(seen); got != 2 {
		t.Fatalf("server saw %d requests, want 2 (one 503, one success)", got)
	}
}

func TestMetaResponses_Retries429ThenSucceeds(t *testing.T) {
	srv, seen := newMetaResponsesServer(t, http.StatusTooManyRequests)
	c := newTestCompatClient(t, ProviderMeta, srv.URL)

	if _, err := c.CompleteWithToolResults(context.Background(), "sys", metaResponsesHistory(), nil); err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if got := atomic.LoadInt32(seen); got != 2 {
		t.Fatalf("server saw %d requests, want 2", got)
	}
}

// Persistent failure is still an error, bounded at maxRetries+1 attempts, and
// the body stays in the message so the vendor's reason is readable.
func TestMetaResponses_GivesUpAfterBoundedAttempts(t *testing.T) {
	srv, seen := newMetaResponsesServer(t,
		http.StatusServiceUnavailable, http.StatusServiceUnavailable,
		http.StatusServiceUnavailable, http.StatusServiceUnavailable,
		http.StatusServiceUnavailable, http.StatusServiceUnavailable)
	c := newTestCompatClient(t, ProviderMeta, srv.URL)

	_, err := c.CompleteWithToolResults(context.Background(), "sys", metaResponsesHistory(), nil)
	if err == nil {
		t.Fatal("expected an error after persistent 503s")
	}
	if !strings.Contains(err.Error(), "responses HTTP 503") || !strings.Contains(err.Error(), "service_overloaded") {
		t.Fatalf("error = %q, want the status and the vendor body", err)
	}
	if !strings.Contains(err.Error(), "after 4 attempts") {
		t.Fatalf("error = %q, want the attempt count", err)
	}
	if got := atomic.LoadInt32(seen); got != 4 {
		t.Fatalf("server saw %d requests, want 4 (maxRetries+1)", got)
	}
}

// A 400 is the request being wrong, not the vendor failing: no retry, and the
// body that names the offending field must surface immediately.
func TestMetaResponses_DoesNotRetry400(t *testing.T) {
	srv, seen := newMetaResponsesServer(t, http.StatusBadRequest)
	c := newTestCompatClient(t, ProviderMeta, srv.URL)

	_, err := c.CompleteWithToolResults(context.Background(), "sys", metaResponsesHistory(), nil)
	if err == nil {
		t.Fatal("expected an error on 400")
	}
	if !strings.Contains(err.Error(), "responses HTTP 400") {
		t.Fatalf("error = %q, want responses HTTP 400", err)
	}
	if got := atomic.LoadInt32(seen); got != 1 {
		t.Fatalf("server saw %d requests, want exactly 1", got)
	}
}
