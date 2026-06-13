package perception

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestProviderKeyFieldName(t *testing.T) {
	cases := map[string]string{
		"anthropic":  "ANTHROPIC_API_KEY",
		"openai":     "OPENAI_API_KEY",
		"gemini":     "GEMINI_API_KEY",
		"xai":        "XAI_API_KEY",
		"zai":        "ZAI_API_KEY",
		"openrouter": "OPENROUTER_API_KEY",
	}
	for provider, wantEnv := range cases {
		got := providerKeyFieldName(provider)
		if !strings.Contains(got, wantEnv) {
			t.Errorf("providerKeyFieldName(%q)=%q, want it to mention %q", provider, got, wantEnv)
		}
	}
	// Unknown providers get a generic descriptor.
	if got := providerKeyFieldName("mystery"); !strings.Contains(got, "mystery") {
		t.Errorf("providerKeyFieldName(mystery)=%q, want it to mention the provider name", got)
	}
}

func TestLoadConfigJSON_OpenAIProvider(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	cfg := `{"provider":"openai","openai_api_key":"sk-test","model":"gpt-4o-mini"}`
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}

	pc, err := LoadConfigJSON(path)
	if err != nil {
		t.Fatalf("LoadConfigJSON: %v", err)
	}
	if pc.Provider != ProviderOpenAI {
		t.Errorf("Provider=%v, want %v", pc.Provider, ProviderOpenAI)
	}
	if pc.APIKey != "sk-test" {
		t.Errorf("APIKey=%q, want sk-test", pc.APIKey)
	}
	if pc.Model != "gpt-4o-mini" {
		t.Errorf("Model=%q, want gpt-4o-mini", pc.Model)
	}
}

func TestLoadConfigJSON_ProviderSetButKeyMissing(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "config.json")
	// Provider is explicitly anthropic but no anthropic key: config-is-boss
	// means an error, not a silent fallback to another provider's key.
	cfg := `{"provider":"anthropic","openai_api_key":"sk-other"}`
	if err := os.WriteFile(path, []byte(cfg), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadConfigJSON(path); err == nil {
		t.Error("expected an error when the configured provider's key is missing")
	}
}
