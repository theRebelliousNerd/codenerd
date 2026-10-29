package perception

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

// The caller owns the system prompt. A client handed an empty one sends no
// system instruction at all: no client invents a persona of its own (until
// 2026-10-28 every engine substituted, and Z.AI prepended to every prompt, a
// hard-coded "You are codeNERD..." line). A supplied prompt reaches the wire
// unchanged.
func TestClients_SystemPromptIsTheCallersOrNone(t *testing.T) {
	openAIBody := func(w http.ResponseWriter) { _ = json.NewEncoder(w).Encode(openAIChatBody(1, 1)) }
	cases := []struct {
		name   string
		reply  func(http.ResponseWriter)
		call   func(ctx context.Context, baseURL, system string) error
		system func(t *testing.T, body map[string]any) (string, bool)
	}{
		{
			name:  "openai",
			reply: func(w http.ResponseWriter) { _ = json.NewEncoder(w).Encode(openAIResponsesBody(1, 1)) },
			call: func(ctx context.Context, baseURL, system string) error {
				c, err := NewOpenAIClient(OpenAIConfig{APIKey: "k", BaseURL: baseURL, Model: "gpt-6.1-sol", Timeout: 5 * time.Second})
				if err != nil {
					return err
				}
				_, err = c.CompleteWithSystem(ctx, system, "user")
				return err
			},
			system: responsesDeveloperText,
		},
		{
			name:  "xai",
			reply: openAIBody,
			call: func(ctx context.Context, baseURL, system string) error {
				_, err := NewXAIClientWithConfig(XAIConfig{APIKey: "k", BaseURL: baseURL, Model: "grok", Timeout: 5 * time.Second}).CompleteWithSystem(ctx, system, "user")
				return err
			},
			system: chatSystemMessage,
		},
		{
			name:  "openrouter",
			reply: openAIBody,
			call: func(ctx context.Context, baseURL, system string) error {
				_, err := NewOpenRouterClientWithConfig(OpenRouterConfig{APIKey: "k", BaseURL: baseURL, Model: "some/model", Timeout: 5 * time.Second}).CompleteWithSystem(ctx, system, "user")
				return err
			},
			system: chatSystemMessage,
		},
		{
			name:  "zai",
			reply: openAIBody,
			call: func(ctx context.Context, baseURL, system string) error {
				_, err := NewZAIClientWithConfig(ZAIConfig{APIKey: "k", BaseURL: baseURL, Model: "glm", Timeout: 5 * time.Second, DisableSemaphore: true}).CompleteWithSystem(ctx, system, "user")
				return err
			},
			system: chatSystemMessage,
		},
		{
			name:  "openai_compat",
			reply: openAIBody,
			call: func(ctx context.Context, baseURL, system string) error {
				c, err := NewOpenAICompatClient(OpenAICompatConfig{Vendor: ProviderDashScope, APIKey: "k", BaseURL: baseURL, Model: "qwen", Timeout: 5 * time.Second})
				if err != nil {
					return err
				}
				_, err = c.CompleteWithSystem(ctx, system, "user")
				return err
			},
			system: chatSystemMessage,
		},
		{
			name: "anthropic",
			reply: func(w http.ResponseWriter) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"type": "message", "stop_reason": "end_turn",
					"content": []map[string]any{{"type": "text", "text": "hi"}},
					"usage":   map[string]any{"input_tokens": 1, "output_tokens": 1},
				})
			},
			call: func(ctx context.Context, baseURL, system string) error {
				_, err := NewAnthropicClientWithConfig(AnthropicConfig{APIKey: "k", BaseURL: baseURL, Model: "claude-x", Timeout: 5 * time.Second}).CompleteWithSystem(ctx, system, "user")
				return err
			},
			system: func(t *testing.T, body map[string]any) (string, bool) {
				switch v := body["system"].(type) {
				case nil:
					return "", false
				case string:
					return v, true
				case []any:
					var text strings.Builder
					for _, block := range v {
						if m, ok := block.(map[string]any); ok {
							text.WriteString(m["text"].(string))
						}
					}
					return text.String(), true
				default:
					t.Fatalf("anthropic system field has type %T", v)
					return "", false
				}
			},
		},
		{
			name: "gemini",
			reply: func(w http.ResponseWriter) {
				_ = json.NewEncoder(w).Encode(map[string]any{
					"candidates":    []map[string]any{{"content": map[string]any{"parts": []map[string]any{{"text": "hi"}}}}},
					"usageMetadata": map[string]any{"promptTokenCount": 1, "candidatesTokenCount": 1, "totalTokenCount": 2},
				})
			},
			call: func(ctx context.Context, baseURL, system string) error {
				_, err := NewGeminiClientWithConfig(GeminiConfig{APIKey: "k", BaseURL: baseURL, Model: "gemini-x", Timeout: 5 * time.Second}).CompleteWithSystem(ctx, system, "user")
				return err
			},
			system: func(t *testing.T, body map[string]any) (string, bool) {
				instruction, ok := body["systemInstruction"].(map[string]any)
				if !ok {
					return "", false
				}
				var text strings.Builder
				for _, part := range instruction["parts"].([]any) {
					text.WriteString(part.(map[string]any)["text"].(string))
				}
				return text.String(), true
			},
		},
	}
	for _, tc := range cases {
		for _, supplied := range []string{"", "the caller's own system prompt"} {
			t.Run(tc.name+"/"+map[bool]string{true: "empty", false: "supplied"}[supplied == ""], func(t *testing.T) {
				var body map[string]any
				srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					raw, err := io.ReadAll(r.Body)
					if err != nil {
						t.Errorf("read request: %v", err)
					}
					if err := json.Unmarshal(raw, &body); err != nil {
						t.Errorf("decode request: %v", err)
					}
					tc.reply(w)
				}))
				defer srv.Close()
				if err := tc.call(context.Background(), srv.URL, supplied); err != nil {
					t.Fatalf("completion: %v", err)
				}
				got, sent := tc.system(t, body)
				if supplied == "" && sent {
					t.Fatalf("an empty system prompt went out as %q; the client invented one", got)
				}
				if supplied != "" && got != supplied {
					t.Fatalf("system prompt on the wire = %q, want the caller's %q unchanged", got, supplied)
				}
			})
		}
	}
}

// chatSystemMessage reads the system message of an OpenAI-style chat body.
func chatSystemMessage(t *testing.T, body map[string]any) (string, bool) {
	t.Helper()
	messages, _ := body["messages"].([]any)
	for _, raw := range messages {
		m, ok := raw.(map[string]any)
		if !ok {
			t.Fatalf("chat message has type %T", raw)
		}
		if m["role"] == "system" {
			content, _ := m["content"].(string)
			return content, true
		}
	}
	return "", false
}
