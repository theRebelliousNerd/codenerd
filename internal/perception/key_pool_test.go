package perception

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/broker"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

var poolTestSequence atomic.Int64

func testPoolConfig(t *testing.T, base string) *ProviderConfig {
	t.Helper()
	id := poolTestSequence.Add(1)
	return &ProviderConfig{Provider: ProviderAnthropic, APIKey: "unused-scalar", Model: "claude-opus-5-5", BaseURL: base,
		APIKeys: []config.ResolvedProviderAPIKey{{Name: "alpha", Key: fmt.Sprintf("test-secret-%d-a", id), WorkspaceID: "wrkspc_a"}, {Name: "beta", Key: fmt.Sprintf("test-secret-%d-b", id), WorkspaceID: "wrkspc_b"}}}
}
func poolReply(w http.ResponseWriter, r *http.Request, stream bool) {
	if r.URL.Path == "/messages/count_tokens" {
		io.WriteString(w, `{"input_tokens":4}`)
		return
	}
	if stream {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, anthropicSSE("end_turn"))
		return
	}
	io.WriteString(w, anthropicJSONReply)
}

func TestKeyPool_AllLLMSurfacesPreserveCapabilitiesAndCredentials(t *testing.T) {
	var mu sync.Mutex
	var counted, inferred []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("x-api-key")
		expected := "wrkspc_b"
		if strings.HasSuffix(key, "-a") {
			expected = "wrkspc_a"
		}
		if r.Header.Get("anthropic-workspace-id") != expected {
			t.Error("workspace/key mismatch")
		}
		mu.Lock()
		if r.URL.Path == "/messages/count_tokens" {
			counted = append(counted, key)
		} else {
			inferred = append(inferred, key)
		}
		mu.Unlock()
		var body map[string]any
		json.NewDecoder(r.Body).Decode(&body)
		streaming, _ := body["stream"].(bool)
		poolReply(w, r, streaming)
	}))
	defer srv.Close()
	cfg := testPoolConfig(t, srv.URL)
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if !broker.IsBrokered(client) {
		t.Fatal("pool bypassed broker")
	}
	schema, ok := core.AsSchemaCapable(client)
	if !ok {
		t.Fatal("schema interface lost")
	}
	trp, ok := client.(types.ToolResultsProvider)
	if !ok {
		t.Fatal("native tool continuation lost")
	}
	if _, ok := client.(types.ThoughtStreamer); ok {
		t.Fatal("pool invented thought streaming")
	}
	ctx := context.Background()
	if _, err := client.Complete(ctx, "hi"); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CompleteWithSystem(ctx, "sys", "hi"); err != nil {
		t.Fatal(err)
	}
	if _, err := schema.CompleteWithSchema(ctx, "sys", "hi", anthropicClosedSchema); err != nil {
		t.Fatal(err)
	}
	if _, err := client.CompleteWithTools(ctx, "sys", "hi", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := trp.CompleteWithToolResults(ctx, "sys", []types.Message{types.NewUserMessage(types.TextBlock("hi"))}, nil); err != nil {
		t.Fatal(err)
	}
	ch, errs := client.CompleteWithStreaming(ctx, "sys", "hi", false)
	if _, err := drainAnthropic(t, ch, errs); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(counted) < 2 || len(counted) > 6 || len(inferred) != 6 {
		t.Fatalf("counts=%v inference=%v", counted, inferred)
	}
	for i := range inferred {
		if inferred[i] != cfg.APIKeys[i%2].Key {
			t.Fatalf("credential selection diverged: count=%v inference=%v", counted, inferred)
		}
	}
}

func TestKeyPool_AuthIsolationIsSharedAcrossModelsAndSlots(t *testing.T) {
	var badHits, goodHits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasSuffix(r.Header.Get("x-api-key"), "-a") {
			badHits.Add(1)
			w.WriteHeader(401)
			io.WriteString(w, `{"error":{"type":"authentication_error","message":"invalid"}}`)
			return
		}
		if r.URL.Path == "/messages" {
			goodHits.Add(1)
		}
		poolReply(w, r, false)
	}))
	defer srv.Close()
	cfg := testPoolConfig(t, srv.URL)
	main, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	classCfg := *cfg
	classCfg.ClassificationModel = "claude-haiku-5-5"
	classification, err := NewClassificationClientFromConfig(&classCfg)
	if err != nil {
		t.Fatal(err)
	}
	for _, client := range []LLMClient{main, classification, main} {
		if _, err := client.Complete(context.Background(), "hi"); err != nil {
			t.Fatal(err)
		}
	}
	if badHits.Load() != 1 || goodHits.Load() != 3 {
		t.Fatalf("disabled key reused: bad=%d good=%d", badHits.Load(), goodHits.Load())
	}
}

