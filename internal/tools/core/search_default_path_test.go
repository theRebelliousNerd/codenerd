package core

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools"
)

// TestExecuteGrep_DefaultPath_SkipsHiddenButFindsVisible verifies a default
// path of "." still walks the workspace. Dot directories are members;
// node_modules is a default exclusion and must not be searched.
func TestExecuteGrep_DefaultPath_SkipsHiddenButFindsVisible(t *testing.T) {
	// Do not run in parallel: this test pins process-global state.
	tmpDir := t.TempDir()

	// Built at runtime so the full token never appears verbatim in this source
	// file. A verbatim literal would make any search whose root includes the
	// package directory legitimately match this file itself.
	visibleToken := "UNIQUE_VISIBLE_" + "TOKEN_abc123_789"
	hiddenToken := "UNIQUE_HIDDEN_" + "TOKEN_xyz789_012"
	depToken := "UNIQUE_DEP_" + "TOKEN_dep000_456"

	visibleFile := filepath.Join(tmpDir, "visible.txt")
	if err := os.WriteFile(visibleFile, []byte("hello "+visibleToken+" world\n"), 0600); err != nil {
		t.Fatalf("write visible file: %v", err)
	}

	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.Mkdir(hiddenDir, 0755); err != nil {
		t.Fatalf("mkdir .hidden: %v", err)
	}
	hiddenFile := filepath.Join(hiddenDir, "hidden.txt")
	if err := os.WriteFile(hiddenFile, []byte("secret "+hiddenToken+" inside hidden\n"), 0600); err != nil {
		t.Fatalf("write hidden file: %v", err)
	}
	depDir := filepath.Join(tmpDir, "node_modules")
	if err := os.Mkdir(depDir, 0755); err != nil {
		t.Fatalf("mkdir node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(depDir, "lib.txt"), []byte(depToken+"\n"), 0600); err != nil {
		t.Fatalf("write dep file: %v", err)
	}

	// Pin the default search root to the temp workspace regardless of test
	// order. executeGrep resolves an omitted path via tools.WorkspaceRoot,
	// which prefers context, then CODENERD_WORKSPACE_ROOT, then cwd. Pinning
	// all three to tmpDir keeps an earlier test's env or cwd from moving the
	// root back to the repository (where the runtime-built tokens are the only
	// thing stopping a self-match). No production change: containment already
	// lands inside the workspace; this only declares which workspace.
	t.Setenv("CODENERD_WORKSPACE_ROOT", tmpDir)
	t.Chdir(tmpDir)
	ctx := tools.WithWorkspaceRoot(context.Background(), tmpDir)

	// Search with NO path argument (defaults to the workspace root) for the
	// visible token. Must be found.
	resultVisible, err := executeGrep(ctx, map[string]any{
		"pattern": visibleToken,
	})
	if err != nil {
		t.Fatalf("executeGrep visible (default path): %v", err)
	}
	if !strings.Contains(resultVisible, visibleToken) {
		t.Errorf("expected visible token %q to be found via default path, got %q", visibleToken, resultVisible)
	}
	resultHidden, err := executeGrep(ctx, map[string]any{
		"pattern": hiddenToken,
	})
	if err != nil {
		t.Fatalf("executeGrep hidden (default path): %v", err)
	}
	if !strings.Contains(resultHidden, hiddenToken) {
		t.Errorf("expected .hidden token to be found, got %q", resultHidden)
	}

	resultDep, err := executeGrep(ctx, map[string]any{
		"pattern": depToken,
	})
	if err != nil {
		t.Fatalf("executeGrep node_modules (default path): %v", err)
	}
	if !strings.Contains(resultDep, "No matches found") {
		t.Errorf("expected node_modules token NOT to be found, got %q", resultDep)
	}
}
