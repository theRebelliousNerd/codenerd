package world

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestScanWorkspace_BlindSpotFix(t *testing.T) {
	// Create temp workspace
	tmpDir, err := os.MkdirTemp("", "blind_spot_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Structure:
	// /main.go (Visible)
	// /.github/workflows/ci.yml (Visible - Allowed hidden)
	// /.vscode/settings.json (Visible - Allowed hidden)
	// /.git/config (Hidden - Blocked)
	// /.nerd/cache/manifest.json (Hidden - Blocked)
	// /.secret/key.pem (Hidden - Blocked by default)

	files := map[string]bool{ // path -> expected visibility
		"main.go":                    true,
		".github/workflows/ci.yml":   true,
		".vscode/settings.json":      true,
		".git/config":                false,
		".nerd/cache/manifest.json":  false,
		".secret/key.pem":            false,
	}

	for path, _ := range files {
		fullPath := filepath.Join(tmpDir, path)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(fullPath, []byte("content"), 0644); err != nil {
			t.Fatal(err)
		}
	}

	scanner := NewScanner()
	facts, err := scanner.ScanWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("ScanWorkspace failed: %v", err)
	}

	// Verify visibility
	foundFiles := make(map[string]bool)
	for _, f := range facts {
		if f.Predicate == "file_topology" {
			path := f.Args[0].(string)
			relPath, _ := filepath.Rel(tmpDir, path)
			// Normalize path separators for Windows
			relPath = filepath.ToSlash(relPath)
			foundFiles[relPath] = true
		}
	}

	for path, expected := range files {
		found := foundFiles[path]
		if found != expected {
			t.Errorf("File %s visibility mismatch: got %v, want %v", path, found, expected)
		}
	}
}

func TestScanWorkspace_CacheBehavior(t *testing.T) {
	// Create temp workspace
	tmpDir, err := os.MkdirTemp("", "cache_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file
	filePath := filepath.Join(tmpDir, "test.go")
	originalContent := "package main"
	if err := os.WriteFile(filePath, []byte(originalContent), 0644); err != nil {
		t.Fatal(err)
	}

	// Set a fixed mtime
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	if err := os.Chtimes(filePath, fixedTime, fixedTime); err != nil {
		t.Fatal(err)
	}

	scanner := NewScanner()

	// 1. First Scan: Should calculate hash and populate cache
	facts1, err := scanner.ScanWorkspace(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	hash1 := facts1[0].Args[1].(string)

	// Verify cache exists
	cachePath := filepath.Join(tmpDir, ".nerd", "cache", "manifest.json")
	if _, err := os.Stat(cachePath); os.IsNotExist(err) {
		t.Fatal("Cache file was not created")
	}

	// 2. Verify Cache Content
	cache := NewFileCache(tmpDir)
	if len(cache.Entries) != 1 {
		t.Errorf("Cache should have 1 entry, got %d", len(cache.Entries))
	}
	entry, ok := cache.Entries[filePath]
	if !ok {
		t.Fatal("Cache entry missing for file")
	}
	if entry.Hash != hash1 {
		t.Errorf("Cache hash mismatch: got %s, want %s", entry.Hash, hash1)
	}

	// 3. Hack the Cache (Simulate "Hash Reuse")
	// We change the hash in the cache BUT keep the mtime/size of the file the same.
	// If the scanner reuses the cache, it will return the HACKED hash.
	// If it reads the file, it will return the TRUE hash.
	hackedHash := "HACKED_HASH_12345"
	cache.Entries[filePath] = CacheEntry{
		Hash:    hackedHash,
		ModTime: entry.ModTime,
		Size:    entry.Size,
	}
	cache.Dirty = true
	if err := cache.Save(); err != nil {
		t.Fatal(err)
	}

	// 4. Second Scan: Should reuse cache (because mtime/size match)
	facts2, err := scanner.ScanWorkspace(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	hash2 := facts2[0].Args[1].(string)

	if hash2 != hackedHash {
		t.Errorf("Cache bypass detected! Scanner re-hashed file instead of using cache.\nGot: %s\nWant: %s", hash2, hackedHash)
	}

	// 5. Modify File: Should invalidate cache
	if err := os.WriteFile(filePath, []byte("package main // changed"), 0644); err != nil {
		t.Fatal(err)
	}
	// Update mtime
	newTime := fixedTime.Add(1 * time.Hour)
	if err := os.Chtimes(filePath, newTime, newTime); err != nil {
		t.Fatal(err)
	}

	// 6. Third Scan: Should calculate NEW hash (ignoring hacked cache because mtime changed)
	facts3, err := scanner.ScanWorkspace(tmpDir)
	if err != nil {
		t.Fatal(err)
	}
	hash3 := facts3[0].Args[1].(string)

	if hash3 == hackedHash {
		t.Error("Cache invalidation failed! Returned hacked hash after file modification.")
	}
	if hash3 == hash1 {
		t.Error("Hash collision or update failed! Returned original hash after content change.")
	}
}

// Test ScanDirectory wrapper for Context support
func TestScanDirectory_BlindSpotFix(t *testing.T) {
	// Reuse logic for ScanDirectory
	tmpDir, err := os.MkdirTemp("", "blind_spot_test_dir")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	os.MkdirAll(filepath.Join(tmpDir, ".github"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".github/ci.yml"), []byte("data"), 0644)
	
	os.MkdirAll(filepath.Join(tmpDir, ".git"), 0755)
	os.WriteFile(filepath.Join(tmpDir, ".git/config"), []byte("data"), 0644)

	scanner := NewScanner()
	res, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatal(err)
	}

	foundGithub := false
	foundGit := false

	for _, f := range res.Facts {
		path := f.Args[0].(string)
		if filepath.Base(path) == "ci.yml" {
			foundGithub = true
		}
		if filepath.Base(path) == "config" {
			foundGit = true
		}
	}

	if !foundGithub {
		t.Error("Failed to find .github/ci.yml")
	}
	if foundGit {
		t.Error("Incorrectly found .git/config")
	}
}
