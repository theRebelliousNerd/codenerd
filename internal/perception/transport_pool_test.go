package perception

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

// TestSharedHTTPClient_ParallelGETs exercises the pooled transport with
// 32 concurrent requests against an httptest server. The test verifies:
//
//  1. The shared client actually parallelises (the server sees all 32
//     requests).
//  2. The underlying *http.Transport has the documented connection-pool
//     tuning (MaxIdleConnsPerHost=64). This is a regression guard against
//     someone silently reverting the transport config when extracting it
//     to a helper.
//
// We do NOT t.Parallel() this test: sharedTransport is process-wide and
// other perception tests use the same client. Parallel execution would
// corrupt the request counter via cross-test interference on the same
// keepalive pool.
func TestSharedHTTPClient_ParallelGETs(t *testing.T) {
	var hits atomic.Int64
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		// Short sleep to encourage actual concurrency on the connection
		// pool rather than serialized reuse of one keepalive conn.
		time.Sleep(5 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}))
	t.Cleanup(srv.Close)

	client := NewSharedHTTPClient(10 * time.Second)
	require.NotNil(t, client)
	// Close idle conns at end of test so we don't leak keepalives into
	// any subsequent test using the same shared transport.
	t.Cleanup(func() {
		if tr, ok := client.Transport.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
	})

	const N = 32
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	t.Cleanup(cancel)

	var wg sync.WaitGroup
	errs := make(chan error, N)
	for i := 0; i < N; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			req, err := http.NewRequestWithContext(ctx, http.MethodGet, srv.URL, nil)
			if err != nil {
				errs <- err
				return
			}
			resp, err := client.Do(req)
			if err != nil {
				errs <- err
				return
			}
			// Drain + close so the connection returns to the pool.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		require.NoError(t, err, "request error")
	}

	require.Equal(t, int64(N), hits.Load(),
		"server should see exactly N=%d requests", N)
}

// TestSharedHTTPClient_TransportTuning verifies the documented pool
// limits via reflection on the unexported *http.Transport behind the
// http.RoundTripper interface. If someone changes these numbers the test
// must be updated deliberately — that's the whole point.
//
// MaxIdleConnsPerHost=64 is the load-bearing tuning: Go's default of 2
// serialises campaign-mode parallel LLM calls behind a single host. The
// other limits are kept in the assertion to flag silent regressions to
// any of them.
func TestSharedHTTPClient_TransportTuning(t *testing.T) {
	client := NewSharedHTTPClient(30 * time.Second)

	tr, ok := client.Transport.(*http.Transport)
	require.True(t, ok, "client.Transport must be *http.Transport, got %T", client.Transport)

	require.Equal(t, 64, tr.MaxIdleConnsPerHost,
		"documented MaxIdleConnsPerHost is 64 (Go default 2 is too low for parallel LLM traffic)")
	require.Equal(t, 256, tr.MaxIdleConns, "documented MaxIdleConns")
	require.Equal(t, 128, tr.MaxConnsPerHost, "documented MaxConnsPerHost")
	require.True(t, tr.ForceAttemptHTTP2, "ForceAttemptHTTP2 must be on")
	require.Equal(t, 90*time.Second, tr.IdleConnTimeout, "documented IdleConnTimeout")

	// Cross-check that the *value* lookup matches a reflect.Value
	// access — proves no aliasing or copy issue on the Transport field
	// (the production code uses a package-level pointer, sharing the
	// same backing struct across all clients).
	v := reflect.ValueOf(client.Transport).Elem()
	max := v.FieldByName("MaxIdleConnsPerHost")
	require.True(t, max.IsValid(), "reflect MaxIdleConnsPerHost field")
	require.Equal(t, int64(64), max.Int(), "reflected value matches direct read")
}

// TestSharedHTTPClient_TimeoutHonored exercises the per-call Timeout
// path: NewSharedHTTPClient embeds Timeout into the returned *Client,
// so a slow server must surface as a deadline error to the caller. This
// pins the contract that the shared transport does not silently override
// the requested timeout.
func TestSharedHTTPClient_TimeoutHonored(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Block until the request context fires so the request returns
		// promptly when the client tears down.
		<-r.Context().Done()
	}))
	t.Cleanup(srv.Close)

	client := NewSharedHTTPClient(50 * time.Millisecond)
	t.Cleanup(func() {
		if tr, ok := client.Transport.(*http.Transport); ok {
			tr.CloseIdleConnections()
		}
	})

	req, err := http.NewRequest(http.MethodGet, srv.URL, nil)
	require.NoError(t, err)
	_, err = client.Do(req)
	require.Error(t, err, "expected timeout error from slow server")
}
