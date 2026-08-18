package security

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPathPolicyConfinesBrowserWrites(t *testing.T) {
	workspace := t.TempDir()
	policy, err := NewPathPolicy(workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(workspace, ".nerd", "browser", "screenshots", "shot.png")
	got, err := policy.ResolveForWrite("shot.png", policy.DefaultRoot(), "default.png")
	if err != nil {
		t.Fatalf("ResolveForWrite(valid) error = %v", err)
	}
	if got != want {
		t.Fatalf("ResolveForWrite(valid) = %q, want %q", got, want)
	}
	if _, err := policy.ResolveForWrite(filepath.Join(workspace, "outside.txt"), "", ""); err == nil {
		t.Fatal("path policy accepted an output outside writable roots")
	}
}

func TestPathPolicyDefaultRootsIncludeSnapshots(t *testing.T) {
	workspace := t.TempDir()
	policy, err := NewPathPolicy(workspace, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"screenshots", "traces", "snapshots"} {
		path := filepath.Join(workspace, ".nerd", "browser", dir, "artifact.mg")
		if _, err := policy.ResolveForWrite(path, "", ""); err != nil {
			t.Fatalf("default policy should allow %s (%q): %v", dir, path, err)
		}
	}
	rejected := filepath.Join(workspace, ".nerd", "browser", "elsewhere", "artifact.mg")
	if _, err := policy.ResolveForWrite(rejected, "", ""); err == nil {
		t.Fatalf("default policy should reject path outside writable roots: %q", rejected)
	}
}

func TestPathPolicyRejectsExistingSymlinkEscape(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "artifacts")
	outside := t.TempDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	policy, err := NewPathPolicy(workspace, []string{root})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := policy.ResolveForWrite(filepath.Join(link, "evidence.json"), "", ""); err == nil {
		t.Fatal("path policy accepted a symlink escape")
	}
}

func TestPrivateBrowserArtifactPermissions(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := EnsurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "artifact.txt")
	if err := WritePrivateFile(path, []byte("evidence")); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "evidence" {
		t.Fatalf("private artifact read = %q, %v", data, err)
	}
	privateDir, err := IsPrivatePath(dir, true)
	if err != nil || !privateDir {
		t.Fatalf("private directory policy = %v, %v", privateDir, err)
	}
	privateFile, err := IsPrivatePath(path, false)
	if err != nil || !privateFile {
		t.Fatalf("private file policy = %v, %v", privateFile, err)
	}
}

func TestWritePrivateFileExclusiveRefusesOverwrite(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "private")
	if err := EnsurePrivateDir(dir); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "artifact.txt")
	if err := WritePrivateFileExclusive(path, []byte("first")); err != nil {
		t.Fatal(err)
	}
	if err := WritePrivateFileExclusive(path, []byte("second")); err == nil {
		t.Fatal("exclusive private write overwrote an existing artifact")
	}
	data, err := os.ReadFile(path)
	if err != nil || string(data) != "first" {
		t.Fatalf("exclusive artifact = %q, %v", data, err)
	}
}

func TestConfineToRoot_WhenInsideRoot_ShouldResolve(t *testing.T) {
	root := t.TempDir()
	sub := filepath.Join(root, "sub")
	if err := os.MkdirAll(sub, 0o700); err != nil {
		t.Fatal(err)
	}
	candidate := filepath.Join(sub, "file.txt")
	got, err := ConfineToRoot(root, candidate)
	if err != nil {
		t.Fatalf("ConfineToRoot inside root error = %v", err)
	}
	if got == "" {
		t.Fatal("ConfineToRoot inside root returned empty path")
	}
	// Resolved path should remain inside root and be absolute.
	if !filepath.IsAbs(got) {
		t.Fatalf("ConfineToRoot returned non-absolute path %q", got)
	}
	// Verify containment using the same helper semantics (via Rel check).
	if rel, _ := filepath.Rel(root, got); rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.IsAbs(rel) {
		t.Fatalf("ConfineToRoot returned path outside root: %q", got)
	}
}

