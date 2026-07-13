package config

import "testing"

func TestGetOllamaLLMConfig_Defaults(t *testing.T) {
	cfg := &UserConfig{}
	o := cfg.GetOllamaLLMConfig()
	if o.Model != "gemma4:12b" {
		t.Fatalf("model=%q", o.Model)
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

func TestIsImageGenerationModel_NanoBanana2(t *testing.T) {
	if !IsImageGenerationModel("gemini-3.1-flash-image") {
		t.Fatal("expected Nano Banana 2 API id")
	}
	if !IsImageGenerationModel("gemini-3.1-flash-lite-image") {
		t.Fatal("expected Nano Banana 2 Lite")
	}
	if IsImageGenerationModel("gemma4:12b") {
		t.Fatal("ollama chat is not image gen")
	}
	if !IsImageShardType("image_generator") {
		t.Fatal("image_generator shard type")
	}
	if IsImageShardType("coder") {
		t.Fatal("coder is not image")
	}
}

func TestGetImageLLMConfig_Defaults(t *testing.T) {
	cfg := &UserConfig{}
	img := cfg.GetImageLLMConfig()
	if img.Model != DefaultImageModel || img.Provider != "gemini" {
		t.Fatalf("%+v", img)
	}
	cfg.Image = &ImageLLMConfig{Model: "nano-banana-2"}
	img = cfg.GetImageLLMConfig()
	if img.Model != DefaultImageModel {
		t.Fatalf("alias normalize got %q", img.Model)
	}
}
