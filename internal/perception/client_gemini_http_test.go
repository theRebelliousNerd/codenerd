package perception

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestExtractResponseText(t *testing.T) {
	parts := []GeminiResponsePart{
		{Text: "answer-a"},
		{Text: "thinking", Thought: true},
		{Text: "answer-b"},
	}
	thoughts, resp := extractResponseText(parts)
	if thoughts != "thinking" {
		t.Errorf("thoughts=%q, want thinking", thoughts)
	}
	if resp != "answer-aanswer-b" {
		t.Errorf("response=%q, want answer-aanswer-b", resp)
	}
}

func TestGeminiCompleteWithSystem_HTTP(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "generateContent") {
			t.Errorf("unexpected request path: %s", r.URL.Path)
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"the answer"}]}}]}`))
	}))
	defer srv.Close()

	c := NewGeminiClientWithConfig(GeminiConfig{
		BaseURL:         srv.URL,
		APIKey:          "test-key",
		Model:           "gemini-2.0-flash",
		MaxOutputTokens: 1024,
		Timeout:         30 * time.Second,
	})
	out, err := c.CompleteWithSystem(context.Background(), "you are a tester", "say hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if out != "the answer" {
		t.Errorf("CompleteWithSystem=%q, want 'the answer'", out)
	}
}

func TestGeminiCompleteWithSystem_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No candidates -> the client should report an error rather than empty success.
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"candidates":[]}`))
	}))
	defer srv.Close()

	c := NewGeminiClientWithConfig(GeminiConfig{BaseURL: srv.URL, APIKey: "k", Model: "gemini-2.0", MaxOutputTokens: 256, Timeout: 30 * time.Second})
	if _, err := c.CompleteWithSystem(context.Background(), "s", "u"); err == nil {
		t.Error("expected an error when the response has no candidates")
	}
}
