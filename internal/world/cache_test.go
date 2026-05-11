package world

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Dummy file info for testing
type testFileInfo struct {
	name    string
	size    int64
	modTime time.Time
}

func (t *testFileInfo) Name() string       { return t.name }
func (t *testFileInfo) Size() int64        { return t.size }
func (t *testFileInfo) Mode() os.FileMode  { return 0644 }
func (t *testFileInfo) ModTime() time.Time { return t.modTime }
func (t *testFileInfo) IsDir() bool        { return false }
func (t *testFileInfo) Sys() interface{}   { return nil }

func TestNewFileCache(t *testing.T) {
	// Create a temporary workspace root
	tempDir := t.TempDir()

	// Test creating a new cache
	cache := NewFileCache(tempDir)

	if cache == nil {
		t.Fatal("Expected NewFileCache to return a non-nil object")
	}

	if cache.path != filepath.Join(tempDir, ".nerd", "cache", "manifest.json") {
		t.Errorf("Expected path %s, got %s", filepath.Join(tempDir, ".nerd", "cache", "manifest.json"), cache.path)
	}

	if len(cache.Entries) != 0 {
		t.Errorf("Expected cache entries to be empty, got %d", len(cache.Entries))
	}

	if cache.Dirty {
		t.Error("Expected new cache to not be dirty")
	}
}

func TestFileCache_LoadAndSave(t *testing.T) {
	tempDir := t.TempDir()

	// Create a pre-existing cache file
	cachePath := filepath.Join(tempDir, ".nerd", "cache", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	initialData := map[string]CacheEntry{
		"test/file1.go": {
			Hash:    "hash1",
			ModTime: 1000,
			Size:    100,
		},
	}

	data, err := json.Marshal(initialData)
	if err != nil {
		t.Fatalf("Failed to marshal initial data: %v", err)
	}

	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		t.Fatalf("Failed to write initial cache: %v", err)
	}

	// Test loading
	cache := NewFileCache(tempDir)

	if len(cache.Entries) != 1 {
		t.Fatalf("Expected 1 entry, got %d", len(cache.Entries))
	}

	entry, ok := cache.Entries["test/file1.go"]
	if !ok {
		t.Fatal("Expected entry 'test/file1.go' to exist")
	}

	if entry.Hash != "hash1" || entry.ModTime != 1000 || entry.Size != 100 {
		t.Errorf("Loaded entry does not match initial data: %+v", entry)
	}

	// Test adding a new entry and saving
	info := &testFileInfo{
		name:    "file2.go",
		size:    200,
		modTime: time.Unix(2000, 0),
	}
	cache.Update("test/file2.go", info, "hash2")

	if !cache.Dirty {
		t.Error("Expected cache to be dirty after update")
	}

	if err := cache.Save(); err != nil {
		t.Fatalf("Save failed: %v", err)
	}

	if cache.Dirty {
		t.Error("Expected cache to not be dirty after save")
	}

	// Verify saved data
	savedData, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("Failed to read saved cache: %v", err)
	}

	var savedEntries map[string]CacheEntry
	if err := json.Unmarshal(savedData, &savedEntries); err != nil {
		t.Fatalf("Failed to unmarshal saved cache: %v", err)
	}

	if len(savedEntries) != 2 {
		t.Fatalf("Expected 2 entries, got %d", len(savedEntries))
	}

	if savedEntries["test/file2.go"].Hash != "hash2" {
		t.Errorf("Expected hash 'hash2', got '%s'", savedEntries["test/file2.go"].Hash)
	}
}

