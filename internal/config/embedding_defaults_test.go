package config

import (
	"testing"

	"codenerd/internal/embedding"
)

// The config a workspace starts from carries the embedding package's defaults,
// not a second copy of them. Round-tripping through EngineConfig must give back
// exactly what the package that builds the engines would choose.
func TestDefaultEmbeddingConfig_ShouldBeTheEmbeddingPackageDefaults(t *testing.T) {
	t.Parallel()
	got := DefaultEmbeddingConfig().EngineConfig()
	if want := embedding.DefaultConfig(); got != want {
		t.Fatalf("DefaultEmbeddingConfig().EngineConfig() = %+v, want embedding.DefaultConfig() %+v", got, want)
	}
	// No model is ever assumed on either side.
	if got.OllamaModel != "" || got.GenAIModel != "" {
		t.Fatalf("a default embedding model was invented: %+v", got)
	}
}
