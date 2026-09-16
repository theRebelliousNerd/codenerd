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

	"codenerd/internal/types"
)

func ollamaTestClient(baseURL string) *OllamaClient {
	return NewOllamaClientWithConfig(OllamaLLMConfig{
		Endpoint: baseURL,
		Model:    "gemma-test",
		Timeout:  time.Minute,
	})
}

// The multi-turn tool-results path must send the client's completion
// ceiling like every sibling path. Without it max_tokens is omitted
// (omitzero) and tool loops truncate at Ollama's own default.
func TestOllamaToolResults_SendsCeiling(t *testing.T) {
	captured := make(chan map[string]any, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req map[string]any
		if err := json.Unmarshal(body, &req); err != nil {
			t.Errorf("request is not JSON: %v", err)
		}
		captured <- req
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "done"}, "finish_reason": "stop"}],
		  "usage": {"prompt_tokens": 4, "completion_tokens": 1, "total_tokens": 5}
		}`))
	}))
	defer srv.Close()

	c := ollamaTestClient(srv.URL)
	history := []types.Message{
		{Role: "user", Text: "hi"},
		{Role: "assistant", Text: "checking", ToolCalls: []types.ToolCall{
			{ID: "call_1", Name: "ping", Input: map[string]any{}},
		}},
		{Role: "user", ToolResults: []types.ToolResult{
			{ToolUseID: "call_1", Content: "pong"},
		}},
	}
	resp, err := c.CompleteWithToolResults(context.Background(), "sys", history, nil)
	if err != nil {
		t.Fatalf("CompleteWithToolResults: %v", err)
	}
	if resp.Text != "done" {
		t.Errorf("response = %q, want done", resp.Text)
	}
	wire := <-captured
	maxTokens, _ := wire["max_tokens"].(float64)
	if maxTokens != 4096 {
		t.Errorf("wire max_tokens = %v, want the client ceiling 4096", wire["max_tokens"])
	}
	if wire["model"] != "gemma-test" {
		t.Errorf("wire model = %v, want gemma-test", wire["model"])
	}
}

// Chat delegates to the shared OpenAI transport pointed at
// {endpoint}/v1/chat/completions with the placeholder key.
func TestOllamaChat_DelegatesToV1Endpoint(t *testing.T) {
	var gotPath, gotAuth string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "  local ok  "}, "finish_reason": "stop"}],
		  "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer srv.Close()

	c := ollamaTestClient(srv.URL)
	resp, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if resp != "local ok" {
		t.Errorf("response = %q, want trimmed content", resp)
	}
	if gotPath != "/v1/chat/completions" {
		t.Errorf("request path = %q, want /v1/chat/completions", gotPath)
	}
	if gotAuth != "Bearer ollama" {
		t.Errorf("auth header = %q, want the placeholder key", gotAuth)
	}
}

// SetModel must update both the wrapper and the wrapped transport, so the
// next request carries the new model everywhere it is read from.
func TestOllamaSetModel_PropagatesToTransport(t *testing.T) {
	captured := make(chan string, 1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Model string `json:"model"`
		}
		_ = json.Unmarshal(body, &req)
		captured <- req.Model
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{
		  "choices": [{"index": 0, "message": {"role": "assistant", "content": "ok"}, "finish_reason": "stop"}],
		  "usage": {"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2}
		}`))
	}))
	defer srv.Close()

	c := ollamaTestClient(srv.URL)
	c.SetModel("gemma-new")
	if c.GetModel() != "gemma-new" {
		t.Errorf("GetModel = %q, want gemma-new", c.GetModel())
	}
	if provider, model := c.ModelIdentity(); provider != "ollama" || model != "gemma-new" {
		t.Errorf("ModelIdentity = (%q, %q), want (ollama, gemma-new)", provider, model)
	}
	if name := c.Name(); !strings.Contains(name, "gemma-new") {
		t.Errorf("Name = %q, want it to carry the model", name)
	}
	if _, err := c.Complete(context.Background(), "hi"); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if model := <-captured; model != "gemma-new" {
		t.Errorf("wire model = %q, want gemma-new (transport not updated)", model)
	}
}

// An empty SetModel is ignored rather than blanking the client.
func TestOllamaSetModel_EmptyIsIgnored(t *testing.T) {
	c := ollamaTestClient("http://127.0.0.1:1")
	c.SetModel("")
	if c.GetModel() != "gemma-test" {
		t.Errorf("GetModel = %q, want the configured model kept", c.GetModel())
	}
}