func TestFileCache_GetAndUpdate(t *testing.T) {
	tempDir := t.TempDir()
	cache := NewFileCache(tempDir)

	filePath := "test/file.go"
	info := &testFileInfo{
		name:    "file.go",
		size:    100,
		modTime: time.Unix(1000, 0),
	}

	// Test Get on missing entry
	hash, ok := cache.Get(filePath, info)
	if ok {
		t.Errorf("Expected Get to return false for missing entry, got true with hash %s", hash)
	}

	// Test Update
	expectedHash := "myhash123"
	cache.Update(filePath, info, expectedHash)

	if !cache.Dirty {
		t.Error("Expected cache to be dirty after Update")
	}

	// Test Get on existing entry
	hash, ok = cache.Get(filePath, info)
	if !ok {
		t.Error("Expected Get to return true for existing entry")
	}
	if hash != expectedHash {
		t.Errorf("Expected hash %s, got %s", expectedHash, hash)
	}

	// Test Get on existing entry with modified info
	changedInfo := &testFileInfo{
		name:    "file.go",
		size:    100,
		modTime: time.Unix(1001, 0), // Mod time changed
	}
	hash, ok = cache.Get(filePath, changedInfo)
	if ok {
		t.Errorf("Expected Get to return false for modified file, got true with hash %s", hash)
	}
}

func TestFileCache_CorruptData(t *testing.T) {
	tempDir := t.TempDir()

	// Create a corrupt cache file
	cachePath := filepath.Join(tempDir, ".nerd", "cache", "manifest.json")
	if err := os.MkdirAll(filepath.Dir(cachePath), 0755); err != nil {
		t.Fatalf("Failed to create cache dir: %v", err)
	}

	if err := os.WriteFile(cachePath, []byte("invalid json"), 0644); err != nil {
		t.Fatalf("Failed to write corrupt cache: %v", err)
	}

	// Test loading corrupt data
	cache := NewFileCache(tempDir)

	if len(cache.Entries) != 0 {
		t.Errorf("Expected cache to be initialized empty when corrupt data is loaded, got %d entries", len(cache.Entries))
	}
}

// Add tests to cover the failure paths in Save
func TestFileCache_SaveErrors(t *testing.T) {
	tempDir := t.TempDir()
	cache := NewFileCache(tempDir)

	// Create a scenario where Save fails due to directory creation error
	// Since we can't easily mock os.MkdirAll, we'll try an invalid path
	// E.g., make the parent directory a file instead of a dir
	invalidDir := filepath.Join(tempDir, "invalid_dir")
	if err := os.WriteFile(invalidDir, []byte("i am a file"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	cache.path = filepath.Join(invalidDir, "manifest.json")
	cache.Dirty = true
	cache.Entries["test"] = CacheEntry{Hash: "test"}

	err := cache.Save()
	if err == nil {
		t.Error("Expected Save to fail when parent dir is a file")
	}

	// What about JSON marshal failure? It's hard to make JSON marshal fail on our struct,
	// because it doesn't contain channels, functions, or cyclic maps.
}

func TestFileCache_SaveWriteError(t *testing.T) {
	tempDir := t.TempDir()
	cache := NewFileCache(tempDir)

	// Create a directory at the exact path we want to write to, which will cause os.WriteFile to fail
	dir := filepath.Join(tempDir, "some_dir")
	targetFile := filepath.Join(dir, "manifest.json")
	if err := os.MkdirAll(targetFile, 0755); err != nil {
		t.Fatalf("Failed to create dir at target file path: %v", err)
	}

	cache.path = targetFile
	cache.Dirty = true
	cache.Entries["test"] = CacheEntry{Hash: "test"}

	err := cache.Save()
	if err == nil {
		t.Error("Expected Save to fail when target path is a directory")
	}
}

func TestFileCache_LoadError(t *testing.T) {
	tempDir := t.TempDir()

	// Make a directory instead of a file to force an error other than os.IsNotExist
	cachePath := filepath.Join(tempDir, ".nerd", "cache", "manifest.json")
	if err := os.MkdirAll(cachePath, 0755); err != nil {
		t.Fatalf("Failed to create dir: %v", err)
	}

	// This should log a warning but not crash
	cache := NewFileCache(tempDir)

	if cache == nil {
		t.Fatal("Expected NewFileCache to return a cache even if load fails")
	}
}
