package config

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEnvOverrides_LLM(t *testing.T) {
	t.Run("ZAI_API_KEY sets provider if empty", func(t *testing.T) {
		t.Setenv("ZAI_API_KEY", "zai-key")
		// Ensure others are unset
		t.Setenv("ANTHROPIC_API_KEY", "")

		cfg := &Config{}
		cfg.applyEnvOverrides()

		assert.Equal(t, "zai-key", cfg.LLM.APIKey)
		assert.Equal(t, "zai", cfg.LLM.Provider)
	})

	t.Run("ZAI_API_KEY does not override existing provider", func(t *testing.T) {
		t.Setenv("ZAI_API_KEY", "zai-key")

		cfg := &Config{
			LLM: LLMConfig{Provider: "custom"},
		}
		cfg.applyEnvOverrides()

		assert.Equal(t, "zai-key", cfg.LLM.APIKey)
		assert.Equal(t, "custom", cfg.LLM.Provider)
	})

	t.Run("ANTHROPIC_API_KEY overrides provider", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "ant-key")

		cfg := &Config{
			LLM: LLMConfig{Provider: "initial"},
		}
		cfg.applyEnvOverrides()

		assert.Equal(t, "ant-key", cfg.LLM.APIKey)
		assert.Equal(t, "anthropic", cfg.LLM.Provider)
	})

	t.Run("Precedence: OPENAI overrides ANTHROPIC", func(t *testing.T) {
		t.Setenv("ANTHROPIC_API_KEY", "ant-key")
		t.Setenv("OPENAI_API_KEY", "oa-key")

		cfg := &Config{}
		cfg.applyEnvOverrides()

		assert.Equal(t, "oa-key", cfg.LLM.APIKey)
		assert.Equal(t, "openai", cfg.LLM.Provider)
	})

	t.Run("Precedence: Full Chain", func(t *testing.T) {
		// We can't easily unset env vars with t.Setenv in a loop effectively to simulate removal
		// because t.Setenv restores the *original* value at cleanup, it doesn't support "unsetting" if it was set by parent.
		// So we will write separate assertions for each stage of precedence.

		// 1. All set -> OPENROUTER wins
		t.Run("All Set -> OpenRouter", func(t *testing.T) {
			setAllLLMKeys(t)
			cfg := &Config{}
			cfg.applyEnvOverrides()
			assert.Equal(t, "or", cfg.LLM.APIKey)
			assert.Equal(t, "openrouter", cfg.LLM.Provider)
		})

		// 2. No OpenRouter -> XAI wins
		t.Run("No OpenRouter -> XAI", func(t *testing.T) {
			setAllLLMKeys(t)
			t.Setenv("OPENROUTER_API_KEY", "")
			cfg := &Config{}
			cfg.applyEnvOverrides()
			assert.Equal(t, "xai", cfg.LLM.APIKey)
			assert.Equal(t, "xai", cfg.LLM.Provider)
		})

		// 3. No XAI -> Gemini wins
		t.Run("No XAI -> Gemini", func(t *testing.T) {
			setAllLLMKeys(t)
			t.Setenv("OPENROUTER_API_KEY", "")
			t.Setenv("XAI_API_KEY", "")
			cfg := &Config{}
			cfg.applyEnvOverrides()
			assert.Equal(t, "gem", cfg.LLM.APIKey)
			assert.Equal(t, "gemini", cfg.LLM.Provider)
		})

		// 4. No Gemini -> OpenAI wins
		t.Run("No Gemini -> OpenAI", func(t *testing.T) {
			setAllLLMKeys(t)
			t.Setenv("OPENROUTER_API_KEY", "")
			t.Setenv("XAI_API_KEY", "")
			t.Setenv("GEMINI_API_KEY", "")
			cfg := &Config{}
			cfg.applyEnvOverrides()
			assert.Equal(t, "oa", cfg.LLM.APIKey)
			assert.Equal(t, "openai", cfg.LLM.Provider)
		})

		// 5. No OpenAI -> Anthropic wins
		t.Run("No OpenAI -> Anthropic", func(t *testing.T) {
			setAllLLMKeys(t)
			t.Setenv("OPENROUTER_API_KEY", "")
			t.Setenv("XAI_API_KEY", "")
			t.Setenv("GEMINI_API_KEY", "")
			t.Setenv("OPENAI_API_KEY", "")
			cfg := &Config{}
			cfg.applyEnvOverrides()
			assert.Equal(t, "ant", cfg.LLM.APIKey)
			assert.Equal(t, "anthropic", cfg.LLM.Provider)
		})

		// 6. No Anthropic -> ZAI wins
		t.Run("No Anthropic -> ZAI", func(t *testing.T) {
			setAllLLMKeys(t)
			t.Setenv("OPENROUTER_API_KEY", "")
			t.Setenv("XAI_API_KEY", "")
			t.Setenv("GEMINI_API_KEY", "")
			t.Setenv("OPENAI_API_KEY", "")
			t.Setenv("ANTHROPIC_API_KEY", "")
			cfg := &Config{}
			cfg.applyEnvOverrides()
			assert.Equal(t, "zai", cfg.LLM.APIKey)
			assert.Equal(t, "zai", cfg.LLM.Provider)
		})
	})
}

