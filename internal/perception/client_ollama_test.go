package perception

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"codenerd/internal/config"
)

func TestNewOllamaClient_Defaults(t *testing.T) {
	c := NewOllamaClient("")
	if c == nil {
		t.Fatal("nil client")
	}
	if c.GetModel() != "gemma4:12b" {
		t.Fatalf("model=%q want gemma4:12b", c.GetModel())
	}
	if !strings.Contains(c.openai.baseURL, "11434") {
		t.Fatalf("baseURL=%q expected ollama port", c.openai.baseURL)
	}
	if c.openai.apiKey != "ollama" {
		t.Fatalf("apiKey=%q want sentinel ollama", c.openai.apiKey)
	}
}

func TestOllamaClient_CompleteWithSystem_OpenAICompat(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			t.Errorf("path=%s want /v1/chat/completions", r.URL.Path)
		}
		var req OpenAIRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Errorf("decode: %v", err)
		}
		if req.Model != "gemma4:12b" {
			t.Errorf("model=%q", req.Model)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"id": "test",
			"choices": []map[string]any{
				{"message": map[string]any{"role": "assistant", "content": "hello local"}, "finish_reason": "stop"},
			},
			"usage": map[string]int{"prompt_tokens": 1, "completion_tokens": 1, "total_tokens": 2},
		})
	}))
	defer srv.Close()

	c := NewOllamaClientWithConfig(OllamaLLMConfig{
		Endpoint: srv.URL,
		Model:    "gemma4:12b",
		Timeout:  5 * time.Second,
	})
	out, err := c.CompleteWithSystem(context.Background(), "sys", "hi")
	if err != nil {
		t.Fatalf("CompleteWithSystem: %v", err)
	}
	if out != "hello local" {
		t.Fatalf("out=%q", out)
	}
}

func TestNewClientFromConfig_Ollama(t *testing.T) {
	cfg := &ProviderConfig{
		Engine:   "api",
		Provider: ProviderOllama,
		APIKey:   "ollama",
		Model:    "gemma4:12b",
		Ollama:   &config.OllamaLLMConfig{Endpoint: "http://127.0.0.1:11434", Model: "gemma4:12b"},
	}
	client, err := NewClientFromConfig(cfg)
	if err != nil {
		t.Fatalf("NewClientFromConfig ollama: %v", err)
	}
	oc, ok := client.(*OllamaClient)
	if !ok {
		t.Fatalf("type=%T want *OllamaClient", client)
	}
	if oc.GetModel() != "gemma4:12b" {
		t.Fatalf("model=%q", oc.GetModel())
	}
}

func TestNewWorkerClientFromUserConfig_Ollama(t *testing.T) {
	uc := &config.UserConfig{
		Provider: "xai",
		Model:    "grok-4.5",
		Worker: &config.WorkerLLMConfig{
			Provider: "ollama",
			Model:    "gemma4:12b",
			Endpoint: "http://127.0.0.1:11434",
		},
	}
	client, err := NewWorkerClientFromUserConfig(uc)
	if err != nil {
		t.Fatalf("worker: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil worker client")
	}
	oc, ok := client.(*OllamaClient)
	if !ok {
		t.Fatalf("type=%T", client)
	}
	if oc.GetModel() != "gemma4:12b" {
		t.Fatalf("model=%q", oc.GetModel())
	}
}

func TestNewWorkerClientFromUserConfig_Nil(t *testing.T) {
	client, err := NewWorkerClientFromUserConfig(&config.UserConfig{Provider: "xai"})
	if err != nil {
		t.Fatal(err)
	}
	if client != nil {
		t.Fatal("expected nil when worker unset")
	}
}
