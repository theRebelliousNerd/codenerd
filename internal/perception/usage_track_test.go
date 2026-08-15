package perception

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/usage"
)

// trackedContext returns a context carrying a fresh tracker rooted in a temp
// workspace, plus that tracker.
func trackedContext(t *testing.T) (context.Context, *usage.Tracker) {
	t.Helper()
	tracker, err := usage.NewTracker(t.TempDir())
	if err != nil {
		t.Fatalf("NewTracker: %v", err)
	}
	t.Cleanup(func() { _ = tracker.Close() })
	return usage.NewContext(context.Background(), tracker), tracker
}

func TestCanonicalProviderIDs_WhenComparedToConfig_ShouldBeAccepted(t *testing.T) {
	// The by-provider breakdown is only useful if its keys are the same strings
	// config uses to name an engine. SetAPIKeyForProvider rejects anything it
	// does not recognize, so it is the authority we check against.
	cfg := &config.UserConfig{}
	for provider := range canonicalProviderIDs {
		err := cfg.SetAPIKeyForProvider(string(provider), "k")
		switch {
		case err == nil:
			// accepted
		case provider == ProviderOllama && strings.Contains(err.Error(), "keyless"):
			// Ollama is a real engine name with no key to set.
		default:
			t.Errorf("provider id %q is not a config engine name: %v", provider, err)
		}
	}
}

func TestUsageProviderID_WhenProviderUnknown_ShouldNotSilentlyImpersonateAKnownOne(t *testing.T) {
	if got := usageProviderID(ProviderZAI); got != "zai" {
		t.Errorf("usageProviderID(zai)=%q, want zai", got)
	}
	if got := usageProviderID(Provider("z.ai")); got != "unregistered:z.ai" {
		t.Errorf("usageProviderID(z.ai)=%q, want unregistered:z.ai", got)
	}
	if got := usageProviderID(""); got != "unknown" {
		t.Errorf("usageProviderID(\"\")=%q, want unknown", got)
	}
}

func TestTrackUsage_WhenContextHasNoTracker_ShouldNotPanic(t *testing.T) {
	trackUsage(context.Background(), "m", ProviderZAI, 10, 20, usageOpChat)
}

// openAIChatBody is a minimal non-streaming chat completion payload with usage.
func openAIChatBody(prompt, completion int) map[string]any {
	return map[string]any{
		"choices": []map[string]any{
			{"index": 0, "message": map[string]any{"role": "assistant", "content": "hi"}},
		},
		"usage": map[string]any{
			"prompt_tokens":     prompt,
			"completion_tokens": completion,
			"total_tokens":      prompt + completion,
		},
	}
}

