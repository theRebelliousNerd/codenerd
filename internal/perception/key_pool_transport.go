package perception

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
)

type keyPoolAttemptContext struct{}

// A child context belongs to one credential attempt, including token counting.
// Closing a failed HTTP response cancels only that attempt so the adapter exits
// its own retry loop and the pool can try another key with fresh admission.
type keyPoolAttempt struct {
	mu             sync.Mutex
	cancel         context.CancelFunc
	status         int
	retryAfter     time.Duration
	networkFailure bool
}

func (a *keyPoolAttempt) failure(status int, after time.Duration, network bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.status == 0 && !a.networkFailure {
		a.status = status
		a.retryAfter = after
		a.networkFailure = network
	}
}
func (a *keyPoolAttempt) result() (int, time.Duration, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.status, a.retryAfter, a.networkFailure
}

type poolObservedTransport struct {
	base   http.RoundTripper
	secret string
}

func (t poolObservedTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.base.RoundTrip(req)
	attempt, _ := req.Context().Value(keyPoolAttemptContext{}).(*keyPoolAttempt)
	if attempt == nil {
		return resp, err
	}
	if err != nil {
		if req.Context().Err() == nil {
			attempt.failure(0, 0, true)
			return &http.Response{StatusCode: 503, Header: make(http.Header), Body: &poolFailureBody{ReadCloser: io.NopCloser(strings.NewReader(`{"error":{"message":"provider transport unavailable"}}`)), cancel: attempt.cancel}, Request: req}, nil
		}
		return resp, err
	}
	if resp.StatusCode >= 400 {
		payload, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		resp.Body.Close()
		if readErr != nil {
			payload = []byte(`{"error":{"message":"provider error response could not be read"}}`)
		}
		if t.secret != "" {
			payload = []byte(strings.ReplaceAll(string(payload), t.secret, "[redacted]"))
		}
		resp.Body = io.NopCloser(bytes.NewReader(payload))
	}
	if resp.StatusCode == 401 || resp.StatusCode == 403 || resp.StatusCode == 429 || isTransientHTTPStatus(resp.StatusCode) {
		attempt.failure(resp.StatusCode, anthropicRetryAfter(resp.Header.Get("Retry-After"), time.Now()), false)
		resp.Body = &poolFailureBody{ReadCloser: resp.Body, cancel: attempt.cancel}
	}
	return resp, nil
}

type poolFailureBody struct {
	io.ReadCloser
	cancel context.CancelFunc
}

func (b *poolFailureBody) Close() error { defer b.cancel(); return b.ReadCloser.Close() }

func observePoolHTTP(client LLMClient) bool {
	var httpClient *http.Client
	var secret string
	switch c := client.(type) {
	case *AnthropicClient:
		httpClient = c.httpClient
		secret = c.apiKey
	case *OpenAIClient:
		httpClient = c.httpClient
		secret = c.apiKey
	case *OpenRouterClient:
		httpClient = c.httpClient
		secret = c.apiKey
	case *XAIClient:
		httpClient = c.httpClient
		secret = c.apiKey
	case *GeminiClient:
		httpClient = c.httpClient
		secret = c.apiKey
	case *ZAIClient:
		httpClient = c.httpClient
		secret = c.apiKey
	case *OpenAICompatClient:
		httpClient = c.httpClient
		secret = c.apiKey
	default:
		return false
	}
	base := httpClient.Transport
	if base == nil {
		base = http.DefaultTransport
	}
	httpClient.Transport = poolObservedTransport{base: base, secret: secret}
	// Providers use custom authentication headers (or query keys). A redirect
	// must not forward them to a different destination.
	httpClient.CheckRedirect = func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }
	return true
}
