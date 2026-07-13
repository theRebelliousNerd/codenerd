package system

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolveWorkspaceRoot_SetsCodenerdWorkspaceEnv(t *testing.T) {
	t.Setenv("CODENERD_WORKSPACE_ROOT", "")
	dir := t.TempDir()
	got := resolveWorkspaceRoot(dir)
	abs, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got != abs {
		t.Fatalf("resolveWorkspaceRoot=%q want %q", got, abs)
	}
	if env := os.Getenv("CODENERD_WORKSPACE_ROOT"); env != abs {
		t.Fatalf("CODENERD_WORKSPACE_ROOT=%q want %q", env, abs)
	}
}
