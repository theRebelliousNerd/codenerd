package perception

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// geminiSSEServer fails the first failCount requests with failStatus, then
// serves a two-chunk Gemini SSE stream ("he"+"llo") with a trailing usage
// chunk. Mirrors sseFlakyServer for the Gemini wire shape.
func geminiSSEServer(t *testing.T, failCount int32, failStatus int, hits *atomic.Int32) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) <= failCount {
			w.WriteHeader(failStatus)
			_, _ = w.Write([]byte(`{"error":{"message":"try again"}}`))
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		for _, text := range []string{"he", "llo"} {
			fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]}}]}\n\n", text)
		}
		fmt.Fprint(w, "data: {\"candidates\":[],\"usageMetadata\":{\"promptTokenCount\":5,\"candidatesTokenCount\":2,\"totalTokenCount\":7}}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
}

// A transient failure during stream setup is retried, and the recovered
// stream still delivers every delta in order.
func TestGeminiStreaming_RetriesTransientSetup(t *testing.T) {
	var hits atomic.Int32
	srv := geminiSSEServer(t, 1, http.StatusServiceUnavailable, &hits)
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	contentCh, errCh := c.CompleteWithStreaming(context.Background(), "sys", "hi", false)
	if got := drainStream(t, contentCh, errCh); got != "hello" {
		t.Errorf("streamed content = %q, want hello", got)
	}
	if n := hits.Load(); n != 2 {
		t.Errorf("server hits = %d, want 2 (one setup retry)", n)
	}
}

// A cancelled turn must exit during the setup backoff and report on the
// error channel, not after sleeping through 1+2+4s of it.
func TestGeminiStreaming_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := geminiTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, errCh := c.CompleteWithStreaming(ctx, "sys", "hi", false)
	select {
	case err := <-errCh:
		elapsed := time.Since(start)
		if err == nil || !strings.Contains(err.Error(), "cancelled during retry backoff") {
			t.Fatalf("err = %v, want cancellation during backoff", err)
		}
		if elapsed > 3*time.Second {
			t.Errorf("cancelled stream took %v, want fast exit (old code slept 7s)", elapsed)
		}
	case <-time.After(10 * time.Second):
		t.Fatal("timed out waiting for stream error")
	}
}

// The 3-channel variant routes thinking parts to thoughtsChan and never
// lets them pollute the content stream (which would corrupt Piggyback
// JSON parsing downstream).
func TestGeminiStreaming_ThoughtsRoutedSeparately(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":\"hmm\",\"thought\":true},{\"text\":\"hi\"}]}}]}\n\n")
		fmt.Fprint(w, "data: [DONE]\n\n")
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	contentCh, thoughtsCh, errCh := c.CompleteWithStreamingAndThoughts(context.Background(), "sys", "hi", false)
	var content, thoughts strings.Builder
	for contentCh != nil || thoughtsCh != nil || errCh != nil {
		select {
		case delta, ok := <-contentCh:
			if !ok {
				contentCh = nil
				continue
			}
			content.WriteString(delta)
		case thought, ok := <-thoughtsCh:
			if !ok {
				thoughtsCh = nil
				continue
			}
			thoughts.WriteString(thought)
		case err, ok := <-errCh:
			if !ok {
				errCh = nil
				continue
			}
			if err != nil {
				t.Fatalf("stream error: %v", err)
			}
		case <-time.After(10 * time.Second):
			t.Fatal("timed out draining stream")
		}
	}
	if content.String() != "hi" {
		t.Errorf("content = %q, want hi (thoughts must not leak in)", content.String())
	}
	if thoughts.String() != "hmm" {
		t.Errorf("thoughts = %q, want hmm", thoughts.String())
	}
}

// The extracted rateLimit (now shared by chat, schema, and streaming)
// must enforce the 100ms inter-request spacing.
func TestGeminiRateLimit_EnforcesSpacing(t *testing.T) {
	c := geminiTestClient("http://127.0.0.1:1")
	c.rateLimit()
	start := time.Now()
	c.rateLimit()
	if gap := time.Since(start); gap < 90*time.Millisecond {
		t.Errorf("rateLimit gap = %v, want >= 90ms", gap)
	}
}
