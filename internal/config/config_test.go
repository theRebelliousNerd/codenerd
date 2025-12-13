package config

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFindWorkspaceRoot_PrefersNerdDir(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir .nerd: %v", err)
	}
	nested := filepath.Join(root, "a", "b", "c")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	got, err := FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("FindWorkspaceRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindWorkspaceRoot=%q, want %q", got, root)
	}
}

func TestFindWorkspaceRoot_FallsBackToGoMod(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/test\n\ngo 1.22\n"), 0o644); err != nil {
		t.Fatalf("write go.mod: %v", err)
	}
	nested := filepath.Join(root, "subdir")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	got, err := FindWorkspaceRoot()
	if err != nil {
		t.Fatalf("FindWorkspaceRoot: %v", err)
	}
	if got != root {
		t.Fatalf("FindWorkspaceRoot=%q, want %q", got, root)
	}
}

func TestDefaultUserConfigPath_UsesWorkspaceRoot(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0o755); err != nil {
		t.Fatalf("mkdir .nerd: %v", err)
	}
	nested := filepath.Join(root, "x", "y")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir nested: %v", err)
	}

	origWD, _ := os.Getwd()
	if err := os.Chdir(nested); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(origWD) })

	got := DefaultUserConfigPath()
	want := filepath.Join(root, ".nerd", "config.json")
	if got != want {
		t.Fatalf("DefaultUserConfigPath=%q, want %q", got, want)
	}
}

func TestUserConfig_GetActiveProvider_PriorityAndLegacy(t *testing.T) {
	cfg := &UserConfig{
		Provider:        "openai",
		OpenAIAPIKey:    "k-openai",
		AnthropicAPIKey: "k-anthropic",
	}
	provider, key := cfg.GetActiveProvider()
	if provider != "openai" || key != "k-openai" {
		t.Fatalf("GetActiveProvider=%q/%q, want openai/k-openai", provider, key)
	}

	legacy := &UserConfig{APIKey: "k-legacy"}
	provider, key = legacy.GetActiveProvider()
	if provider != "zai" || key != "k-legacy" {
		t.Fatalf("GetActiveProvider legacy=%q/%q, want zai/k-legacy", provider, key)
	}
}

func TestUserConfig_SetEngine_Validates(t *testing.T) {
	cfg := &UserConfig{}
	if err := cfg.SetEngine("not-a-real-engine"); err == nil {
		t.Fatalf("expected invalid engine to error")
	}
	if err := cfg.SetEngine("codex-cli"); err != nil {
		t.Fatalf("SetEngine(codex-cli) error: %v", err)
	}
	if got := cfg.GetEngine(); got != "codex-cli" {
		t.Fatalf("GetEngine=%q, want codex-cli", got)
	}
}

func TestUserConfig_GetContext7APIKey_EnvOverridesConfig(t *testing.T) {
	t.Setenv("CONTEXT7_API_KEY", "env-key")
	cfg := &UserConfig{Context7APIKey: "file-key"}
	if got := cfg.GetContext7APIKey(); got != "env-key" {
		t.Fatalf("GetContext7APIKey=%q, want env-key", got)
	}
}

func TestLoadUserConfig_SaveRoundTrip(t *testing.T) {
	root := t.TempDir()
	path := filepath.Join(root, ".nerd", "config.json")

	cfg := &UserConfig{
		Provider:       "zai",
		Model:          "glm-4.6",
		ZAIAPIKey:      "k-zai",
		Theme:          "dark",
		Context7APIKey: "ctx7",
	}
	if err := cfg.Save(path); err != nil {
		t.Fatalf("Save: %v", err)
	}

	loaded, err := LoadUserConfig(path)
	if err != nil {
		t.Fatalf("LoadUserConfig: %v", err)
	}
	if loaded.Provider != cfg.Provider || loaded.Model != cfg.Model || loaded.ZAIAPIKey != cfg.ZAIAPIKey || loaded.Theme != cfg.Theme {
		t.Fatalf("round-trip mismatch: got=%+v want=%+v", loaded, cfg)
	}
}
