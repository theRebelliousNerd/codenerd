package perception

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

// drainZAIStream collects a CompleteWithStreaming result the way consumers do:
// all chunks, then the terminal error (nil means clean completion).
func drainZAIStream(t *testing.T, content <-chan string, errCh <-chan error) (string, error) {
	t.Helper()
	var sb strings.Builder
	for chunk := range content {
		sb.WriteString(chunk)
	}
	if err, ok := <-errCh; ok {
		return sb.String(), err
	}
	return sb.String(), nil
}

func zaiSSEChunk(content string) string {
	return fmt.Sprintf("data: {\"choices\":[{\"delta\":{\"content\":%q}}]}\n\n", content)
}

// Happy path: SSE deltas concatenate in order, a malformed chunk is skipped,
// [DONE] ends the stream, and both channels close with no error.
func TestZAIStreaming_HappyPathConcatenates(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(zaiSSEChunk("Hello, ") + "data: {broken json\n\n" +
			zaiSSEChunk("world!") + "data: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := NewZAIClient("test-key")
	c.baseURL = srv.URL
	cc, ec := c.CompleteWithStreaming(context.Background(), "", "hi", false)
	text, err := drainZAIStream(t, cc, ec)
	if err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if text != "Hello, world!" {
		t.Errorf("text = %q, want Hello, world!", text)
	}
}

// A 429 with Retry-After is honored and the stream resumes on the next
// attempt instead of failing the turn.
func TestZAIStreaming_Retries429ThenStreams(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&seen, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusTooManyRequests)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(zaiSSEChunk("recovered") + "data: [DONE]\n\n"))
	}))
	defer srv.Close()

	c := NewZAIClient("test-key")
	c.baseURL = srv.URL
	c.retryBackoffBase = time.Millisecond
	c.retryBackoffMax = 10 * time.Millisecond
	cc, ec := c.CompleteWithStreaming(context.Background(), "", "hi", false)
	text, err := drainZAIStream(t, cc, ec)
	if err != nil {
		t.Fatalf("stream error: %v", err)
	}
	if text != "recovered" {
		t.Errorf("text = %q, want recovered", text)
	}
	if got := atomic.LoadInt32(&seen); got != 2 {
		t.Errorf("server saw %d requests, want 2", got)
	}
}

// When the backoff would not fit before the context deadline, the client used
// to send ctx.Err() — which is nil, because the deadline has NOT fired yet —
// and the consumer read a nil error as clean success with zero chunks. The
// give-up must be a real, deadline-identifying error.
func TestZAIStreaming_BackoffPastDeadlineIsError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	c := NewZAIClient("test-key")
	c.baseURL = srv.URL
	c.retryBackoffBase = time.Hour
	c.retryBackoffMax = time.Hour
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()
	cc, ec := c.CompleteWithStreaming(ctx, "", "hi", false)
	text, err := drainZAIStream(t, cc, ec)
	if err == nil {
		t.Fatalf("expected a deadline error, got nil with text %q", text)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("error = %v, want DeadlineExceeded identity", err)
	}
	if !strings.Contains(err.Error(), "would exceed") {
		t.Errorf("error = %q, want the give-up named", err)
	}
}

// Cancelling mid-stream surfaces the cancellation instead of hanging on a
// consumer that stopped reading.
func TestZAIStreaming_CancelMidStream(t *testing.T) {
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		_, _ = w.Write([]byte(zaiSSEChunk("partial")))
		if f, ok := w.(http.Flusher); ok {
			f.Flush()
		}
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer srv.Close()
	defer close(release)

	c := NewZAIClient("test-key")
	c.baseURL = srv.URL
	ctx, cancel := context.WithCancel(context.Background())
	content, errCh := c.CompleteWithStreaming(ctx, "", "hi", false)
	select {
	case chunk := <-content:
		if chunk != "partial" {
			t.Fatalf("first chunk = %q, want partial", chunk)
		}
	case err := <-errCh:
		t.Fatalf("stream failed before first chunk: %v", err)
	case <-time.After(5 * time.Second):
		t.Fatal("timed out waiting for the first chunk")
	}
	cancel()
	text, err := drainZAIStream(t, content, errCh)
	if !errors.Is(err, context.Canceled) {
		t.Errorf("error = %v (text %q), want context.Canceled", err, text)
	}
}
