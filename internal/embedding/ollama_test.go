package embedding

import (
	"testing"
)

func TestNewOllamaEngine(t *testing.T) {
	t.Run("default parameters", func(t *testing.T) {
		engine, err := NewOllamaEngine("", "")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if engine.endpoint != "http://localhost:11434" {
			t.Errorf("expected endpoint http://localhost:11434, got %s", engine.endpoint)
		}
		if engine.model != defaultOllamaEmbedModel {
			t.Errorf("expected model %s, got %s", defaultOllamaEmbedModel, engine.model)
		}
	})

	t.Run("custom parameters", func(t *testing.T) {
		engine, err := NewOllamaEngine("http://custom:11434/", "custom-model")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if engine.endpoint != "http://custom:11434" {
			t.Errorf("expected endpoint http://custom:11434, got %s", engine.endpoint)
		}
		if engine.model != "custom-model" {
			t.Errorf("expected model custom-model, got %s", engine.model)
		}
	})

	t.Run("embeddinggemma defaults to tagged version", func(t *testing.T) {
		engine, err := NewOllamaEngine("", "embeddinggemma")
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if engine.model != defaultOllamaEmbedModel {
			t.Errorf("expected model %s, got %s", defaultOllamaEmbedModel, engine.model)
		}
	})
}

func TestOllamaEngine_Properties(t *testing.T) {
	engine, err := NewOllamaEngine("http://test:11434", "test-model")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	t.Run("Dimensions", func(t *testing.T) {
		if dim := engine.Dimensions(); dim != 768 {
			t.Errorf("expected dimensions 768, got %d", dim)
		}
	})

	t.Run("Name", func(t *testing.T) {
		if name := engine.Name(); name != "ollama:test-model" {
			t.Errorf("expected name ollama:test-model, got %s", name)
		}
	})

	t.Run("Model", func(t *testing.T) {
		if model := engine.Model(); model != "test-model" {
			t.Errorf("expected model test-model, got %s", model)
		}

		// Test thread safety of Model()
		engine.ensureMu.Lock()
		engine.model = "updated-model"
		engine.ensureMu.Unlock()

		if model := engine.Model(); model != "updated-model" {
			t.Errorf("expected updated model, got %s", model)
		}
	})
}
