package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestOpenAIComplete_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasSuffix(r.URL.Path, "/chat/completions") {
			t.Errorf("unexpected path: %s", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Errorf("Authorization=%q, want Bearer test-key", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []map[string]any{
				{"index": 0, "message": map[string]any{"role": "assistant", "content": "  hello there  "}},
			},
		})
	}))
	defer srv.Close()

	c := NewOpenAIClientWithConfig(OpenAIConfig{
		APIKey:  "test-key",
		BaseURL: srv.URL,
		Model:   "gpt-4o-mini",
		Timeout: 30 * time.Second,
	})

	out, err := c.Complete(context.Background(), "say hi")
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	// Response is trimmed of surrounding whitespace.
	if out != "hello there" {
		t.Errorf("Complete=%q, want 'hello there'", out)
	}
}

func TestOpenAICompleteWithSystem_NoChoices(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"choices":[]}`))
	}))
	defer srv.Close()

	c := NewOpenAIClientWithConfig(OpenAIConfig{
		APIKey:  "k",
		BaseURL: srv.URL,
		Model:   "gpt-4o-mini",
		Timeout: 30 * time.Second,
	})
	if _, err := c.CompleteWithSystem(context.Background(), "sys", "user"); err == nil {
		t.Error("expected an error when the API returns no choices")
	}
}

func TestOpenAIComplete_MissingAPIKey(t *testing.T) {
	c := NewOpenAIClientWithConfig(OpenAIConfig{BaseURL: "http://unused", Model: "m", Timeout: time.Second})
	if _, err := c.Complete(context.Background(), "hi"); err == nil {
		t.Error("expected an error when the API key is not configured")
	}
}
