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
