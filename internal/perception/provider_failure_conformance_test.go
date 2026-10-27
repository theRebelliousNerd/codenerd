package perception

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/config"
)

const conformanceSecret = "sk-live-CONFORMANCE-SECRET-0123"

type conformanceAdapter struct {
	name  string
	build func(baseURL string) LLMClient
}

func conformanceAdapters() []conformanceAdapter {
	return []conformanceAdapter{
		{"anthropic", func(u string) LLMClient { return anthropicTestClient(u) }},
		{"openai", func(u string) LLMClient { return openaiTestClient(u) }},
		{"openrouter", func(u string) LLMClient { return openrouterTestClient(u) }},
		{"xai", func(u string) LLMClient { return xaiTestClient(u) }},
		{"gemini", func(u string) LLMClient { return geminiTestClient(u) }},
		{"ollama", func(u string) LLMClient { return ollamaTestClient(u) }},
		{"zai", func(u string) LLMClient {
			return NewZAIClientWithConfig(ZAIConfig{
				APIKey: "test-key", BaseURL: u, Model: "glm-test", Timeout: time.Minute,
				MaxRetries: 1, RetryBackoffBase: time.Millisecond, RetryBackoffMax: 5 * time.Millisecond,
			})
		}},
		{"openai-compat", func(u string) LLMClient {
			c, err := NewOpenAICompatClient(OpenAICompatConfig{Vendor: ProviderDashScope, APIKey: "test-key", BaseURL: u, Model: "qwen-test", Timeout: time.Minute})
			if err != nil {
				panic(err)
			}
			return c
		}},
	}
}

// Every HTTP adapter, driven through the same failing responses, must land in
// the same provider-independent class, retry exactly the retryable classes,
// and never let the provider's response body into the surfaced message. The
// perception-provider-failure-contract-v1 card: a user sees the same truthful
// degraded turn whichever model is configured.
func TestProviderFailureConformance(t *testing.T) {
	prev := config.GetLLMTimeouts()
	fast := prev
	fast.MaxRetries = 1
	fast.RetryBackoffBase = time.Millisecond
	fast.RetryBackoffMax = 5 * time.Millisecond
	config.SetLLMTimeouts(fast)
	t.Cleanup(func() { config.SetLLMTimeouts(prev) })

	statuses := []struct {
		status  int
		class   ProviderFailureClass
		retried bool
	}{
		{http.StatusUnauthorized, FailureAuth, false},
		{http.StatusForbidden, FailureAuth, false},
		{http.StatusBadRequest, FailureInvalidRequest, false},
		{http.StatusServiceUnavailable, FailureTransient, true},
		{http.StatusTooManyRequests, FailureRateLimited, true},
	}

	for _, adapter := range conformanceAdapters() {
		for _, tc := range statuses {
			t.Run(fmt.Sprintf("%s/%d", adapter.name, tc.status), func(t *testing.T) {
				var hits atomic.Int32
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits.Add(1)
					w.Header().Set("Content-Type", "application/json")
					w.WriteHeader(tc.status)
					_, _ = fmt.Fprintf(w, `{"error":{"message":"provider body echoing %s","type":"x"}}`, conformanceSecret)
				}))
				defer srv.Close()

				ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
				defer cancel()
				_, err := adapter.build(srv.URL).CompleteWithSystem(ctx, "sys", "hello")
				if err == nil {
					t.Fatalf("HTTP %d returned no error", tc.status)
				}
				if got := ClassifyProviderFailure(err); got != tc.class {
					t.Fatalf("HTTP %d classified %q, want %q (err: %v)", tc.status, got, tc.class, err)
				}
				if got := IsTransientProviderFailure(err); got != tc.class.Retryable() {
					t.Fatalf("IsTransientProviderFailure = %v for %q", got, tc.class)
				}
				if tc.retried && hits.Load() < 2 {
					t.Fatalf("HTTP %d (%s) was not retried: %d request(s)", tc.status, tc.class, hits.Load())
				}
				if !tc.retried && hits.Load() != 1 {
					t.Fatalf("HTTP %d (%s) must not be retried: %d request(s)", tc.status, tc.class, hits.Load())
				}
				msg := SafeProviderFailureMessage(err)
				if strings.Contains(msg, conformanceSecret) || strings.Contains(msg, "provider body") {
					t.Fatalf("surfaced message leaks the provider body: %q", msg)
				}
				if !strings.Contains(msg, strconv.Itoa(tc.status)) {
					t.Fatalf("surfaced message %q does not name the status %d", msg, tc.status)
				}
			})
		}

		t.Run(adapter.name+"/canceled", func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusServiceUnavailable)
			}))
			defer srv.Close()
			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			start := time.Now()
			_, err := adapter.build(srv.URL).CompleteWithSystem(ctx, "sys", "hello")
			if err == nil {
				t.Fatal("a canceled call returned no error")
			}
			if got := ClassifyProviderFailure(err); got != FailureCanceled {
				t.Fatalf("canceled call classified %q (err: %v)", got, err)
			}
			if elapsed := time.Since(start); elapsed > 5*time.Second {
				t.Fatalf("canceled call took %s", elapsed)
			}
		})
	}
}

