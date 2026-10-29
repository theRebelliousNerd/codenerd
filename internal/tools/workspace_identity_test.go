package tools

import (
	"os"
	"path/filepath"
	"testing"
)

func TestWorkspaceIdentityThroughAlias(t *testing.T) {
	realRoot := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(realRoot, alias); err != nil {
		t.Skipf("symlink unavailable: %v", err)
	}
	ctx := WithWorkspaceRoot(t.Context(), alias)
	root, err := WorkspaceRoot(ctx)
	if err != nil {
		t.Fatal(err)
	}
	path, err := ResolveWorkspacePath(ctx, root, "new/file.go")
	if err != nil {
		t.Fatal(err)
	}
	rel, err := filepath.Rel(root, path)
	if err != nil || rel != filepath.Join("new", "file.go") {
		t.Fatalf("root/target identity mismatch: %q %q: %v", root, path, err)
	}
	if _, err := ResolveWorkspacePath(ctx, root, "../escape"); err == nil {
		t.Fatal("containment weakened")
	}
}
