package embedding

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The pull client's Timeout is the installed pull bound, not the caller's
// deadline and not a leftover 30-minute literal. The caller context is longer
// than the bound, and the handler does not answer, so only Client.Timeout
// brings the request back.
func TestPullModel_ClientTimeoutFollowsInstalledBound(t *testing.T) {
	prev := PullTimeout()
	t.Cleanup(func() {
		if prev > 0 {
			SetPullTimeout(prev)
			return
		}
		pullTimeout.Store(0)
	})

	const bound = 200 * time.Millisecond
	SetPullTimeout(bound)

	// Longer than the pull bound and shorter than the caller deadline. A
	// client that ignores the bound gets a successful response here and the
	// test fails; the bound returns first, with Client.Timeout.
	const handlerWait = 750 * time.Millisecond
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/pull" {
			http.NotFound(w, r)
			return
		}
		timer := time.NewTimer(handlerWait)
		defer timer.Stop()
		select {
		case <-r.Context().Done():
		case <-timer.C:
			w.WriteHeader(http.StatusOK)
		}
	}))
	t.Cleanup(server.Close)

	engine, err := NewOllamaEngine(server.URL, "pull-timeout-probe", 3)
	if err != nil {
		t.Fatal(err)
	}
	caller, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	start := time.Now()
	err = engine.pullModel(caller, "pull-timeout-probe")
	elapsed := time.Since(start)
	if err == nil {
		t.Fatal("pull succeeded; the client timeout did not fire")
	}
	if !strings.Contains(err.Error(), "Client.Timeout") {
		t.Fatalf("pull error = %v, want the pull client's Timeout (%s)", err, bound)
	}
	if elapsed >= 2*time.Second {
		t.Fatalf("pull returned after %s; want the %s client bound, not the caller deadline", elapsed, bound)
	}
}
