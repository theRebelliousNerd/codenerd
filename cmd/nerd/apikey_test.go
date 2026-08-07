package main

import (
	"os"
	"path/filepath"
	"testing"
)

// Config is boss. Every CLI command used to fall back to ZAI_API_KEY no matter
// which provider the user had configured — 26 identical copies of
// `if key == "" { key = os.Getenv("ZAI_API_KEY") }`. That is the same
// env-over-config confusion as F-INIT-1, where a stale ambient Z.AI key made an
// entire cold start run against an unconfigured provider: 195 of 196 calls
// failed and the summary still reported success.

func writeConfig(t *testing.T, body string) string {
	t.Helper()
	dir := t.TempDir()
	nerdDir := filepath.Join(dir, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), []byte(body), 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}
	return dir
}

func TestResolveAPIKey_FlagWinsOverEverything(t *testing.T) {
	ws := writeConfig(t, `{"provider":"dashscope","dashscope_api_key":"from-config"}`)
	t.Setenv("DASHSCOPE_API_KEY", "from-env")

	if got := resolveAPIKey("from-flag", ws); got != "from-flag" {
		t.Errorf("resolveAPIKey = %q, want the explicit --api-key value", got)
	}
}

func TestResolveAPIKey_ConfigWinsOverEnv(t *testing.T) {
	ws := writeConfig(t, `{"provider":"dashscope","dashscope_api_key":"from-config"}`)
	t.Setenv("DASHSCOPE_API_KEY", "from-env")

	if got := resolveAPIKey("", ws); got != "from-config" {
		t.Errorf("resolveAPIKey = %q, want the configured key", got)
	}
}

// The env var consulted must belong to the CONFIGURED provider.
func TestResolveAPIKey_UsesTheConfiguredProvidersEnvVar(t *testing.T) {
	ws := writeConfig(t, `{"provider":"dashscope"}`)
	t.Setenv("DASHSCOPE_API_KEY", "dashscope-env")
	t.Setenv("ZAI_API_KEY", "stale-zai-key")

	got := resolveAPIKey("", ws)
	if got == "stale-zai-key" {
		t.Fatal("a stale ZAI_API_KEY was used for a dashscope project; this is exactly F-INIT-1")
	}
	if got != "dashscope-env" {
		t.Errorf("resolveAPIKey = %q, want %q", got, "dashscope-env")
	}
}

// No silent cross-provider substitution. Billing the wrong vendor and debugging
// the wrong client both start here.
func TestResolveAPIKey_NeverSubstitutesAnotherProvidersKey(t *testing.T) {
	ws := writeConfig(t, `{"provider":"meta"}`)
	t.Setenv("ZAI_API_KEY", "zai-key")
	t.Setenv("ANTHROPIC_API_KEY", "anthropic-key")
	os.Unsetenv("META_API_KEY")

	if got := resolveAPIKey("", ws); got != "" {
		t.Errorf("resolveAPIKey = %q; with no meta key available the answer must be empty, "+
			"not another provider's key", got)
	}
}

// A workspace with no config has no provider to be specific about, so the
// historical variable is the only sensible fallback — an existing setup that
// relied on it keeps working.
func TestResolveAPIKey_NoConfigFallsBackToHistoricalVar(t *testing.T) {
	t.Setenv("ZAI_API_KEY", "legacy-key")
	if got := resolveAPIKey("", t.TempDir()); got != "legacy-key" {
		t.Errorf("resolveAPIKey = %q, want the legacy fallback when there is no config", got)
	}
}

// Every provider GetActiveProvider can return must have an env var mapped, or
// that provider silently loses its environment fallback.
func TestProviderEnvVar_CoversEveryNamedProvider(t *testing.T) {
	for _, provider := range []string{
		"anthropic", "openai", "gemini", "xai", "zai",
		"openrouter", "dashscope", "meta", "moonshot",
	} {
		if _, ok := providerEnvVar[provider]; !ok {
			t.Errorf("provider %q has no environment variable mapped, so `nerd <cmd>` cannot pick up its key from the environment", provider)
		}
	}
}
