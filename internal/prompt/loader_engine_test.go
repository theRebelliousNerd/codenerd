package prompt

import (
	"context"
	"testing"
)

type enginePinStub struct{ dim int }

func (s enginePinStub) Embed(_ context.Context, _ string) ([]float32, error) {
	out := make([]float32, s.dim)
	for i := range out {
		out[i] = 0.3
	}
	return out, nil
}
func (s enginePinStub) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range out {
		vec := make([]float32, s.dim)
		for j := range vec {
			vec[j] = 0.3
		}
		out[i] = vec
	}
	return out, nil
}
func (s enginePinStub) Dimensions() int { return s.dim }
func (s enginePinStub) Name() string    { return "pin-stub" }

// TestAtomLoaderEmbeddingEngine_PinsAccessor drives the EmbeddingEngine
// accessor added for the boot-time knowledge backfill. Removing the method
// breaks compilation; returning the wrong engine breaks the assertion.
func TestAtomLoaderEmbeddingEngine_PinsAccessor(t *testing.T) {
	stub := enginePinStub{dim: 8}
	loader := NewAtomLoader(stub)
	got := loader.EmbeddingEngine()
	if got == nil {
		t.Fatalf("EmbeddingEngine() = nil, want the engine passed to NewAtomLoader")
	}
	if got.Dimensions() != 8 || got.Name() != "pin-stub" {
		t.Fatalf("EmbeddingEngine() = dims %d name %q, want dims 8 name pin-stub", got.Dimensions(), got.Name())
	}
	var nilLoader *AtomLoader
	if nilLoader.EmbeddingEngine() != nil {
		t.Fatalf("nil loader EmbeddingEngine() should be nil")
	}
	// Nil-engine loader must report nil so callers can skip embedding work.
	plain := NewAtomLoader(nil)
	if plain.EmbeddingEngine() != nil {
		t.Fatalf("NewAtomLoader(nil).EmbeddingEngine() should be nil")
	}
}
