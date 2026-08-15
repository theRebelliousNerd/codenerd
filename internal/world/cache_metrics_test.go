package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"codenerd/internal/atomicfile"
)

// TestFileCache_WhenSaving_ShouldReplaceTheFileNotTruncateIt — the manifest is
// the only record of which files still have a valid hash. A truncating write
// (os.WriteFile) empties the live file before the new bytes land, so a crash or
// a concurrent reader sees a destroyed cache; an atomic save writes a temp file
// and renames it, which swaps the directory entry to a NEW inode and leaves the
// old bytes intact until the instant of the swap.
func TestFileCache_WhenSaving_ShouldReplaceTheFileNotTruncateIt(t *testing.T) {
	root := t.TempDir()
	cachePath := filepath.Join(root, ".nerd", "cache", "manifest.json")

	c := NewFileCache(root)
	c.Entries["a.go"] = CacheEntry{Hash: "h1", ModTime: 1, Size: 2}
	c.Dirty = true
	if err := c.Save(); err != nil {
		t.Fatalf("Save: %v", err)
	}
	before, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}

	// Hold the old contents open: with a rename-based save this handle keeps
	// reading a complete, consistent manifest even after the swap.
	oldHandle, err := atomicfile.Open(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	defer oldHandle.Close()

	c.Entries["b.go"] = CacheEntry{Hash: "h2", ModTime: 3, Size: 4}
	c.Dirty = true
	if err := c.Save(); err != nil {
		t.Fatalf("second Save: %v", err)
	}

	after, err := os.Stat(cachePath)
	if err != nil {
		t.Fatal(err)
	}
	// ReplaceFileW preserves file identity by design, so the inode proxy does
	// not hold on Windows while the guarantee still does.
	if runtime.GOOS != "windows" {
		if os.SameFile(before, after) {
			t.Error("save wrote through the existing file; a partial write would have destroyed the only copy")
		}
	}

	buf := make([]byte, 4096)
	n, _ := oldHandle.Read(buf)
	var previous map[string]CacheEntry
	if err := json.Unmarshal(buf[:n], &previous); err != nil {
		t.Errorf("previous manifest was mutated mid-save: %v", err)
	}
	if previous["a.go"].Hash != "h1" {
		t.Errorf("previous manifest lost data during the save: %v", previous)
	}
}

// TestFileCache_WhenSaved_ShouldLeaveNoTempFiles — the temp file is unique per
// save; if it is not cleaned up (or is left behind on the success path), the
// cache directory accumulates junk that later scans may try to read.
func TestFileCache_WhenSaved_ShouldLeaveNoTempFiles(t *testing.T) {
	root := t.TempDir()
	c := NewFileCache(root)
	c.Entries["a.go"] = CacheEntry{Hash: "h", ModTime: 1, Size: 1}
	c.Dirty = true
	if err := c.Save(); err != nil {
		t.Fatal(err)
	}

	entries, err := os.ReadDir(filepath.Join(root, ".nerd", "cache"))
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.Contains(e.Name(), ".tmp-") {
			t.Errorf("temp file %q left behind after a successful save", e.Name())
		}
	}

	// And the manifest must be complete, parseable JSON.
	data, err := os.ReadFile(filepath.Join(root, ".nerd", "cache", "manifest.json"))
	if err != nil {
		t.Fatal(err)
	}
	var got map[string]CacheEntry
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("manifest is not valid JSON after save: %v", err)
	}
	if got["a.go"].Hash != "h" {
		t.Errorf("manifest lost its entry: %v", got)
	}
}

// TestFileCache_WhenLookedUp_ShouldReportHitRate — the file cache decides
// whether a scan rehashes the whole repository, and until now it reported no
// effectiveness metric at all, so a cache that had quietly stopped matching was
// indistinguishable from a cold one.
func TestFileCache_WhenLookedUp_ShouldReportHitRate(t *testing.T) {
	root := t.TempDir()
	file := filepath.Join(root, "a.go")
	if err := os.WriteFile(file, []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(file)
	if err != nil {
		t.Fatal(err)
	}

	c := NewFileCache(root)
	if _, hit := c.Get(file, info); hit {
		t.Fatal("cold cache reported a hit")
	}
	c.Update(file, info, "hash")
	if _, hit := c.Get(file, info); !hit {
		t.Fatal("warm cache reported a miss")
	}

	stats := c.Stats()
	if stats.Hits != 1 || stats.Misses != 1 {
		t.Errorf("stats = %+v, want 1 hit and 1 miss", stats)
	}
	if got := stats.HitRate(); got != 50 {
		t.Errorf("hit rate = %v, want 50", got)
	}
	if !strings.Contains(stats.String(), "hitRate") {
		t.Errorf("stats string %q does not report a hit rate", stats.String())
	}
}