func TestKeyPool_RateLimitFailoverRecountsAndRespectsCooldown(t *testing.T) {
	var mu sync.Mutex
	var calls []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		key := r.Header.Get("x-api-key")
		suffix := key[len(key)-1:]
		mu.Lock()
		calls = append(calls, suffix+":"+r.URL.Path)
		mu.Unlock()
		if suffix == "a" && r.URL.Path == "/messages" {
			w.Header().Set("Retry-After", "3600")
			w.WriteHeader(429)
			io.WriteString(w, `{"error":{"type":"rate_limit_error","message":"wait"}}`)
			return
		}
		poolReply(w, r, false)
	}))
	defer srv.Close()
	cfg := testPoolConfig(t, srv.URL)
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now()
	for i := 0; i < 2; i++ {
		if _, err := client.Complete(context.Background(), "hi"); err != nil {
			t.Fatal(err)
		}
	}
	if time.Since(start) > 2*time.Second {
		t.Fatal("adapter retried/slept on a cooled credential instead of failing over")
	}
	mu.Lock()
	defer mu.Unlock()
	want := []string{"a:/messages/count_tokens", "a:/messages", "b:/messages/count_tokens", "b:/messages", "b:/messages"}
	if fmt.Sprint(calls) != fmt.Sprint(want) {
		t.Fatalf("calls=%v want=%v", calls, want)
	}
}

func TestKeyPool_AllCoolingWaitsCancelAndBadRequestsDoNotRotate(t *testing.T) {
	for _, status := range []int{400, 429} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			var hits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/messages/count_tokens" {
					poolReply(w, r, false)
					return
				}
				hits.Add(1)
				w.Header().Set("Retry-After", "3600")
				w.WriteHeader(status)
				io.WriteString(w, `{"error":{"type":"invalid_request_error","message":"bad request"}}`)
			}))
			defer srv.Close()
			cfg := testPoolConfig(t, srv.URL)
			client, err := NewClientFromConfig(cfg)
			if err != nil {
				t.Fatal(err)
			}
			_, err = client.Complete(context.Background(), "hi")
			if err == nil {
				t.Fatal("failure accepted")
			}
			want := int32(1)
			if status == 429 {
				want = 2
			}
			if hits.Load() != want {
				t.Fatalf("hits=%d want=%d", hits.Load(), want)
			}
			if status == 429 {
				ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
				defer cancel()
				_, err = client.Complete(ctx, "hi")
				if !errors.Is(err, context.DeadlineExceeded) || hits.Load() != 2 {
					t.Fatalf("cooldown ignored or cancellation lost: hits=%d err=%v", hits.Load(), err)
				}
			}
		})
	}
}

func TestKeyPool_StreamRetriesBeforeOutputAndNeverAfterOutput(t *testing.T) {
	for _, partial := range []bool{false, true} {
		t.Run(fmt.Sprint(partial), func(t *testing.T) {
			var bHits atomic.Int32
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.URL.Path == "/messages/count_tokens" {
					poolReply(w, r, false)
					return
				}
				if strings.HasSuffix(r.Header.Get("x-api-key"), "-a") {
					if !partial {
						w.WriteHeader(503)
						return
					}
					w.Header().Set("Content-Type", "text/event-stream")
					io.WriteString(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":4}}}\n\ndata: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":\"partial\"}}\n\ndata: {\"type\":\"error\",\"error\":{\"type\":\"overloaded_error\"}}\n\n")
					return
				}
				bHits.Add(1)
				poolReply(w, r, true)
			}))
			defer srv.Close()
			client, err := NewClientFromConfig(testPoolConfig(t, srv.URL))
			if err != nil {
				t.Fatal(err)
			}
			ch, errs := client.CompleteWithStreaming(context.Background(), "sys", "hi", false)
			text, err := drainAnthropic(t, ch, errs)
			if partial {
				if err == nil || text != "partial" || bHits.Load() != 0 {
					t.Fatalf("partial stream replayed: text=%q err=%v B=%d", text, err, bHits.Load())
				}
			} else if err != nil || text != "hello" || bHits.Load() != 1 {
				t.Fatalf("pre-output failover failed: text=%q err=%v B=%d", text, err, bHits.Load())
			}
		})
	}
}