func setAllLLMKeys(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "zai")
	t.Setenv("ANTHROPIC_API_KEY", "ant")
	t.Setenv("OPENAI_API_KEY", "oa")
	t.Setenv("GEMINI_API_KEY", "gem")
	t.Setenv("XAI_API_KEY", "xai")
	t.Setenv("OPENROUTER_API_KEY", "or")
}

func TestEnvOverrides_Embedding(t *testing.T) {
	t.Run("GENAI_API_KEY sets provider if empty", func(t *testing.T) {
		t.Setenv("GENAI_API_KEY", "gen-key")

		cfg := &Config{}
		cfg.applyEnvOverrides()

		assert.Equal(t, "gen-key", cfg.Embedding.GenAIAPIKey)
		assert.Equal(t, "genai", cfg.Embedding.Provider)
	})

	t.Run("GENAI_API_KEY sets provider if ollama", func(t *testing.T) {
		t.Setenv("GENAI_API_KEY", "gen-key")

		cfg := &Config{
			Embedding: EmbeddingConfig{Provider: "ollama"},
		}
		cfg.applyEnvOverrides()

		assert.Equal(t, "gen-key", cfg.Embedding.GenAIAPIKey)
		assert.Equal(t, "genai", cfg.Embedding.Provider)
	})

	t.Run("GENAI_API_KEY does not override other providers", func(t *testing.T) {
		t.Setenv("GENAI_API_KEY", "gen-key")

		cfg := &Config{
			Embedding: EmbeddingConfig{Provider: "openai"},
		}
		cfg.applyEnvOverrides()

		assert.Equal(t, "gen-key", cfg.Embedding.GenAIAPIKey)
		assert.Equal(t, "openai", cfg.Embedding.Provider)
	})

	t.Run("GEMINI_API_KEY fallback", func(t *testing.T) {
		t.Setenv("GEMINI_API_KEY", "gem-key")
		t.Setenv("GENAI_API_KEY", "")

		cfg := &Config{}
		cfg.applyEnvOverrides()

		assert.Equal(t, "gem-key", cfg.Embedding.GenAIAPIKey)
		assert.Equal(t, "genai", cfg.Embedding.Provider)
	})

	t.Run("GENAI_API_KEY priority over GEMINI_API_KEY", func(t *testing.T) {
		t.Setenv("GENAI_API_KEY", "gen-key")
		t.Setenv("GEMINI_API_KEY", "gem-key")

		cfg := &Config{}
		cfg.applyEnvOverrides()

		assert.Equal(t, "gen-key", cfg.Embedding.GenAIAPIKey)
	})

	t.Run("Ollama Overrides", func(t *testing.T) {
		t.Setenv("OLLAMA_ENDPOINT", "http://custom:11434")
		t.Setenv("OLLAMA_EMBEDDING_MODEL", "custom-model")

		cfg := &Config{}
		cfg.applyEnvOverrides()

		assert.Equal(t, "http://custom:11434", cfg.Embedding.OllamaEndpoint)
		assert.Equal(t, "custom-model", cfg.Embedding.OllamaModel)
	})
}

func TestEnvOverrides_Integrations_And_DB(t *testing.T) {
	t.Run("Integrations URLs", func(t *testing.T) {
		t.Setenv("CODEGRAPH_URL", "http://codegraph")
		t.Setenv("BROWSERNERD_URL", "http://browser")
		t.Setenv("SCRAPER_URL", "http://scraper")

		cfg := &Config{}
		cfg.applyEnvOverrides()

		require.Contains(t, cfg.Integrations.Servers, "code_graph")
		assert.Equal(t, "http://codegraph", cfg.Integrations.Servers["code_graph"].BaseURL)

		require.Contains(t, cfg.Integrations.Servers, "browser")
		assert.Equal(t, "http://browser", cfg.Integrations.Servers["browser"].BaseURL)

		require.Contains(t, cfg.Integrations.Servers, "scraper")
		assert.Equal(t, "http://scraper", cfg.Integrations.Servers["scraper"].BaseURL)
	})

	t.Run("Database Path", func(t *testing.T) {
		t.Setenv("CODENERD_DB", "/tmp/test.db")

		cfg := &Config{}
		cfg.applyEnvOverrides()

		assert.Equal(t, "/tmp/test.db", cfg.Memory.DatabasePath)
	})
}

func TestMCPServerEnabledHelpers(t *testing.T) {
	cfg := &Config{
		Integrations: IntegrationsConfig{
			Servers: map[string]MCPServerIntegration{
				"code_graph": {Enabled: true},
				"browser":    {Enabled: true},
				"scraper":    {Enabled: true},
				"other":      {Enabled: false},
			},
		},
	}

	assert.True(t, cfg.IsCodeGraphEnabled())
	assert.True(t, cfg.IsBrowserEnabled())
	assert.True(t, cfg.IsScraperEnabled())
	assert.True(t, cfg.IsMCPServerEnabled("code_graph"))
	assert.False(t, cfg.IsMCPServerEnabled("other"))
	assert.False(t, cfg.IsMCPServerEnabled("missing"))

	// Test with nil map
	emptyCfg := &Config{}
	assert.False(t, emptyCfg.IsCodeGraphEnabled())
}
