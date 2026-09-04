package config

import (
	"os"
	"path/filepath"
	"testing"
)


// =============================================================================
// UserConfig Tests - GetActiveProvider
// =============================================================================

func TestGetActiveProvider_WhenExplicitProvider_ShouldUseMatchingKey(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		cfg      *UserConfig
		wantProv string
		wantKey  string
	}{
		{
			name:     "anthropic with key",
			provider: "anthropic",
			cfg: &UserConfig{
				Provider:        "anthropic",
				AnthropicAPIKey: "ant-key",
			},
			wantProv: "anthropic",
			wantKey:  "ant-key",
		},
		{
			name:     "openai with key",
			provider: "openai",
			cfg: &UserConfig{
				Provider:     "openai",
				OpenAIAPIKey: "oai-key",
			},
			wantProv: "openai",
			wantKey:  "oai-key",
		},
		{
			name:     "gemini with key",
			provider: "gemini",
			cfg: &UserConfig{
				Provider:     "gemini",
				GeminiAPIKey: "gem-key",
			},
			wantProv: "gemini",
			wantKey:  "gem-key",
		},
		{
			name:     "xai with key",
			provider: "xai",
			cfg: &UserConfig{
				Provider:  "xai",
				XAIAPIKey: "xai-key",
			},
			wantProv: "xai",
			wantKey:  "xai-key",
		},
		{
			name:     "zai with key",
			provider: "zai",
			cfg: &UserConfig{
				Provider:  "zai",
				ZAIAPIKey: "zai-key",
			},
			wantProv: "zai",
			wantKey:  "zai-key",
		},
		{
			name:     "openrouter with key",
			provider: "openrouter",
			cfg: &UserConfig{
				Provider:         "openrouter",
				OpenRouterAPIKey: "or-key",
			},
			wantProv: "openrouter",
			wantKey:  "or-key",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			prov, key := tt.cfg.GetActiveProvider()
			if prov != tt.wantProv {
				t.Errorf("provider = %q, want %q", prov, tt.wantProv)
			}
			if key != tt.wantKey {
				t.Errorf("key = %q, want %q", key, tt.wantKey)
			}
		})
	}
}

func TestGetActiveProvider_WhenNoProvider_ShouldFallbackByPriority(t *testing.T) {
	cfg := &UserConfig{
		AnthropicAPIKey: "ant",
		OpenAIAPIKey:    "oai",
	}
	// No explicit provider — anthropic should win (first in priority)
	prov, key := cfg.GetActiveProvider()
	if prov != "anthropic" || key != "ant" {
		t.Errorf("got (%q, %q), want ('anthropic', 'ant')", prov, key)
	}
}

func TestGetActiveProvider_WhenNoKeys_ShouldReturnEmpty(t *testing.T) {
	cfg := &UserConfig{}
	prov, key := cfg.GetActiveProvider()
	if prov != "" || key != "" {
		t.Errorf("expected empty, got (%q, %q)", prov, key)
	}
}

func TestGetActiveProvider_WhenLegacyAPIKey_ShouldDefaultToZAI(t *testing.T) {
	cfg := &UserConfig{APIKey: "legacy-key"}
	prov, key := cfg.GetActiveProvider()
	if prov != "zai" || key != "legacy-key" {
		t.Errorf("got (%q, %q), want ('zai', 'legacy-key')", prov, key)
	}
}

func TestGetActiveProvider_WhenProviderSetButKeyMissing_ShouldReturnEmptyKey(t *testing.T) {
	// Config is boss: explicit provider with missing key must NOT silently
	// fall back to another provider. The caller is responsible for failing
	// loudly when the key is empty.
	cfg := &UserConfig{
		Provider:        "openai",
		AnthropicAPIKey: "ant-key",
	}
	prov, key := cfg.GetActiveProvider()
	if prov != "openai" || key != "" {
		t.Errorf("got (%q, %q), want ('openai', '') — explicit provider must not silently fall back", prov, key)
	}
}

// =============================================================================
// UserConfig Tests - Engine
// =============================================================================

