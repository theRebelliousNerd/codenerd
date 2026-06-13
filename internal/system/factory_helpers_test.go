package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestCortexKey(t *testing.T) {
	base := cortexKey("/ws", "zai", "key123", "glm-4")
	if len(base) != 64 {
		t.Fatalf("cortexKey length=%d, want 64 (hex sha256)", len(base))
	}
	// Deterministic: identical inputs hash identically.
	if again := cortexKey("/ws", "zai", "key123", "glm-4"); again != base {
		t.Error("cortexKey is not deterministic for identical inputs")
	}
	// Each component independently changes the key (no field collisions).
	variants := map[string]string{
		"workspace": cortexKey("/other", "zai", "key123", "glm-4"),
		"provider":  cortexKey("/ws", "gemini", "key123", "glm-4"),
		"apiKey":    cortexKey("/ws", "zai", "different", "glm-4"),
		"model":     cortexKey("/ws", "zai", "key123", "gpt"),
	}
	for field, k := range variants {
		if k == base {
			t.Errorf("changing %s did not change the cortex key", field)
		}
	}
}

func TestResolveWorkspaceRoot(t *testing.T) {
	if got := resolveWorkspaceRoot("/explicit/path"); got != "/explicit/path" {
		t.Errorf("resolveWorkspaceRoot(explicit)=%q, want /explicit/path", got)
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
	provider, model := resolveProviderModelForKey(dir)
	if provider != "zai" || model != "glm-4.6" {
		t.Errorf("resolveProviderModelForKey=(%q,%q), want (zai, glm-4.6)", provider, model)
	}

	// Workspace without a config falls back to empty strings (no error path).
	empty := t.TempDir()
	if p, m := resolveProviderModelForKey(empty); p != "" || m != "" {
		t.Errorf("missing config should yield empty provider/model, got (%q,%q)", p, m)
	}
}