func TestClients_WhenCompletionSucceeds_ShouldTrackUsageUnderCanonicalProvider(t *testing.T) {
	cases := []struct {
		name         string
		wantProvider string
		handler      http.HandlerFunc
		call         func(ctx context.Context, baseURL string) error
	}{
		{
			name:         "openai",
			wantProvider: "openai",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(openAIChatBody(11, 7))
			},
			call: func(ctx context.Context, baseURL string) error {
				c := NewOpenAIClientWithConfig(OpenAIConfig{APIKey: "k", BaseURL: baseURL, Model: "gpt-x", Timeout: 5 * time.Second})
				_, err := c.CompleteWithSystem(ctx, "sys", "user")
				return err
			},
		},
		{
			name:         "ollama",
			wantProvider: "ollama",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(openAIChatBody(11, 7))
			},
			call: func(ctx context.Context, baseURL string) error {
				// Ollama borrows the OpenAI transport; the point of the case is
				// that its tokens are not billed to openai.
				c := NewOllamaClientWithConfig(OllamaLLMConfig{Endpoint: strings.TrimSuffix(baseURL, "/v1"), Model: "gemma", Timeout: 5 * time.Second})
				_, err := c.CompleteWithSystem(ctx, "sys", "user")
				return err
			},
		},
		{
			name:         "xai",
			wantProvider: "xai",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(openAIChatBody(11, 7))
			},
			call: func(ctx context.Context, baseURL string) error {
				c := NewXAIClientWithConfig(XAIConfig{APIKey: "k", BaseURL: baseURL, Model: "grok", Timeout: 5 * time.Second})
				_, err := c.CompleteWithSystem(ctx, "sys", "user")
				return err
			},
		},
		{
			name:         "openrouter",
			wantProvider: "openrouter",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(openAIChatBody(11, 7))
			},
			call: func(ctx context.Context, baseURL string) error {
				c := NewOpenRouterClientWithConfig(OpenRouterConfig{APIKey: "k", BaseURL: baseURL, Model: "some/model", Timeout: 5 * time.Second})
				_, err := c.CompleteWithSystem(ctx, "sys", "user")
				return err
			},
		},
		{
			name:         "zai",
			wantProvider: "zai",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"choices": []map[string]any{
						{"index": 0, "message": map[string]any{"role": "assistant", "content": "hi"}},
					},
					"usage": map[string]any{"prompt_tokens": 11, "completion_tokens": 7, "total_tokens": 18},
				})
			},
			call: func(ctx context.Context, baseURL string) error {
				c := NewZAIClientWithConfig(ZAIConfig{APIKey: "k", BaseURL: baseURL, Model: "glm", Timeout: 5 * time.Second, DisableSemaphore: true})
				_, err := c.CompleteWithSystem(ctx, "sys", "user")
				return err
			},
		},
		{
			name:         "dashscope",
			wantProvider: "dashscope",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(openAIChatBody(11, 7))
			},
			call: func(ctx context.Context, baseURL string) error {
				c, err := NewOpenAICompatClient(OpenAICompatConfig{Vendor: ProviderDashScope, APIKey: "k", BaseURL: baseURL, Model: "qwen", Timeout: 5 * time.Second})
				if err != nil {
					return err
				}
				_, err = c.CompleteWithSystem(ctx, "sys", "user")
				return err
			},
		},
		{
			name:         "anthropic",
			wantProvider: "anthropic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"content": []map[string]any{{"type": "text", "text": "hi"}},
					"usage":   map[string]any{"input_tokens": 11, "output_tokens": 7},
				})
			},
			call: func(ctx context.Context, baseURL string) error {
				c := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "k", BaseURL: baseURL, Model: "claude-x", Timeout: 5 * time.Second})
				_, err := c.CompleteWithSystem(ctx, "sys", "user")
				return err
			},
		},
		{
			name:         "gemini",
			wantProvider: "gemini",
			handler: func(w http.ResponseWriter, r *http.Request) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"candidates": []map[string]any{
						{"content": map[string]any{"parts": []map[string]any{{"text": "hi"}}}},
					},
					// Thinking tokens are billed as output, so the expected
					// output count below is candidates + thoughts.
					"usageMetadata": map[string]any{
						"promptTokenCount":     11,
						"candidatesTokenCount": 4,
						"thoughtsTokenCount":   3,
						"totalTokenCount":      18,
					},
				})
			},
			call: func(ctx context.Context, baseURL string) error {
				c := NewGeminiClientWithConfig(GeminiConfig{APIKey: "k", BaseURL: baseURL, Model: "gemini-x", Timeout: 5 * time.Second})
				_, err := c.CompleteWithSystem(ctx, "sys", "user")
				return err
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			ctx, tracker := trackedContext(t)
			if err := tc.call(ctx, srv.URL); err != nil {
				t.Fatalf("completion: %v", err)
			}

			stats := tracker.Stats()
			got, ok := stats.ByProvider[tc.wantProvider]
			if !ok {
				t.Fatalf("no usage recorded for provider %q; ByProvider=%v", tc.wantProvider, stats.ByProvider)
			}
			if got.Input != 11 || got.Output != 7 {
				t.Errorf("provider %s: input=%d output=%d, want 11/7", tc.wantProvider, got.Input, got.Output)
			}
			if stats.TotalProject.Total != 18 {
				t.Errorf("total=%d, want 18 (a turn counted twice or not at all)", stats.TotalProject.Total)
			}
		})
	}
}

// sseChatStream writes an OpenAI-style SSE stream whose final chunk carries the
// billed usage and no choices, which is what include_usage produces.
func sseChatStream(w http.ResponseWriter, deltas []string, prompt, completion int) {
	w.Header().Set("Content-Type", "text/event-stream")
	flusher, _ := w.(http.Flusher)
	for _, d := range deltas {
		fmt.Fprintf(w, "data: {\"choices\":[{\"index\":0,\"delta\":{\"content\":%q}}]}\n\n", d)
		if flusher != nil {
			flusher.Flush()
		}
	}
	fmt.Fprintf(w, "data: {\"choices\":[],\"usage\":{\"prompt_tokens\":%d,\"completion_tokens\":%d,\"total_tokens\":%d}}\n\n",
		prompt, completion, prompt+completion)
	fmt.Fprint(w, "data: [DONE]\n\n")
	if flusher != nil {
		flusher.Flush()
	}
}

