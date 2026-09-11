package perception

import (
	"codenerd/internal/broker"
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
	oc, ok := broker.Base(client).(*OllamaClient)
	if !ok {
		t.Fatalf("type=%T want *OllamaClient", broker.Base(client))
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
	oc, ok := broker.Base(client).(*OllamaClient)
	if !ok {
		t.Fatalf("type=%T", broker.Base(client))
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

func TestNewImageClientFromUserConfig_NeverOllama(t *testing.T) {
	// Even when worker is ollama and provider is ollama, image client must be Gemini.
	uc := &config.UserConfig{
		Provider:     "ollama",
		Model:        "gemma4:12b",
		GeminiAPIKey: "test-gemini-key",
		Worker: &config.WorkerLLMConfig{
			Provider: "ollama",
			Model:    "gemma4:12b",
		},
		Image: &config.ImageLLMConfig{
			Provider: "gemini",
			Model:    "nano-banana-2", // alias → gemini-3.1-flash-image
		},
	}
	client, err := NewImageClientFromUserConfig(uc)
	if err != nil {
		t.Fatalf("image client: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil image client")
	}
	if _, ok := broker.Base(client).(*OllamaClient); ok {
		t.Fatal("image client must never be Ollama")
	}
	gc, ok := broker.Base(client).(*GeminiClient)
	if !ok {
		t.Fatalf("want *GeminiClient, got %T", broker.Base(client))
	}
	if gc.GetModel() != config.NanoBanana2ImageModel {
		t.Fatalf("model=%q want %q", gc.GetModel(), config.NanoBanana2ImageModel)
	}

	// No invented model: an unset image.model is an error, not a tier pick.
	uc.Image = &config.ImageLLMConfig{Provider: "gemini"}
	if _, err := NewImageClientFromUserConfig(uc); err == nil || !strings.Contains(err.Error(), "image.model") {
		t.Fatalf("expected an image.model error, got %v", err)
	}

	// Unsupported image provider must fail closed (not fall through to ollama).
	uc.Image = &config.ImageLLMConfig{Provider: "ollama", Model: "gemma4:12b"}
	_, err = NewImageClientFromUserConfig(uc)
	if err == nil {
		t.Fatal("expected error for image provider=ollama")
	}
}
