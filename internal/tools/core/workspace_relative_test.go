package core

import (
	"os"
	"path/filepath"
	"testing"
)

// A relative tool path must resolve against the workspace root, not the process
// working directory. filepath.Abs does the latter, and the two coincide only
// when the workspace happens to be the CWD — the default, but not the contract:
// -w/--workspace sets the root without chdir'ing. Before this was fixed, running
// codeNERD against any other directory made every relative path a model produced
// fail with "path escapes workspace root".
func TestResolveWorkspacePath_RelativeIsWorkspaceRelativeNotCWDRelative(t *testing.T) {
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}

	nested := filepath.Join(root, "internal", "projectdoc")
	if err := os.MkdirAll(nested, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	target := filepath.Join(nested, "nerdmd.go")
	if err := os.WriteFile(target, []byte("package projectdoc\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	// CWD is the package directory, which is emphatically not `root`.
	got, err := resolveWorkspacePath(nil, root, "internal/projectdoc/nerdmd.go")
	if err != nil {
		t.Fatalf("resolveWorkspacePath: %v", err)
	}
	if got != target {
		t.Errorf("resolved to %q, want %q", got, target)
	}
}

// Containment must still hold: workspace-relative resolution is not a licence
// to climb out with "..".
func TestResolveWorkspacePath_RelativeEscapeStillRejected(t *testing.T) {
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	if err := os.MkdirAll(filepath.Join(root, "sub"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	if _, err := resolveWorkspacePath(nil, root, filepath.Join("..", "outside.txt")); err == nil {
		t.Error("a relative path climbing above the workspace root was accepted")
	}
}

// Absolute paths inside the root keep working.
func TestResolveWorkspacePath_AbsoluteInsideRootAccepted(t *testing.T) {
	root := t.TempDir()
	if resolved, err := filepath.EvalSymlinks(root); err == nil {
		root = resolved
	}
	target := filepath.Join(root, "file.txt")
	if err := os.WriteFile(target, []byte("x"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	got, err := resolveWorkspacePath(nil, root, target)
	if err != nil {
		t.Fatalf("resolveWorkspacePath: %v", err)
	}
	if got != target {
		t.Errorf("resolved to %q, want %q", got, target)
	}
}
