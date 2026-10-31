package perception

import (
	"context"
	"errors"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/usage"
)

type attemptDispatchTransport func(*http.Request) (*http.Response, error)

func (f attemptDispatchTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// The transport deliberately ignores cancellation. Refusal must happen before
// entering it, rather than relying on net/http noticing a canceled context.
func TestProviderAttemptStopsBeforeTransport(t *testing.T) {
	var calls atomic.Int32
	client := &http.Client{Transport: attemptDispatchTransport(func(r *http.Request) (*http.Response, error) {
		calls.Add(1)
		return &http.Response{StatusCode: 200, Header: make(http.Header), Body: io.NopCloser(strings.NewReader("answer")), Request: r}, nil
	})}
	ctx, scope := usage.WithAttemptScope(t.Context())
	defer scope.Close()
	scope.StopUnknown()
	req, err := http.NewRequestWithContext(context.WithoutCancel(ctx), "POST", "https://provider.invalid", strings.NewReader("synthetic"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := doProviderRequest(client, req); !errors.Is(err, usage.ErrUnknownSpend) || calls.Load() != 0 {
		t.Fatalf("closed transport called: %v/%d", err, calls.Load())
	}
	other, owner := usage.WithAttemptScope(t.Context())
	defer owner.Close()
	for _, ctx := range []context.Context{other, t.Context()} {
		response, err := doProviderRequest(client, req.Clone(ctx))
		if err != nil {
			t.Fatal(err)
		}
		_ = response.Body.Close()
	}
	if calls.Load() != 2 {
		t.Fatalf("unrelated transports blocked: %d", calls.Load())
	}
}
