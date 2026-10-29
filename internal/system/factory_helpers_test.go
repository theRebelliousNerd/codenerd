package system

import (
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/config"
)

func TestCortexIdentityNormalizesDisabledShardSet(t *testing.T) {
	makeIdentity := func(disabled []string) cortexIdentity {
		t.Helper()
		identity, err := newCortexIdentity("/ws", &config.UserConfig{Engine: "api", Provider: "zai", Model: "glm-4"}, "secret", disabled)
		if err != nil {
			t.Fatal(err)
		}
		return identity
	}
	want := makeIdentity([]string{"campaign_runner", "tactile_router"}).cacheKey()
	got := makeIdentity([]string{
		" tactile_router ",
		"campaign_runner",
		"tactile_router",
		"",
	}).cacheKey()
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