func TestKeyPool_ConcurrentCallsUseLeastLoadedCredentials(t *testing.T) {
	entered := make(chan string, 2)
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/messages/count_tokens" {
			poolReply(w, r, false)
			return
		}
		entered <- r.Header.Get("x-api-key")
		select {
		case <-release:
		case <-r.Context().Done():
			return
		}
		poolReply(w, r, false)
	}))
	defer srv.Close()
	client, err := NewClientFromConfig(testPoolConfig(t, srv.URL))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { _, err := client.Complete(ctx, "hi"); results <- err }()
	}
	var keys []string
	for i := 0; i < 2; i++ {
		select {
		case key := <-entered:
			keys = append(keys, key)
		case <-ctx.Done():
			close(release)
			t.Fatal("requests did not run concurrently")
		}
	}
	close(release)
	if keys[0] == keys[1] {
		t.Fatal("both concurrent requests selected the same loaded key")
	}
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Fatal(err)
		}
	}
}

func TestKeyPool_UsageUsesAliasesAndNeverPersistsKeys(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { poolReply(w, r, false) }))
	defer srv.Close()
	cfg := testPoolConfig(t, srv.URL)
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	dir := t.TempDir()
	tracker, err := usage.NewTracker(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := usage.NewContext(context.Background(), tracker)
	for i := 0; i < 2; i++ {
		if _, err := client.Complete(ctx, "hi"); err != nil {
			t.Fatal(err)
		}
	}
	stats := tracker.Stats()
	for _, alias := range []string{"anthropic/alpha", "anthropic/beta"} {
		if stats.ByCredential[alias].Total != 9 {
			t.Fatalf("credential usage lost: %+v", stats.ByCredential)
		}
	}
	stats.ByCredential["anthropic/alpha"] = usage.TokenCounts{}
	if tracker.Stats().ByCredential["anthropic/alpha"].Total != 9 {
		t.Fatal("Stats leaked live map")
	}
	if err := tracker.Close(); err != nil {
		t.Fatal(err)
	}
	paths, err := filepath.Glob(filepath.Join(dir, "*"))
	if err != nil {
		t.Fatal(err)
	}
	for _, path := range paths {
		data, _ := os.ReadFile(path)
		for _, key := range cfg.APIKeys {
			if strings.Contains(string(data), key.Key) {
				t.Fatal("usage persisted a raw credential")
			}
		}
	}
}

func TestKeyPool_OtherProvidersAndModelSwitchKeepRouting(t *testing.T) {
	for _, provider := range []Provider{ProviderOpenAI, ProviderOpenRouter, ProviderXAI, ProviderZAI, ProviderGemini, ProviderDashScope, ProviderMeta, ProviderMoonshot} {
		t.Run(string(provider), func(t *testing.T) {
			var mu sync.Mutex
			seen := map[string]bool{}
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				key := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
				if key == "" {
					key = r.URL.Query().Get("key")
				}
				mu.Lock()
				seen[key] = true
				mu.Unlock()
				if provider == ProviderMeta {
					io.WriteString(w, `{"status":"completed","output":[{"type":"message","role":"assistant","content":[{"type":"output_text","text":"hello"}]}],"usage":{"input_tokens":4,"output_tokens":5}}`)
				} else if provider == ProviderGemini {
					io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"hello"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":5}}`)
				} else {
					io.WriteString(w, `{"choices":[{"message":{"role":"assistant","content":"hello"},"finish_reason":"stop"}],"usage":{"prompt_tokens":4,"completion_tokens":5}}`)
				}
			}))
			defer srv.Close()
			cfg := testPoolConfig(t, srv.URL)
			cfg.Provider = provider
			cfg.Model = "test-model"
			if provider == ProviderMeta {
				cfg.Model = "muse-spark-1.3-contributor"
			}
			for i := range cfg.APIKeys {
				cfg.APIKeys[i].WorkspaceID = ""
			}
			client, err := NewClientFromConfig(cfg)
			if err != nil {
				t.Fatal(err)
			}
			for i := 0; i < 2; i++ {
				if _, err := client.Complete(context.Background(), "hi"); err != nil {
					t.Fatal(err)
				}
			}
			nextModel := "another-model"
			if provider == ProviderMeta {
				nextModel = "muse-spark-1.2-contributor"
			}
			client.(interface{ SetModel(string) }).SetModel(nextModel)
			if client.(interface{ GetModel() string }).GetModel() != nextModel {
				t.Fatal("model switch lost")
			}
			if _, err := client.Complete(context.Background(), "hi"); err != nil {
				t.Fatal(err)
			}
			mu.Lock()
			defer mu.Unlock()
			if len(seen) != 2 || !seen[cfg.APIKeys[0].Key] || !seen[cfg.APIKeys[1].Key] {
				t.Fatalf("pool did not balance auth: %v", seen)
			}
		})
	}
}

