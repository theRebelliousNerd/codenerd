package perception

import (
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/embedding"
)

// Test-only constructors and accessors. Production builds every client through
// NewClientFromConfig / the *WithConfig constructors, the semantic classifier
// through NewSemanticClassifierFromConfig, and classifies through
// ClassifyInputWithMatches; these shorthands exist so tests can inject stores
// and defaults without going through configuration files.

// NewGeminiClient creates a Gemini client with DefaultGeminiConfig.
func NewGeminiClient(apiKey string) *GeminiClient {
	return NewGeminiClientWithConfig(DefaultGeminiConfig(apiKey))
}

// NewOllamaClient creates an Ollama chat client with defaults.
func NewOllamaClient(model string) *OllamaClient {
	cfg := DefaultOllamaLLMConfig()
	if model != "" {
		cfg.Model = model
	}
	return NewOllamaClientWithConfig(cfg)
}

// NewSemanticClassifier creates a classifier over injected stores.
func NewSemanticClassifier(
	kernel core.Kernel,
	embeddedStore *EmbeddedCorpusStore,
	learnedStore *LearnedCorpusStore,
	embedEngine embedding.EmbeddingEngine,
) *SemanticClassifier {
	return &SemanticClassifier{
		kernel:        kernel,
		embeddedStore: embeddedStore,
		learnedStore:  learnedStore,
		embedEngine:   embedEngine,
		config:        DefaultSemanticConfig(),
	}
}

// SetConfig replaces the classifier configuration.
func (sc *SemanticClassifier) SetConfig(cfg SemanticConfig) {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	sc.config = cfg
}

// ClassifyInput scores candidates with no neural matches bridged in.
func (t *TaxonomyEngine) ClassifyInput(input string, candidates []VerbEntry) (bestVerb string, bestConf float64, err error) {
	return t.ClassifyInputWithMatches(input, candidates, nil)
}

// loadProviderConfigFile resolves a provider config from a config file the way
// boot does: parse once, then derive the provider contract from that parse.
func loadProviderConfigFile(path string) (*ProviderConfig, error) {
	userCfg, err := config.LoadUserConfig(path)
	if err != nil {
		return nil, err
	}
	return ProviderConfigFromUserConfig(userCfg)
}