func TestGetEngine_WhenEmpty_ShouldDefaultToAPI(t *testing.T) {
	cfg := &UserConfig{}
	if got := cfg.GetEngine(); got != "api" {
		t.Errorf("GetEngine() = %q, want 'api'", got)
	}
}

func TestGetEngine_WhenSet_ShouldReturnValue(t *testing.T) {
	cfg := &UserConfig{Engine: "claude-cli"}
	if got := cfg.GetEngine(); got != "claude-cli" {
		t.Errorf("GetEngine() = %q, want 'claude-cli'", got)
	}
}

func TestSetEngine_WhenValid_ShouldSucceed(t *testing.T) {
	validEngines := []string{"api", "claude-cli", "codex-cli", "xai-oauth"}
	for _, engine := range validEngines {
		t.Run(engine, func(t *testing.T) {
			cfg := &UserConfig{}
			if err := cfg.SetEngine(engine); err != nil {
				t.Errorf("SetEngine(%q) error: %v", engine, err)
			}
			if cfg.Engine != engine {
				t.Errorf("Engine = %q, want %q", cfg.Engine, engine)
			}
		})
	}
}

func TestSetEngine_WhenInvalid_ShouldReturnError(t *testing.T) {
	invalidEngines := []string{"", "invalid", "docker", "ssh"}
	for _, engine := range invalidEngines {
		t.Run(engine, func(t *testing.T) {
			cfg := &UserConfig{}
			if err := cfg.SetEngine(engine); err == nil {
				t.Errorf("SetEngine(%q) should have returned error", engine)
			}
		})
	}
}

// =============================================================================
// UserConfig Tests - ClaudeCLI/CodexCLI Defaults
// =============================================================================

func TestGetClaudeCLIConfig_WhenNil_ShouldReturnDefaults(t *testing.T) {
	cfg := &UserConfig{}
	cliCfg := cfg.GetClaudeCLIConfig()
	if cliCfg == nil {
		t.Fatal("expected non-nil ClaudeCLI config")
	}
	if cliCfg.Model != "sonnet" {
		t.Errorf("Model = %q, want 'sonnet'", cliCfg.Model)
	}
	if cliCfg.Timeout != 300 {
		t.Errorf("Timeout = %d, want 300", cliCfg.Timeout)
	}
}

func TestGetClaudeCLIConfig_WhenPartial_ShouldFillDefaults(t *testing.T) {
	cfg := &UserConfig{
		ClaudeCLI: &ClaudeCLIConfig{Model: "opus"},
	}
	cliCfg := cfg.GetClaudeCLIConfig()
	if cliCfg.Model != "opus" {
		t.Errorf("Model = %q, want 'opus'", cliCfg.Model)
	}
	if cliCfg.Timeout != 300 {
		t.Errorf("Timeout should default to 300, got %d", cliCfg.Timeout)
	}
}

func TestGetCodexCLIConfig_WhenNil_ShouldReturnDefaults(t *testing.T) {
	cfg := &UserConfig{}
	codexCfg := cfg.GetCodexCLIConfig()
	if codexCfg == nil {
		t.Fatal("expected non-nil CodexCLI config")
	}
	if codexCfg.Model != "gpt-5.4" {
		t.Errorf("Model = %q, want 'gpt-5.4'", codexCfg.Model)
	}
	if codexCfg.Sandbox != "read-only" {
		t.Errorf("Sandbox = %q, want 'read-only'", codexCfg.Sandbox)
	}
	if codexCfg.SkillEnabled == nil || !*codexCfg.SkillEnabled {
		t.Error("SkillEnabled should default to true")
	}
}

// =============================================================================
// UserConfig Tests - LoadUserConfig
// =============================================================================

func TestLoadUserConfig_WhenMissingFile_ShouldReturnEmptyConfig(t *testing.T) {
	cfg, err := LoadUserConfig("/nonexistent/path/config.json")
	if err != nil {
		t.Fatalf("LoadUserConfig should not error for missing file, got: %v", err)
	}
	if cfg == nil {
		t.Fatal("expected non-nil config for missing file")
	}
	// Should be empty defaults
	if cfg.Provider != "" {
		t.Errorf("Provider should be empty, got %q", cfg.Provider)
	}
}