func TestKeyPool_GeminiThoughtStreamingAndGroundingSettersSurvive(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		io.WriteString(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"thought\":true,\"text\":\"summary\"},{\"text\":\"answer\"}]},\"finishReason\":\"STOP\"}],\"usageMetadata\":{\"promptTokenCount\":4,\"candidatesTokenCount\":5}}\n\n")
	}))
	defer srv.Close()
	cfg := testPoolConfig(t, srv.URL)
	cfg.Provider = ProviderGemini
	cfg.Model = "gemini-3-pro-preview"
	for i := range cfg.APIKeys {
		cfg.APIKeys[i].WorkspaceID = ""
	}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	stream, ok := client.(types.ThoughtStreamer)
	if !ok {
		t.Fatal("Gemini thought streaming masked")
	}
	text, thought, errs := stream.CompleteWithStreamingAndThoughts(context.Background(), "sys", "hi", true)
	var content, summary string
	for text != nil || thought != nil || errs != nil {
		select {
		case part, ok := <-text:
			if !ok {
				text = nil
			} else {
				content += part
			}
		case part, ok := <-thought:
			if !ok {
				thought = nil
			} else {
				summary += part
			}
		case res, ok := <-errs:
			if !ok {
				errs = nil
			} else if res.Err != nil {
				t.Fatal(res.Err)
			}
		}
	}
	if content != "answer" || summary != "summary" {
		t.Fatalf("content=%q thoughts=%q", content, summary)
	}
	client.(interface{ SetEnableWebSearch(bool) }).SetEnableWebSearch(true)
	if !client.(interface{ IsWebSearchEnabled() bool }).IsWebSearchEnabled() {
		t.Fatal("grounding setter lost")
	}
}

func TestKeyPool_PanicsReleaseCredentialLeases(t *testing.T) {
	pool := &providerKeyPool{health: make([]keyHealth, 1)}
	router := &providerKeyRouter{provider: ProviderAnthropic, timeout: time.Second, aliases: []string{"test"}, clients: []LLMClient{nil}, pool: pool, pinned: -1}
	err := router.Do(context.Background(), func(context.Context, types.LLMClient) error { panic("secret-panic-payload") })
	if err == nil || strings.Contains(err.Error(), "secret-panic-payload") || pool.health[0].active != 0 {
		t.Fatalf("panic leaked lease or payload: err=%v active=%d", err, pool.health[0].active)
	}
	content, thoughts, errs := router.Stream(context.Background(), func(context.Context, types.LLMClient) (<-chan string, <-chan string, <-chan types.StreamResult) {
		panic("secret-panic-payload")
	})
	for range content {
	}
	for range thoughts {
	}
	err = types.AwaitStreamResult(context.Background(), errs).Err
	if err == nil || strings.Contains(err.Error(), "secret-panic-payload") || pool.health[0].active != 0 {
		t.Fatalf("stream panic leaked lease or payload: err=%v active=%d", err, pool.health[0].active)
	}
}

func TestKeyPool_ServerCachedContentPinsItsOwningCredential(t *testing.T) {
	var mu sync.Mutex
	var seen []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		seen = append(seen, r.URL.Query().Get("key"))
		mu.Unlock()
		io.WriteString(w, `{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":4,"candidatesTokenCount":5}}`)
	}))
	defer srv.Close()
	cfg := testPoolConfig(t, srv.URL)
	cfg.Provider = ProviderGemini
	cfg.Model = "gemini-3-pro-preview"
	for i := range cfg.APIKeys {
		cfg.APIKeys[i].WorkspaceID = ""
	}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := client.Complete(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	setter := client.(interface{ SetCachedContent(string) })
	setter.SetCachedContent("cachedContents/owned-by-alpha")
	for i := 0; i < 2; i++ {
		if _, err := client.Complete(context.Background(), "hi"); err != nil {
			t.Fatal(err)
		}
	}
	setter.SetCachedContent("")
	if _, err := client.Complete(context.Background(), "hi"); err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	defer mu.Unlock()
	if len(seen) != 4 || seen[0] != cfg.APIKeys[0].Key || seen[1] != seen[0] || seen[2] != seen[0] || seen[3] != cfg.APIKeys[1].Key {
		t.Fatalf("cached account resource moved between keys: %v", seen)
	}
}
