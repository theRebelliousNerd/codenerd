package world

import (
	"os"
	"path/filepath"
	"testing"
)

// F-REC-1a: scanning a subdirectory must not write a .nerd/cache/manifest.json
// into the source tree. The manifest belongs to the workspace that owns the
// scan, resolved by walking up to the nearest ancestor with .nerd/config.json.

func writeOwnerProbeFile(t *testing.T, dir string) os.FileInfo {
	t.Helper()
	src := filepath.Join(dir, "probe.go")
	if err := os.WriteFile(src, []byte("package probe\n"), 0644); err != nil {
		t.Fatalf("Failed to write probe file: %v", err)
	}
	info, err := os.Stat(src)
	if err != nil {
		t.Fatalf("Failed to stat probe file: %v", err)
	}
	return info
}

func TestCacheOwner_NestedScanUsesWorkspaceManifest(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create workspace .nerd dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".nerd", "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write workspace config: %v", err)
	}
	nested := filepath.Join(workspace, "sub", "dir")
	if err := os.MkdirAll(nested, 0755); err != nil {
		t.Fatalf("Failed to create nested dir: %v", err)
	}

	cache := NewFileCache(nested)
	want := filepath.Join(workspace, ".nerd", "cache", "manifest.json")
	if cache.path != want {
		t.Fatalf("Expected cache path %s, got %s", want, cache.path)
	}

	info := writeOwnerProbeFile(t, nested)
	cache.Update(filepath.Join(nested, "probe.go"), info, "hash-nested")
	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(want); err != nil {
		t.Fatalf("Expected workspace manifest %s to exist: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(nested, ".nerd")); !os.IsNotExist(err) {
		t.Fatalf("Expected no <nested>/.nerd to be created, stat err = %v", err)
	}
}

func TestCacheOwner_NoConfigKeepsGivenDir(t *testing.T) {
	root := t.TempDir()

	cache := NewFileCache(root)
	want := filepath.Join(root, ".nerd", "cache", "manifest.json")
	if cache.path != want {
		t.Fatalf("Expected cache path %s, got %s", want, cache.path)
	}

	info := writeOwnerProbeFile(t, root)
	cache.Update(filepath.Join(root, "probe.go"), info, "hash-local")
	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(want); err != nil {
		t.Fatalf("Expected local manifest %s to exist: %v", want, err)
	}
}

func TestCacheOwner_StrayCacheDirDoesNotCapture(t *testing.T) {
	workspace := t.TempDir()
	if err := os.MkdirAll(filepath.Join(workspace, ".nerd"), 0755); err != nil {
		t.Fatalf("Failed to create workspace .nerd dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(workspace, ".nerd", "config.json"), []byte("{}"), 0644); err != nil {
		t.Fatalf("Failed to write workspace config: %v", err)
	}
	nested := filepath.Join(workspace, "sub", "dir")
	strayCache := filepath.Join(nested, ".nerd", "cache")
	if err := os.MkdirAll(strayCache, 0755); err != nil {
		t.Fatalf("Failed to create stray cache dir: %v", err)
	}

	cache := NewFileCache(nested)
	want := filepath.Join(workspace, ".nerd", "cache", "manifest.json")
	if cache.path != want {
		t.Fatalf("Stray dir captured the cache: expected %s, got %s", want, cache.path)
	}

	info := writeOwnerProbeFile(t, nested)
	cache.Update(filepath.Join(nested, "probe.go"), info, "hash-stray")
	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if _, err := os.Stat(want); err != nil {
		t.Fatalf("Expected workspace manifest %s to exist: %v", want, err)
	}
	if _, err := os.Stat(filepath.Join(strayCache, "manifest.json")); !os.IsNotExist(err) {
		t.Fatalf("Expected no manifest under stray dir, stat err = %v", err)
	}
}