func TestLoadUserConfig_WhenMalformedJSON_ShouldReturnError(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	os.WriteFile(path, []byte("{{{invalid json"), 0644)

	_, err := LoadUserConfig(path)
	if err == nil {
		t.Fatal("expected error for malformed JSON")
	}
}

func TestLoadUserConfig_WhenValidJSON_ShouldParse(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "config.json")
	content := `{
		"provider": "gemini",
		"gemini_api_key": "gem-key",
		"model": "gemini-pro",
		"theme": "dark"
	}`
	os.WriteFile(path, []byte(content), 0644)

	cfg, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig error: %v", err)
	}
	if cfg.Provider != "gemini" {
		t.Errorf("Provider = %q, want 'gemini'", cfg.Provider)
	}
	if cfg.GeminiAPIKey != "gem-key" {
		t.Errorf("GeminiAPIKey = %q, want 'gem-key'", cfg.GeminiAPIKey)
	}
	if cfg.Theme != "dark" {
		t.Errorf("Theme = %q, want 'dark'", cfg.Theme)
	}
}

// =============================================================================
// UserConfig Tests - Save
// =============================================================================

func TestUserConfigSave_WhenRoundTrip_ShouldPreserve(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, ".nerd", "config.json")

	orig := &UserConfig{
		Provider:  "zai",
		ZAIAPIKey: "key-123",
		Model:     "glm-4.7",
		Theme:     "light",
	}
	if err := orig.Save(path); err != nil {
		t.Fatalf("Save error: %v", err)
	}

	loaded, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig error: %v", err)
	}
	if loaded.Provider != "zai" {
		t.Errorf("Provider = %q, want 'zai'", loaded.Provider)
	}
	if loaded.ZAIAPIKey != "key-123" {
		t.Errorf("ZAIAPIKey = %q, want 'key-123'", loaded.ZAIAPIKey)
	}
}

// =============================================================================
// UserConfig Tests - Context Window Defaults
// =============================================================================

func TestGetContextWindowConfig_WhenNil_ShouldReturnDefaults(t *testing.T) {
	cfg := &UserConfig{}
	cwCfg := cfg.GetContextWindowConfig()
	if cwCfg.MaxTokens == 0 {
		t.Error("MaxTokens should have a non-zero default")
	}
	if cwCfg.CoreReservePercent == 0 {
		t.Error("CoreReservePercent should have a non-zero default")
	}
}

func TestGetContextWindowConfig_WhenPartial_ShouldFillDefaults(t *testing.T) {
	cfg := &UserConfig{
		ContextWindow: &ContextWindowConfig{MaxTokens: 50000},
	}
	cwCfg := cfg.GetContextWindowConfig()
	if cwCfg.MaxTokens != 50000 {
		t.Errorf("MaxTokens = %d, want 50000", cwCfg.MaxTokens)
	}
	if cwCfg.CoreReservePercent == 0 {
		t.Error("CoreReservePercent should be filled from defaults")
	}
}

// =============================================================================
// UserConfig Tests - Embedding Config Defaults
// =============================================================================

func TestGetEmbeddingConfig_WhenNil_ShouldReturnDefaults(t *testing.T) {
	cfg := &UserConfig{}
	embCfg := cfg.GetEmbeddingConfig()
	if embCfg.Provider != "ollama" {
		t.Errorf("Provider = %q, want 'ollama'", embCfg.Provider)
	}
	if embCfg.OllamaEndpoint != "http://localhost:11434" {
		t.Errorf("OllamaEndpoint = %q, want default", embCfg.OllamaEndpoint)
	}
}

func TestGetEmbeddingConfig_WhenPartial_ShouldFillDefaults(t *testing.T) {
	cfg := &UserConfig{
		Embedding: &EmbeddingConfig{Provider: "genai"},
	}
	embCfg := cfg.GetEmbeddingConfig()
	if embCfg.Provider != "genai" {
		t.Errorf("Provider = %q, want 'genai'", embCfg.Provider)
	}
	if embCfg.OllamaModel != "embeddinggemma:300m" {
		t.Errorf("OllamaModel should default, got %q", embCfg.OllamaModel)
	}
}