func TestStreamingClients_WhenStreamCompletes_ShouldTrackOnceWithFinalBilledTokens(t *testing.T) {
	deltas := []string{"one ", "two ", "three"}

	cases := []struct {
		name         string
		wantProvider string
		handler      http.HandlerFunc
		stream       func(ctx context.Context, baseURL string) (<-chan string, <-chan error)
	}{
		{
			name:         "openai",
			wantProvider: "openai",
			handler:      func(w http.ResponseWriter, r *http.Request) { sseChatStream(w, deltas, 11, 7) },
			stream: func(ctx context.Context, baseURL string) (<-chan string, <-chan error) {
				c := NewOpenAIClientWithConfig(OpenAIConfig{APIKey: "k", BaseURL: baseURL, Model: "gpt-x", Timeout: 5 * time.Second})
				return c.CompleteWithStreaming(ctx, "sys", "user", false)
			},
		},
		{
			name:         "openrouter",
			wantProvider: "openrouter",
			handler:      func(w http.ResponseWriter, r *http.Request) { sseChatStream(w, deltas, 11, 7) },
			stream: func(ctx context.Context, baseURL string) (<-chan string, <-chan error) {
				c := NewOpenRouterClientWithConfig(OpenRouterConfig{APIKey: "k", BaseURL: baseURL, Model: "some/model", Timeout: 5 * time.Second})
				return c.CompleteWithStreaming(ctx, "sys", "user", false)
			},
		},
		{
			name:         "dashscope",
			wantProvider: "dashscope",
			handler:      func(w http.ResponseWriter, r *http.Request) { sseChatStream(w, deltas, 11, 7) },
			stream: func(ctx context.Context, baseURL string) (<-chan string, <-chan error) {
				c, err := NewOpenAICompatClient(OpenAICompatConfig{Vendor: ProviderDashScope, APIKey: "k", BaseURL: baseURL, Model: "qwen", Timeout: 5 * time.Second})
				if err != nil {
					t.Fatalf("client: %v", err)
				}
				return c.CompleteWithStreaming(ctx, "sys", "user", false)
			},
		},
		{
			name:         "zai",
			wantProvider: "zai",
			handler:      func(w http.ResponseWriter, r *http.Request) { sseChatStream(w, deltas, 11, 7) },
			stream: func(ctx context.Context, baseURL string) (<-chan string, <-chan error) {
				c := NewZAIClientWithConfig(ZAIConfig{APIKey: "k", BaseURL: baseURL, Model: "glm", Timeout: 5 * time.Second, DisableSemaphore: true})
				return c.CompleteWithStreaming(ctx, "sys", "user", false)
			},
		},
		{
			name:         "anthropic",
			wantProvider: "anthropic",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				flusher, _ := w.(http.Flusher)
				fmt.Fprint(w, "data: {\"type\":\"message_start\",\"message\":{\"usage\":{\"input_tokens\":11,\"output_tokens\":1}}}\n\n")
				for _, d := range deltas {
					fmt.Fprintf(w, "data: {\"type\":\"content_block_delta\",\"delta\":{\"type\":\"text_delta\",\"text\":%q}}\n\n", d)
				}
				fmt.Fprint(w, "data: {\"type\":\"message_delta\",\"usage\":{\"output_tokens\":7}}\n\n")
				fmt.Fprint(w, "data: [DONE]\n\n")
				if flusher != nil {
					flusher.Flush()
				}
			},
			stream: func(ctx context.Context, baseURL string) (<-chan string, <-chan error) {
				c := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "k", BaseURL: baseURL, Model: "claude-x", Timeout: 5 * time.Second})
				return c.CompleteWithStreaming(ctx, "sys", "user", false)
			},
		},
		{
			name:         "gemini",
			wantProvider: "gemini",
			handler: func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "text/event-stream")
				flusher, _ := w.(http.Flusher)
				for _, d := range deltas {
					fmt.Fprintf(w, "data: {\"candidates\":[{\"content\":{\"parts\":[{\"text\":%q}]}}]}\n\n", d)
				}
				fmt.Fprint(w, "data: {\"candidates\":[{\"finishReason\":\"STOP\",\"content\":{\"parts\":[]}}],\"usageMetadata\":{\"promptTokenCount\":11,\"candidatesTokenCount\":4,\"thoughtsTokenCount\":3,\"totalTokenCount\":18}}\n\n")
				if flusher != nil {
					flusher.Flush()
				}
			},
			stream: func(ctx context.Context, baseURL string) (<-chan string, <-chan error) {
				c := NewGeminiClientWithConfig(GeminiConfig{APIKey: "k", BaseURL: baseURL, Model: "gemini-x", Timeout: 5 * time.Second})
				return c.CompleteWithStreaming(ctx, "sys", "user", false)
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(tc.handler)
			defer srv.Close()

			ctx, tracker := trackedContext(t)
			contentChan, errChan := tc.stream(ctx, srv.URL)

			var sb strings.Builder
			for chunk := range contentChan {
				sb.WriteString(chunk)
			}
			for err := range errChan {
				t.Fatalf("stream error: %v", err)
			}
			if sb.String() != "one two three" {
				t.Fatalf("streamed content=%q", sb.String())
			}

			// Tokens are billed once per stream, from the final usage payload —
			// not once per delta and not zero.
			stats := tracker.Stats()
			got := stats.ByProvider[tc.wantProvider]
			if got.Input != 11 || got.Output != 7 {
				t.Errorf("provider %s: input=%d output=%d, want 11/7 (ByProvider=%v)",
					tc.wantProvider, got.Input, got.Output, stats.ByProvider)
			}
			if stats.TotalProject.Total != 18 {
				t.Errorf("total=%d, want 18 — a stream tracked per delta or not at all", stats.TotalProject.Total)
			}
		})
	}
}

