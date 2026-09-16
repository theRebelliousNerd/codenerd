package xaioauth

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// newUpliftOAuthClient builds a client against a test server with fresh temp
// credentials, following the chat_test.go pattern.
func newUpliftOAuthClient(t *testing.T, srvURL string, mutate func(*Credentials)) *Client {
	t.Helper()
	creds := &Credentials{
		AccessToken:  "test-token",
		RefreshToken: "test-refresh",
		ExpiresAt:    time.Now().Add(time.Hour),
		ClientID:     DefaultClientID,
	}
	if mutate != nil {
		mutate(creds)
	}
	credPath := filepath.Join(t.TempDir(), "creds.json")
	if err := SaveCredentials(credPath, creds); err != nil {
		t.Fatal(err)
	}
	return NewClient(Config{
		Model:          "grok-4.5",
		BaseURL:        srvURL + "/v1",
		Issuer:         srvURL,
		CredentialPath: credPath,
		ImportGrokAuth: false,
		Timeout:        time.Minute,
	})
}

const upliftChatOK = `{"choices":[{"message":{"role":"assistant","content":"uplift ok"}}]}`

// The chat surface never retried anything: one 503 killed the turn while the
// sibling API-key clients retried the same failure.
func TestOAuthChat_Retries503ThenSucceeds(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if atomic.AddInt32(&seen, 1) == 1 {
			w.Header().Set("Retry-After", "0")
			w.WriteHeader(http.StatusServiceUnavailable)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upliftChatOK))
	}))
	defer srv.Close()

	out, err := newUpliftOAuthClient(t, srv.URL, nil).CompleteWithSystem(context.Background(), "s", "u")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if out != "uplift ok" {
		t.Errorf("out = %q, want uplift ok", out)
	}
	if got := atomic.LoadInt32(&seen); got != 2 {
		t.Errorf("server saw %d requests, want 2", got)
	}
}

// Persistent rate limits surface as RateLimitedError with the vendor's
// Retry-After parsed — it used to be permanently zero because doJSON dropped
// the response headers and every call site passed nil.
func TestOAuthChat_RateLimitCarriesRetryAfter(t *testing.T) {
	var seen int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		atomic.AddInt32(&seen, 1)
		w.Header().Set("Retry-After", "0")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":"slow down"}`))
	}))
	defer srv.Close()

	_, err := newUpliftOAuthClient(t, srv.URL, nil).CompleteWithSystem(context.Background(), "s", "u")
	var rl *RateLimitedError
	if !errors.As(err, &rl) {
		t.Fatalf("err = %v, want RateLimitedError", err)
	}
	if got := atomic.LoadInt32(&seen); got != maxChatAttempts {
		t.Errorf("server saw %d requests, want %d", got, maxChatAttempts)
	}

	// And a nonzero header must arrive intact on the typed error: the
	// classifier parses it, and the transport above threads real headers in.
	h := http.Header{"Retry-After": []string{"7"}}
	if err := classifyHTTPError(http.StatusTooManyRequests, []byte(`{"error":"x"}`), h); true {
		var rl2 *RateLimitedError
		if !errors.As(err, &rl2) {
			t.Fatalf("classify = %v, want RateLimitedError", err)
		}
		if rl2.RetryAfter != 7*time.Second {
			t.Errorf("RetryAfter = %v, want 7s", rl2.RetryAfter)
		}
	}
}

// Only entitlement-worded 403s are tier gates. A quota-403 misclassified as
// tier-forbidden sends the probe down the reauth path instead of degrading.
func TestOAuthChat_Quota403IsNotTierForbidden(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
		_, _ = w.Write([]byte(`{"error":"quota exceeded for this minute, retry shortly"}`))
	}))
	defer srv.Close()

	_, err := newUpliftOAuthClient(t, srv.URL, nil).CompleteWithSystem(context.Background(), "s", "u")
	if err == nil {
		t.Fatal("expected an error on 403")
	}
	if IsTierForbidden(err) {
		t.Errorf("err = %v, a quota 403 must not classify as tier-forbidden", err)
	}
}

