package tools

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The bug class these pin: a containment gate that normalizes with a
// separator-aware helper. filepath.ToSlash and filepath.Clean are no-ops on
// backslashes off Windows, so a Windows-shaped path walks straight through a
// gate that believes it has normalized. That is not hypothetical in this
// repo — `.nerd\config.json` passed the nerd.md write-protection gate on Linux
// for exactly that reason.

func TestResolveWorkspacePath_WhenBackslashTraversal_ShouldRefuse(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	if err := os.MkdirAll(root, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	for _, p := range []string{
		`..\escape.txt`,
		`..\..\etc\passwd`,
		`sub\..\..\escape.txt`,
		`.nerd\config.json\..\..\..\escape`,
	} {
		got, err := ResolveWorkspacePath(context.Background(), root, p)
		if err == nil {
			t.Fatalf("backslash traversal %q accepted, resolved to %q", p, got)
		}
		if !errors.Is(err, ErrPathOutsideWorkspace) {
			t.Fatalf("expected ErrPathOutsideWorkspace for %q, got %v", p, err)
		}
	}
}

func TestResolveWorkspacePath_WhenWindowsAbsolutePathOnPosix_ShouldRefuse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("the smuggling shape being tested only exists off Windows")
	}
	t.Parallel()
	root := t.TempDir()

	// `C:\Users\victim\.ssh\id_rsa` is not absolute to a POSIX filepath, so
	// without separator normalization it was joined onto the root as a single
	// filename and accepted.
	got, err := ResolveWorkspacePath(context.Background(), root, `C:\Users\victim\.ssh\id_rsa`)
	if err == nil {
		// It may resolve inside the root as C:/Users/... which is contained and
		// therefore harmless, but it must never keep the raw backslash form.
		if strings.Contains(got, `\`) {
			t.Fatalf("path kept unnormalized backslashes: %q", got)
		}
	}
}

func TestResolveWorkspacePath_WhenSiblingSharesRootPrefix_ShouldRefuse(t *testing.T) {
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	sibling := filepath.Join(base, "ws-evil")
	for _, d := range []string{root, sibling} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	victim := filepath.Join(sibling, "secret.txt")
	if err := os.WriteFile(victim, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}

	if got, err := ResolveWorkspacePath(context.Background(), root, victim); err == nil {
		t.Fatalf("/ws-evil accepted as inside /ws, resolved to %q", got)
	}
}

func TestResolveWorkspacePath_WhenSymlinkEscapesRoot_ShouldRefuse(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("symlink creation requires elevation on Windows")
	}
	t.Parallel()
	base := t.TempDir()
	root := filepath.Join(base, "ws")
	outside := filepath.Join(base, "outside")
	for _, d := range []string{root, outside} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatalf("mkdir: %v", err)
		}
	}
	victim := filepath.Join(outside, "secret.txt")
	if err := os.WriteFile(victim, []byte("x"), 0o600); err != nil {
		t.Fatalf("write: %v", err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "link")); err != nil {
		t.Skipf("symlink unsupported: %v", err)
	}

	// Existing target through a link.
	if got, err := ResolveWorkspacePath(context.Background(), root, "link/secret.txt"); err == nil {
		t.Fatalf("symlinked read escaped: %q", got)
	}
	// Not-yet-existing target through a link — the write case, which resolves
	// the closest existing ancestor instead.
	if got, err := ResolveWorkspacePath(context.Background(), root, "link/new.txt"); err == nil {
		t.Fatalf("symlinked write escaped: %q", got)
	}
}

func TestResolveWorkspacePath_WhenPathIsInsideRoot_ShouldAccept(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	nested := filepath.Join(root, "a", "b")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	for _, p := range []string{
		"a/b/file.go",
		`a\b\file.go`, // Windows-shaped but contained: normalized, then accepted
		"a/../a/b/file.go",
		filepath.Join(root, "a", "b", "file.go"),
	} {
		got, err := ResolveWorkspacePath(context.Background(), root, p)
		if err != nil {
			t.Fatalf("legitimate in-workspace path %q rejected: %v", p, err)
		}
		if !containedIn(mustEval(t, root), got) {
			t.Fatalf("resolved %q to %q, which is not inside %q", p, got, root)
		}
	}
}

func TestContainedIn_WhenPathSharesNamePrefix_ShouldReturnFalse(t *testing.T) {
	t.Parallel()
	sep := string(filepath.Separator)
	root := sep + "ws"

	cases := map[string]bool{
		sep + "ws":                  true,
		sep + "ws" + sep:            true,
		sep + "ws" + sep + "a":      true,
		sep + "ws-evil":             false,
		sep + "ws-evil" + sep + "a": false,
		sep + "wsx":                 false,
		sep + "other":               false,
	}
	for path, want := range cases {
		if got := containedIn(root, path); got != want {
			t.Errorf("containedIn(%q, %q) = %v, want %v", root, path, got, want)
		}
	}
}

func TestResolveWorkspaceDir_WhenEmptyOrDot_ShouldReturnRoot(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	want := mustEval(t, root)

	for _, p := range []string{"", ".", "./", "   "} {
		got, err := ResolveWorkspaceDir(context.Background(), root, p)
		if err != nil {
			t.Fatalf("ResolveWorkspaceDir(%q): %v", p, err)
		}
		if got != want {
			t.Fatalf("ResolveWorkspaceDir(%q) = %q, want %q", p, got, want)
		}
	}
}

func TestWorkspaceRoot_WhenContextSet_ShouldOutrankEnv(t *testing.T) {
	envRoot := t.TempDir()
	ctxRoot := t.TempDir()
	t.Setenv("CODENERD_WORKSPACE_ROOT", envRoot)

	got, err := WorkspaceRoot(WithWorkspaceRoot(context.Background(), ctxRoot))
	if err != nil {
		t.Fatalf("WorkspaceRoot: %v", err)
	}
	if got != ctxRoot {
		t.Fatalf("context root ignored: got %q want %q", got, ctxRoot)
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	resolved, err := filepath.EvalSymlinks(p)
	if err != nil {
		return filepath.Clean(p)
	}
	return resolved
}