func TestOpenAICompatExecuteChat_WhenThinkingRetryFires_ShouldTrackBothBilledCalls(t *testing.T) {
	// The empty-content retry is a second billed request. Counting it once
	// would under-report every recovered turn.
	var calls int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		if calls == 1 {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"choices": []map[string]any{
					{"index": 0, "finish_reason": "stop", "message": map[string]any{
						"role": "assistant", "content": "", "reasoning_content": "thinking hard",
					}},
				},
				"usage": map[string]any{"prompt_tokens": 10, "completion_tokens": 5, "total_tokens": 15},
			})
			return
		}
		_ = json.NewEncoder(w).Encode(openAIChatBody(10, 5))
	}))
	defer srv.Close()

	c, err := NewOpenAICompatClient(OpenAICompatConfig{
		Vendor: ProviderDashScope, APIKey: "k", BaseURL: srv.URL, Model: "qwen",
		Timeout: 5 * time.Second, EnableThinking: true,
	})
	if err != nil {
		t.Fatalf("client: %v", err)
	}

	ctx, tracker := trackedContext(t)
	if _, err := c.CompleteWithSystem(ctx, "sys", "user"); err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if calls != 2 {
		t.Fatalf("expected the thinking retry to fire, got %d calls", calls)
	}
	if total := tracker.Stats().TotalProject.Total; total != 30 {
		t.Errorf("total=%d, want 30 (both billed attempts)", total)
	}
}

func TestToolCompletion_WhenToolsOffered_ShouldTrackUnderToolGenOperation(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(openAIChatBody(11, 7))
	}))
	defer srv.Close()

	c := NewOpenAIClientWithConfig(OpenAIConfig{APIKey: "k", BaseURL: srv.URL, Model: "gpt-x", Timeout: 5 * time.Second})
	ctx, tracker := trackedContext(t)
	if _, err := c.CompleteWithTools(ctx, "sys", "user", []ToolDefinition{{Name: "read_file", Description: "d"}}); err != nil {
		t.Fatalf("CompleteWithTools: %v", err)
	}

	stats := tracker.Stats()
	if got := stats.ByOperation[usageOpToolGen]; got.Total != 18 {
		t.Errorf("ByOperation[%s]=%d, want 18 (ByOperation=%v)", usageOpToolGen, got.Total, stats.ByOperation)
	}
}
