package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
)

const groundedOKBody = `{"output":[{"type":"message","role":"assistant",` +
	`"content":[{"type":"output_text","text":"grounded answer","annotations":[]}]}],` +
	`"usage":{"input_tokens":10,"output_tokens":5,"total_tokens":15}}`

// A grounded search used to be a single POST: one transient 503 killed the
// whole search while the sibling tool-loop path retried the same failure.
func TestGroundedWebSearch_Retries503ThenSucceeds(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&seen, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"overloaded, retry"}}`))
			return
		}
		_, _ = w.Write([]byte(groundedOKBody))
	}))
	defer srv.Close()

	res, err := newTestCompatClient(t, ProviderMeta, srv.URL).GroundedWebSearch(context.Background(), "q")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if res.Text != "grounded answer" {
		t.Errorf("Text = %q, want grounded answer", res.Text)
	}
	if got := atomic.LoadInt32(&seen); got != 2 {
		t.Errorf("server saw %d requests, want 2 (one 503, one success)", got)
	}
}

// Persistent transients still fail, bounded at four attempts, and the final
// error keeps the sanitized status shape the existing tests pin.
func TestGroundedWebSearch_GivesUpAfterBoundedAttempts(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"code":"service_overloaded","type":"server_error"}}`))
	}))
	defer srv.Close()

	_, err := newTestCompatClient(t, ProviderMeta, srv.URL).GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected an error after persistent 503s")
	}
	if !strings.Contains(err.Error(), "status 503") || !strings.Contains(err.Error(), "after 4 attempts") {
		t.Errorf("error = %q, want the status and the attempt count", err)
	}
	if got := atomic.LoadInt32(&seen); got != 4 {
		t.Errorf("server saw %d requests, want 4 (maxRetries+1)", got)
	}
}

// A 400 is the request being wrong: no retry, exactly one attempt.
func TestGroundedWebSearch_DoesNotRetry400(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{"error":{"message":"bad request"}}`))
	}))
	defer srv.Close()

	_, err := newTestCompatClient(t, ProviderMeta, srv.URL).GroundedWebSearch(context.Background(), "q")
	if err == nil {
		t.Fatal("expected an error on 400")
	}
	if got := atomic.LoadInt32(&seen); got != 1 {
		t.Errorf("server saw %d requests, want exactly 1", got)
	}
}

// A poisoned transient body — API key and reasoning in the 503 message —
// must not leak anywhere when the retry succeeds.
func TestGroundedWebSearch_RetryAbsorbsPoisonedTransientBody(t *testing.T) {
	const secret = "super secret reasoning trace"
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if atomic.AddInt32(&seen, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			_, _ = w.Write([]byte(`{"error":{"message":"` + secret + ` key=test-key"}}`))
			return
		}
		_, _ = w.Write([]byte(groundedOKBody))
	}))
	defer srv.Close()

	res, err := newTestCompatClient(t, ProviderMeta, srv.URL).GroundedWebSearch(context.Background(), "q")
	if err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if res.Text != "grounded answer" {
		t.Errorf("Text = %q, want grounded answer", res.Text)
	}
}

// "none" is rejected by Muse Spark, so the override must be dropped from the
// wire payload rather than forwarded — parity with newResponsesRequest.
func TestGroundedWebSearch_NoneEffortOmitsReasoning(t *testing.T) {
	var sawNil = false
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req metaGroundedRequest
		_ = json.NewDecoder(r.Body).Decode(&req)
		sawNil = req.Reasoning == nil
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(groundedOKBody))
	}))
	defer srv.Close()

	c := newTestCompatClient(t, ProviderMeta, srv.URL)
	c.reasoningEffortOverride = "none"
	if _, err := c.GroundedWebSearch(context.Background(), "q"); err != nil {
		t.Fatalf("GroundedWebSearch: %v", err)
	}
	if !sawNil {
		t.Error("reasoning block was sent with effort \"none\", want it omitted")
	}
}