// The subscription CLI adapters report rate-limit exhaustion as a typed
// RateLimitError rather than an HTTP-shaped message. Exercise both their
// normal and streaming production entry points through the portable fake
// executables; the shared classifier and safe user-facing message must match
// the HTTP adapter contract.
func TestCLIProviderFailureConformance(t *testing.T) {
	type cliAdapter struct {
		name    string
		binary  string
		modeEnv string
		build   func() LLMClient
	}
	adapters := []cliAdapter{
		{
			name:    "claude-cli",
			binary:  "claude",
			modeEnv: "CLAUDE_TEST_MODE",
			build:   func() LLMClient { return NewClaudeCodeCLIClient(nil) },
		},
		{
			name:    "codex-cli",
			binary:  "codex",
			modeEnv: "CODEX_TEST_MODE",
			build:   func() LLMClient { return NewCodexCLIClient(nil) },
		},
	}

	for _, adapter := range adapters {
		t.Run(adapter.name, func(t *testing.T) {
			dir := installFakeCLI(t, adapter.binary)
			t.Setenv("PATH", prependPath(dir, os.Getenv("PATH")))
			t.Setenv(adapter.modeEnv, "rate_limit")
			rawDetail := "rate limit reached; token=" + conformanceSecret
			t.Setenv("CLI_TEST_RATE_LIMIT_DETAIL", rawDetail)
			client := adapter.build()

			assertRateLimited := func(path string, err error) {
				t.Helper()
				if err == nil {
					t.Fatalf("%s returned no error", path)
				}
				if got := ClassifyProviderFailure(err); got != FailureRateLimited {
					t.Fatalf("%s classified %q, want %q (err: %v)", path, got, FailureRateLimited, err)
				}
				if !IsTransientProviderFailure(err) {
					t.Fatalf("%s is not retryable", path)
				}
				msg := SafeProviderFailureMessage(err)
				if !strings.Contains(msg, "HTTP 429") {
					t.Fatalf("%s safe message %q does not identify HTTP 429", path, msg)
				}
				if strings.Contains(msg, rawDetail) || strings.Contains(msg, conformanceSecret) {
					t.Fatalf("%s safe message leaked raw stderr or credentials: %q", path, msg)
				}
			}

			_, err := client.CompleteWithSystem(context.Background(), "sys", "hello")
			assertRateLimited("CompleteWithSystem", err)

			content, errorsCh := client.CompleteWithStreaming(context.Background(), "sys", "hello", false)
			for range content {
			}
			var streamErr error
			for err := range errorsCh {
				if err != nil {
					streamErr = err
				}
			}
			assertRateLimited("CompleteWithStreaming", streamErr)

			ctx, cancel := context.WithCancel(context.Background())
			cancel()
			_, err = client.CompleteWithSystem(ctx, "sys", "hello")
			if got := ClassifyProviderFailure(err); got != FailureCanceled {
				t.Fatalf("canceled CompleteWithSystem classified %q, want %q (err: %v)", got, FailureCanceled, err)
			}

			content, errorsCh = client.CompleteWithStreaming(ctx, "sys", "hello", false)
			for range content {
			}
			streamErr = nil
			for err := range errorsCh {
				if err != nil {
					streamErr = err
				}
			}
			if got := ClassifyProviderFailure(streamErr); got != FailureCanceled {
				t.Fatalf("canceled CompleteWithStreaming classified %q, want %q (err: %v)", got, FailureCanceled, streamErr)
			}
		})
	}
}

func TestClassifyProviderFailure_WrappedRateLimitAndCancellationPrecedence(t *testing.T) {
	rateLimit := fmt.Errorf("CLI retry exhausted: %w", &RateLimitError{
		Provider:    "codex-cli",
		RawResponse: "rate limit response echoed " + conformanceSecret,
	})
	if got := ClassifyProviderFailure(rateLimit); got != FailureRateLimited {
		t.Fatalf("wrapped rate limit classified %q, want %q", got, FailureRateLimited)
	}

	canceledAndRateLimited := fmt.Errorf("request ended during cancellation: %w; provider: %w", context.Canceled, rateLimit)
	if got := ClassifyProviderFailure(canceledAndRateLimited); got != FailureCanceled {
		t.Fatalf("canceled rate-limited request classified %q, want %q", got, FailureCanceled)
	}
}

// The degraded turn reads the class, not the adapter: a non-Gemini outage is
// reported as the model being unreachable, and an auth failure never shows the
// provider's body to the user.
func TestDegradedIntentUsesTheSharedFailureContract(t *testing.T) {
	outage := fmt.Errorf("LLM classification failed: %w", errors.New("transient server error (503): upstream connect error"))
	if intent := degradedClassificationIntent(outage); !intent.TransientFailure {
		t.Fatalf("a 503 from a non-Gemini adapter was not treated as an outage: %+v", intent)
	}

	auth := fmt.Errorf("LLM classification failed: %w", fmt.Errorf("API request failed with status 401: {\"error\":\"%s\"}", conformanceSecret))
	intent := degradedClassificationIntent(auth)
	if intent.TransientFailure {
		t.Fatal("an auth failure was reported as a transient outage")
	}
	if strings.Contains(intent.Response, conformanceSecret) {
		t.Fatalf("the degraded response leaks the provider body: %q", intent.Response)
	}
	if !strings.Contains(intent.Response, "credentials") {
		t.Fatalf("the degraded response does not say what failed: %q", intent.Response)
	}
}