// 401 forces one refresh and retries with the new token: the Authorization
// header on the second attempt must carry the refreshed access token.
func TestOAuthChat_UnauthorizedRefreshesAndRetries(t *testing.T) {
	var authHeaders []string
	var mu sync.Mutex
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.URL.Path == "/.well-known/openid-configuration":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"issuer":                        "http://" + r.Host,
				"token_endpoint":                "http://" + r.Host + "/oauth2/token",
				"device_authorization_endpoint": "http://" + r.Host + "/oauth2/device/code",
			})
		case r.URL.Path == "/oauth2/token":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"access_token": "refreshed-token", "refresh_token": "test-refresh",
				"token_type": "Bearer", "expires_in": 3600,
			})
		default:
			mu.Lock()
			authHeaders = append(authHeaders, r.Header.Get("Authorization"))
			n := len(authHeaders)
			mu.Unlock()
			if n == 1 {
				w.WriteHeader(http.StatusUnauthorized)
				return
			}
			_, _ = w.Write([]byte(upliftChatOK))
		}
	}))
	defer srv.Close()

	out, err := newUpliftOAuthClient(t, srv.URL, nil).CompleteWithSystem(context.Background(), "s", "u")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if out != "uplift ok" {
		t.Errorf("out = %q, want uplift ok", out)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(authHeaders) != 2 {
		t.Fatalf("chat attempts = %d, want 2 (401 then retry)", len(authHeaders))
	}
	if authHeaders[0] != "Bearer test-token" || authHeaders[1] != "Bearer refreshed-token" {
		t.Errorf("auth headers = %v, want stale then refreshed token", authHeaders)
	}
}

// MaxConcurrentCalls was configured but never enforced: no semaphore existed.
// With a cap of 1, two concurrent turns must serialize; DisableSemaphore
// restores full concurrency for the external scheduler.
func TestOAuthChat_ConcurrencyCapEnforced(t *testing.T) {
	var current, peak int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		n := atomic.AddInt32(&current, 1)
		for {
			p := atomic.LoadInt32(&peak)
			if n <= p || atomic.CompareAndSwapInt32(&peak, p, n) {
				break
			}
		}
		time.Sleep(200 * time.Millisecond)
		atomic.AddInt32(&current, -1)
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(upliftChatOK))
	}))
	defer srv.Close()

	runPair := func(c *Client) {
		var wg sync.WaitGroup
		errs := make(chan error, 2)
		for i := 0; i < 2; i++ {
			wg.Add(1)
			go func() {
				defer wg.Done()
				_, err := c.CompleteWithSystem(context.Background(), "s", "u")
				errs <- err
			}()
		}
		wg.Wait()
		close(errs)
		for err := range errs {
			if err != nil {
				t.Fatalf("CompleteWithSystem: %v", err)
			}
		}
	}

	c := newUpliftOAuthClient(t, srv.URL, nil)
	c.cfg.MaxConcurrentCalls = 1
	c.sem = make(chan struct{}, 1)
	atomic.StoreInt32(&peak, 0)
	runPair(c)
	if got := atomic.LoadInt32(&peak); got != 1 {
		t.Errorf("peak in-flight = %d with cap 1, want exactly 1", got)
	}

	c.DisableSemaphore()
	atomic.StoreInt32(&peak, 0)
	runPair(c)
	if got := atomic.LoadInt32(&peak); got != 2 {
		t.Errorf("peak in-flight = %d after DisableSemaphore, want 2", got)
	}
}

// A device login poll runs for minutes; one network blip used to kill the
// whole flow. Only context death aborts now — anything else polls again.
func TestDevicePoll_SurvivesTransientBlip(t *testing.T) {
	var calls int32
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		switch atomic.AddInt32(&calls, 1) {
		case 1:
			return nil, fmt.Errorf("connection reset by peer")
		case 2:
			return jsonPollResponse(`{"error":"authorization_pending"}`), nil
		default:
			return jsonPollResponse(`{"access_token":"fresh","refresh_token":"r","expires_in":3600}`), nil
		}
	})
	hc := &http.Client{Transport: rt, Timeout: 10 * time.Second}
	creds, err := PollDeviceToken(context.Background(), hc, "http://issuer/oauth2/token", "cid", "dev", 10*time.Millisecond, time.Minute)
	if err != nil {
		t.Fatalf("PollDeviceToken: %v", err)
	}
	if creds.AccessToken != "fresh" {
		t.Errorf("access token = %q, want fresh", creds.AccessToken)
	}
	if got := atomic.LoadInt32(&calls); got != 3 {
		t.Errorf("poll attempts = %d, want 3 (blip, pending, success)", got)
	}
}

// But a cancelled context still aborts the poll immediately instead of
// polling into the void.
func TestDevicePoll_CancelAborts(t *testing.T) {
	rt := roundTripperFunc(func(r *http.Request) (*http.Response, error) {
		return nil, fmt.Errorf("connection reset by peer")
	})
	hc := &http.Client{Transport: rt, Timeout: 10 * time.Second}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := PollDeviceToken(ctx, hc, "http://issuer/oauth2/token", "cid", "dev", time.Millisecond, time.Minute); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v, want context.Canceled", err)
	}
}

func jsonPollResponse(body string) *http.Response {
	return &http.Response{
		StatusCode: http.StatusOK,
		Header:     make(http.Header),
		Body:       io.NopCloser(strings.NewReader(body)),
	}
}

type roundTripperFunc func(*http.Request) (*http.Response, error)

func (f roundTripperFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }
