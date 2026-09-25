package system

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCortexKey(t *testing.T) {
	base := cortexKey("/ws", "api", "zai", "key123", "glm-4", []string{"tactile_router"})
	if len(base) != 64 {
		t.Fatalf("cortexKey length=%d, want 64 (hex sha256)", len(base))
	}
	// Deterministic: identical inputs hash identically.
	if again := cortexKey("/ws", "api", "zai", "key123", "glm-4", []string{"tactile_router"}); again != base {
		t.Error("cortexKey is not deterministic for identical inputs")
	}
	// Each component independently changes the key (no field collisions).
	variants := map[string]string{
		"workspace":      cortexKey("/other", "api", "zai", "key123", "glm-4", []string{"tactile_router"}),
		"provider":       cortexKey("/ws", "api", "gemini", "key123", "glm-4", []string{"tactile_router"}),
		"apiKey":         cortexKey("/ws", "api", "zai", "different", "glm-4", []string{"tactile_router"}),
		"model":          cortexKey("/ws", "api", "zai", "key123", "gpt", []string{"tactile_router"}),
		"disabledShards": cortexKey("/ws", "api", "zai", "key123", "glm-4", []string{"campaign_runner"}),
		"engine":         cortexKey("/ws", "claude-cli", "zai", "key123", "glm-4", []string{"tactile_router"}),
	}
	for field, k := range variants {
		if k == base {
			t.Errorf("changing %s did not change the cortex key", field)
		}
	}
	if strings.Contains(base, "key123") {
		t.Fatal("cortexKey leaked API key material")
	}
}

func TestCortexKeyNormalizesDisabledShardSet(t *testing.T) {
	want := cortexKey("/ws", "api", "zai", "secret", "glm-4", []string{"campaign_runner", "tactile_router"})
	got := cortexKey("/ws", "api", "zai", "secret", "glm-4", []string{
		" tactile_router ",
		"campaign_runner",
		"tactile_router",
		"",
	})
	if got != want {
		t.Fatalf("order, duplicates, or whitespace changed disabled-shard identity: got %s want %s", got, want)
	}
}

func TestResolveWorkspaceRoot(t *testing.T) {
	// Clear first so resolveWorkspaceRoot's Setenv is cleaned up by testing.
	t.Setenv("CODENERD_WORKSPACE_ROOT", "")

	// Use a real temp dir so filepath.Abs is stable across Windows/Unix.
	dir := t.TempDir()
	want, err := filepath.EvalSymlinks(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got := resolveWorkspaceRoot(dir); got != want {
		t.Errorf("resolveWorkspaceRoot(explicit)=%q, want %q", got, want)
	}
	if env := os.Getenv("CODENERD_WORKSPACE_ROOT"); env != want {
		t.Errorf("CODENERD_WORKSPACE_ROOT=%q, want %q", env, want)
	}

	// Empty workspace must resolve to *some* concrete directory (found root or cwd).
	if got := resolveWorkspaceRoot(""); got == "" {
		t.Error("resolveWorkspaceRoot(\"\") should fall back to a non-empty directory")
	}
}

func TestResolveProviderModelForKey(t *testing.T) {
	// Workspace with a config.json yields its provider/model.
	dir := t.TempDir()
	nerd := filepath.Join(dir, ".nerd")
	if err := os.MkdirAll(nerd, 0o755); err != nil {
		t.Fatal(err)
	}
	cfgJSON := `{"provider":"zai","model":"glm-4.6"}`
	if err := os.WriteFile(filepath.Join(nerd, "config.json"), []byte(cfgJSON), 0o644); err != nil {
		t.Fatal(err)
	}
	engine, provider, model := resolveProviderModelForKey(dir)
	if engine != "api" || provider != "zai" || model != "glm-4.6" {
		t.Errorf("resolveProviderModelForKey=(%q,%q,%q), want (api, zai, glm-4.6)", engine, provider, model)
	}

	// Workspace without a config: the default engine, no provider/model.
	empty := t.TempDir()
	if e, p, m := resolveProviderModelForKey(empty); e != "api" || p != "" || m != "" {
		t.Errorf("missing config should yield (api, \"\", \"\"), got (%q,%q,%q)", e, p, m)
	}
}
