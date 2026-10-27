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

// A grounded search is sent the slot's configured effort like every other
// request, and a client configured with none sends none: no effort is
// invented for it (it used to default to xhigh).
func TestGroundedWebSearch_SendsTheConfiguredEffortOnly(t *testing.T) {
	tests := []struct {
		name   string
		effort string
		want   string // "" means the reasoning block is omitted
	}{
		{name: "configured effort", effort: "low", want: "low"},
		{name: "no effort configured", effort: "", want: ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var got *metaReasoningConfig
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				var req metaResponsesRequest
				_ = json.NewDecoder(r.Body).Decode(&req)
				got = req.Reasoning
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(groundedOKBody))
			}))
			defer srv.Close()

			c := newTestCompatClient(t, ProviderMeta, srv.URL)
			c.reasoningEffort = tt.effort
			if _, err := c.GroundedWebSearch(context.Background(), "q"); err != nil {
				t.Fatalf("GroundedWebSearch: %v", err)
			}
			switch {
			case tt.want == "" && got != nil:
				t.Errorf("reasoning = %+v, want it omitted", got)
			case tt.want != "" && (got == nil || got.Effort != tt.want):
				t.Errorf("reasoning = %+v, want effort %q", got, tt.want)
			}
		})
	}
}