func TestConfineToRoot_WhenRootItself_ShouldResolve(t *testing.T) {
	root := t.TempDir()
	got, err := ConfineToRoot(root, root)
	if err != nil {
		t.Fatalf("ConfineToRoot root itself error = %v", err)
	}
	if got == "" {
		t.Fatal("ConfineToRoot root itself returned empty path")
	}
	if !filepath.IsAbs(got) {
		t.Fatalf("ConfineToRoot root itself returned non-absolute %q", got)
	}
}

func TestConfineToRoot_WhenOutsideRoot_ShouldReject(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	candidate := filepath.Join(outside, "file.txt")
	if _, err := ConfineToRoot(root, candidate); err == nil {
		t.Fatal("ConfineToRoot accepted a sibling directory outside root")
	}
}

func TestConfineToRoot_WhenParentTraversal_ShouldReject(t *testing.T) {
	root := t.TempDir()
	candidate := filepath.Join(root, "..", "evil.txt")
	if _, err := ConfineToRoot(root, candidate); err == nil {
		t.Fatal("ConfineToRoot accepted parent traversal outside root")
	}
}

func TestConfineToRoot_WhenSymlinkEscapes_ShouldReject(t *testing.T) {
	workspace := t.TempDir()
	root := filepath.Join(workspace, "repo")
	outside := t.TempDir()
	if err := os.MkdirAll(root, 0o700); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(root, "escape")
	if err := os.Symlink(outside, link); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	candidate := filepath.Join(link, "secret.txt")
	if _, err := ConfineToRoot(root, candidate); err == nil {
		t.Fatal("ConfineToRoot accepted a symlink escape")
	}
}

func TestConfineToRoot_WhenEmptyArguments_ShouldError(t *testing.T) {
	root := t.TempDir()
	candidate := filepath.Join(root, "file.txt")
	if _, err := ConfineToRoot("", candidate); err == nil {
		t.Fatal("ConfineToRoot accepted empty root")
	}
	if _, err := ConfineToRoot("   ", candidate); err == nil {
		t.Fatal("ConfineToRoot accepted whitespace root")
	}
	if _, err := ConfineToRoot(root, ""); err == nil {
		t.Fatal("ConfineToRoot accepted empty candidate")
	}
	if _, err := ConfineToRoot(root, "   "); err == nil {
		t.Fatal("ConfineToRoot accepted whitespace candidate")
	}
}

func TestConfineToRoot_ShouldNotLeakOutsidePathInError(t *testing.T) {
	root := t.TempDir()
	outside := t.TempDir()
	candidate := filepath.Join(outside, "secret.txt")
	_, err := ConfineToRoot(root, candidate)
	if err == nil {
		t.Fatal("ConfineToRoot accepted outside path")
	}
	if strings.Contains(err.Error(), outside) {
		t.Fatalf("error leaks outside path %q in %q", outside, err.Error())
	}
	if strings.Contains(err.Error(), candidate) {
		t.Fatalf("error leaks outside candidate %q in %q", candidate, err.Error())
	}
}

func TestProtectPrivateFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "protect_me.txt")

	// Create a file with open permissions.
	if err := os.WriteFile(path, []byte("content"), 0o644); err != nil {
		t.Fatal(err)
	}

	if err := ProtectPrivateFile(path); err != nil {
		t.Fatalf("ProtectPrivateFile failed: %v", err)
	}

	isPrivate, err := IsPrivatePath(path, false)
	if err != nil {
		t.Fatalf("IsPrivatePath failed: %v", err)
	}
	if !isPrivate {
		t.Fatal("File is not private after ProtectPrivateFile")
	}
}

func TestProtectPrivateFile_NonExistent(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "nonexistent.txt")

	if err := ProtectPrivateFile(path); err == nil {
		t.Fatal("ProtectPrivateFile on nonexistent file should return an error")
	}
}