// =============================================================================
// UserConfig Tests - Core Limits Defaults
// =============================================================================

func TestGetCoreLimits_WhenNil_ShouldReturnDefaults(t *testing.T) {
	cfg := &UserConfig{}
	limits := cfg.GetCoreLimits()
	if limits.MaxTotalMemoryMB != 12288 {
		t.Errorf("MaxTotalMemoryMB = %d, want 12288", limits.MaxTotalMemoryMB)
	}
	if limits.MaxConcurrentShards != 12 {
		t.Errorf("MaxConcurrentShards = %d, want 12", limits.MaxConcurrentShards)
	}
}

func TestGetCoreLimits_WhenPartial_ShouldFillDefaults(t *testing.T) {
	cfg := &UserConfig{
		CoreLimits: &CoreLimits{MaxConcurrentShards: 8},
	}
	limits := cfg.GetCoreLimits()
	if limits.MaxConcurrentShards != 8 {
		t.Errorf("MaxConcurrentShards = %d, want 8", limits.MaxConcurrentShards)
	}
	if limits.MaxTotalMemoryMB != 12288 {
		t.Errorf("MaxTotalMemoryMB should default to 12288, got %d", limits.MaxTotalMemoryMB)
	}
}

// =============================================================================
// UserConfig Tests - Learning Candidate Config
// =============================================================================

func TestGetLearningCandidateThreshold_WhenZero_ShouldDefaultTo3(t *testing.T) {
	cfg := &UserConfig{}
	if got := cfg.GetLearningCandidateThreshold(); got != 3 {
		t.Errorf("threshold = %d, want 3", got)
	}
}

func TestGetLearningCandidateThreshold_WhenSet_ShouldReturnValue(t *testing.T) {
	cfg := &UserConfig{LearningCandidateThreshold: 5}
	if got := cfg.GetLearningCandidateThreshold(); got != 5 {
		t.Errorf("threshold = %d, want 5", got)
	}
}

func TestGetLearningCandidateAutoPromote_WhenNilConfig_ShouldReturnFalse(t *testing.T) {
	var cfg *UserConfig
	if got := cfg.GetLearningCandidateAutoPromote(); got != false {
		t.Errorf("auto_promote = %v, want false", got)
	}
}

// =============================================================================
// UserConfig Tests - Context7 API Key
// =============================================================================

func TestGetContext7APIKey_WhenEnvSet_ShouldPreferEnv(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "env-key")
	cfg := &UserConfig{Context7APIKey: "file-key"}
	if got := cfg.GetContext7APIKey(); got != "env-key" {
		t.Errorf("GetContext7APIKey() = %q, want 'env-key'", got)
	}
}

func TestGetContext7APIKey_WhenEnvEmpty_ShouldUseConfig(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "")
	cfg := &UserConfig{Context7APIKey: "file-key"}
	if got := cfg.GetContext7APIKey(); got != "file-key" {
		t.Errorf("GetContext7APIKey() = %q, want 'file-key'", got)
	}
}

func TestGetContext7APIKey_WhenBothEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "")
	cfg := &UserConfig{}
	if got := cfg.GetContext7APIKey(); got != "" {
		t.Errorf("GetContext7APIKey() = %q, want empty", got)
	}
}

func TestGetContext7APIKey_WhenNilConfig_ShouldReturnEmpty(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "")
	var cfg *UserConfig
	if got := cfg.GetContext7APIKey(); got != "" {
		t.Errorf("GetContext7APIKey() = %q, want empty", got)
	}
}

// =============================================================================
// DefaultUserConfig Tests
// =============================================================================

func TestDefaultUserConfig_ShouldHaveSensibleDefaults(t *testing.T) {
	cfg := DefaultUserConfig()
	if cfg.Provider != "zai" {
		t.Errorf("Provider = %q, want 'zai'", cfg.Provider)
	}
	if cfg.Model != "glm-4.7" {
		t.Errorf("Model = %q, want 'glm-4.7'", cfg.Model)
	}
	if cfg.Theme != "light" {
		t.Errorf("Theme = %q, want 'light'", cfg.Theme)
	}
}
