package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// always503 returns a server that fails every request transiently, forcing
// the client into its retry/backoff loop deterministically.
func always503(t *testing.T) *httptest.Server {
	t.Helper()
	return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte(`{"error":{"message":"high demand"}}`))
	}))
}

func geminiTestClient(baseURL string) *GeminiClient {
	return NewGeminiClientWithConfig(GeminiConfig{
		APIKey:  "test-key",
		BaseURL: baseURL,
		Model:   "gemini-2.5-flash",
		Timeout: time.Minute,
	})
}

// A cancelled turn must exit during the retry backoff, not after sleeping
// through 1+2+4s of it. Same guarantee as ExecuteOpenAIRequest.
func TestGeminiCompleteWithSystem_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := geminiTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := c.CompleteWithSystem(ctx, "sys", "hi")
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "cancelled during retry backoff") {
		t.Fatalf("err = %v, want cancellation during backoff", err)
	}
	if !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("err = %v, want wrapped ctx.Err()", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("cancelled turn took %v, want fast exit (old code slept 7s)", elapsed)
	}
}

// The schema path carries its own retry loop; it must honor cancellation too.
func TestGeminiCompleteWithSchema_CancelledBackoffExitsFast(t *testing.T) {
	srv := always503(t)
	defer srv.Close()
	c := geminiTestClient(srv.URL)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	start := time.Now()
	_, err := c.CompleteWithSchema(ctx, "sys", "hi", `{"type":"object"}`)
	elapsed := time.Since(start)

	if err == nil || !strings.Contains(err.Error(), "cancelled during retry backoff") {
		t.Fatalf("err = %v, want cancellation during backoff", err)
	}
	if elapsed > 3*time.Second {
		t.Errorf("cancelled turn took %v, want fast exit (old code slept 7s)", elapsed)
	}
}

// SetModel must match constructor semantics: Gemini 3 models always think,
// so switching into one (e.g. via the broker) flips thinking on. Switching
// within 2.x must not spuriously enable it.
func TestGeminiSetModel_IntoGemini3EnablesThinking(t *testing.T) {
	c := NewGeminiClientWithConfig(GeminiConfig{APIKey: "k", Model: "gemini-2.5-flash", EnableThinking: false})
	if c.IsThinkingEnabled() {
		t.Fatal("premise broken: 2.5 with thinking off should start disabled")
	}
	c.SetModel("gemini-3.5-flash")
	if !c.IsThinkingEnabled() {
		t.Error("switching into gemini-3 must enable thinking (constructor semantics)")
	}

	c2 := NewGeminiClientWithConfig(GeminiConfig{APIKey: "k", Model: "gemini-2.5-flash", EnableThinking: false})
	c2.SetModel("gemini-2.5-pro")
	if c2.IsThinkingEnabled() {
		t.Error("switching within 2.x must not spuriously enable thinking")
	}
}

// Readers of grounding sources must get a copy: mutating the returned slice
// must not corrupt client state.
func TestGeminiGroundingSources_CopyOnRead(t *testing.T) {
	c := NewGeminiClient("k")
	c.lastGroundingSources = []string{"https://a.example"}
	got := c.GetLastGroundingSources()
	got[0] = "MUTATED"
	if again := c.GetLastGroundingSources(); again[0] != "https://a.example" {
		t.Errorf("reader mutated client state: %q", again[0])
	}
}

// The URL-context setter must copy: later caller-slice mutation must not
// leak into subsequently built request tools.
func TestGeminiSetURLContextURLs_CopyOnWrite(t *testing.T) {
	c := NewGeminiClient("k")
	c.SetEnableURLContext(true)
	urls := []string{"https://a.example"}
	c.SetURLContextURLs(urls)
	urls[0] = "MUTATED"

	built := c.buildBuiltInTools()
	if len(built) != 1 || built[0].URLContext == nil {
		t.Fatalf("built tools = %+v, want one URL-context tool", built)
	}
	if built[0].URLContext.URLs[0] != "https://a.example" {
		t.Errorf("caller-slice mutation leaked into request tools: %q", built[0].URLContext.URLs[0])
	}
}

// CountTokens reads the cached-content name while SetCachedContent may write
// it; run with -race to prove the lock discipline. Behaviorally, every call
// must still return the server's count under concurrent cache flips.
func TestGeminiCountTokens_ConcurrentCacheFlip(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"totalTokens": 42}`))
	}))
	defer srv.Close()
	c := geminiTestClient(srv.URL)

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if i%2 == 0 {
				c.SetCachedContent("cachedContents/abc")
			} else {
				c.SetCachedContent("")
			}
		}(i)
		wg.Add(1)
		go func() {
			defer wg.Done()
			n, err := c.CountTokens(context.Background(), "sys", "hi")
			if err != nil || n != 42 {
				t.Errorf("CountTokens = %d, %v; want 42, nil", n, err)
			}
		}()
	}
	wg.Wait()
}

// The shared transient policy covers 529 on the Gemini paths too (the old
// switch only knew 500/502/503/504): a 529 is retried, and the
// sentinel-wrapped error still lets the firewall tell transient apart.
func TestGeminiChat_Retries529ThenSucceeds(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			w.WriteHeader(529)
			_, _ = w.Write([]byte(`{"error":{"code":529,"message":"overloaded"}}`))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"recovered"}]}}]}`))
	}))
	defer srv.Close()

	c := geminiTestClient(srv.URL)
	resp, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "recovered" {
		t.Errorf("response = %q, want recovered", resp)
	}
	if got := hits.Load(); got != 2 {
		t.Errorf("server hits = %d, want 2 (one 529 retried)", got)
	}
}
