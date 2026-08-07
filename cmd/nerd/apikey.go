package main

import (
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/config"
)

// providerEnvVar maps a provider to the environment variable that holds its key.
// Mirrors the wizard's table in cmd/nerd/chat/config_wizard.go.
var providerEnvVar = map[string]string{
	"anthropic":  "ANTHROPIC_API_KEY",
	"openai":     "OPENAI_API_KEY",
	"gemini":     "GEMINI_API_KEY",
	"xai":        "XAI_API_KEY",
	"zai":        "ZAI_API_KEY",
	"openrouter": "OPENROUTER_API_KEY",
	"dashscope":  "DASHSCOPE_API_KEY",
	"meta":       "META_API_KEY",
	"moonshot":   "MOONSHOT_API_KEY",
}

// resolveAPIKey returns the key for the CONFIGURED provider, in precedence
// order: the --api-key flag, then .nerd/config.json, then that provider's
// environment variable.
//
// Every CLI command used to do this instead:
//
//	key := apiKey
//	if key == "" {
//	    key = os.Getenv("ZAI_API_KEY")
//	}
//
// — 26 copies, all naming Z.AI regardless of what the user configured. That is
// the same env-over-config confusion that produced F-INIT-1, where a stale
// ambient ZAI_API_KEY made an entire cold start run against an unconfigured
// provider: 195 of 196 LLM calls failed and the summary still printed
// "All 10 knowledge bases validated / Quality 80%".
//
// Config is boss. The env var consulted is the one belonging to the provider
// the user actually chose, and no other provider's key is ever substituted —
// a silent cross-provider fallback is how you end up billing the wrong vendor
// and debugging the wrong client.
//
// Returns "" when nothing resolves. That is not fatal here: BootCortex does its
// own config-driven client construction, and an empty key simply means this
// command contributes no override.
func resolveAPIKey(flagValue, workspace string) string {
	if strings.TrimSpace(flagValue) != "" {
		return flagValue
	}

	// LoadUserConfig takes the config FILE path, not the workspace directory.
	// Passing the directory makes os.ReadFile fail with "is a directory", which
	// is not os.IsNotExist, so it returns an error and every lookup silently
	// fell through to the legacy variable — reintroducing the exact bug this
	// function exists to fix.
	ws := workspace
	if strings.TrimSpace(ws) == "" {
		if cwd, err := os.Getwd(); err == nil {
			ws = cwd
		}
	}

	cfg, err := config.LoadUserConfig(filepath.Join(ws, ".nerd", "config.json"))
	if err != nil || cfg == nil {
		// An unreadable or malformed config is not a licence to guess a
		// provider. Return nothing and let BootCortex report the config error
		// properly, rather than papering over it with a key for a vendor the
		// user may not use.
		return ""
	}

	provider, key := cfg.GetActiveProvider()
	if strings.TrimSpace(key) != "" {
		return key
	}
	if envName, ok := providerEnvVar[provider]; ok {
		return os.Getenv(envName)
	}

	// provider == "" means the config named no provider and held no key —
	// LoadUserConfig returns an empty struct rather than an error when the file
	// is absent, so this covers both "no config" and "empty config". There is
	// nothing to be specific about, so the historical variable is the only
	// sensible answer and an existing setup that relied on it keeps working.
	// This is the ONLY path that may still consult ZAI_API_KEY.
	return os.Getenv("ZAI_API_KEY")
}
