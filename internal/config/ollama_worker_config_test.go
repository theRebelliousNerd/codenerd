package config

import "testing"

func TestGetOllamaLLMConfig_Defaults(t *testing.T) {
	cfg := &UserConfig{}
	o := cfg.GetOllamaLLMConfig()
	// No invented model: an Ollama workspace names its chat model or client
	// construction fails and says so.
	if o.Model != "" {
		t.Fatalf("model=%q, want empty", o.Model)
	}
	if o.Endpoint == "" {
		t.Fatal("empty endpoint")
	}
}

func TestGetWorkerLLMConfig_Ollama(t *testing.T) {
	cfg := &UserConfig{
		Worker: &WorkerLLMConfig{Provider: "ollama", Model: "gemma4:12b"},
		Ollama: &OllamaLLMConfig{Endpoint: "http://localhost:11434", Model: "gemma4:12b"},
	}
	w := cfg.GetWorkerLLMConfig()
	if w == nil || w.Provider != "ollama" || w.Model != "gemma4:12b" {
		t.Fatalf("%+v", w)
	}
}

func TestGetActiveProvider_Ollama(t *testing.T) {
	cfg := &UserConfig{Provider: "ollama"}
	p, k := cfg.GetActiveProvider()
	if p != "ollama" || k != "ollama" {
		t.Fatalf("p=%q k=%q", p, k)
	}
}

func TestGetImageLLMConfig_Defaults(t *testing.T) {
	cfg := &UserConfig{}
	img := cfg.GetImageLLMConfig()
	// The provider defaults (gemini is the only one); the model does not.
	if img.Model != "" || img.Provider != "gemini" {
		t.Fatalf("%+v", img)
	}
	cfg.Image = &ImageLLMConfig{Model: "nano-banana-2"}
	img = cfg.GetImageLLMConfig()
	if img.Model != NanoBanana2ImageModel {
		t.Fatalf("alias normalize got %q", img.Model)
	}
}
