package security

import (
	"os"
	"path/filepath"
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
}
