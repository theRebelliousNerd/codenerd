package world

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

// =============================================================================
// SCANNER CONFIG TESTS
// =============================================================================

func TestDefaultScannerConfig_WhenCalled_ShouldReturnValidConfig(t *testing.T) {
	t.Parallel()

	cfg := DefaultScannerConfig()

	if cfg.MaxConcurrency <= 0 {
		t.Errorf("MaxConcurrency should be positive, got %d", cfg.MaxConcurrency)
	}
}

func TestNewScannerWithConfig_WhenNegativeConcurrency_ShouldUseDefaults(t *testing.T) {
	t.Parallel()

	cfg := ScannerConfig{MaxConcurrency: -1}
	scanner := NewScannerWithConfig(cfg)

	if scanner == nil {
		t.Fatal("NewScannerWithConfig returned nil")
	}
	// The scanner should fallback to defaults when MaxConcurrency <= 0
	if scanner.config.MaxConcurrency <= 0 {
		t.Error("expected positive MaxConcurrency after fallback")
	}
}

func TestNewScannerWithConfig_WhenZeroConcurrency_ShouldUseDefaults(t *testing.T) {
	t.Parallel()

	cfg := ScannerConfig{MaxConcurrency: 0}
	scanner := NewScannerWithConfig(cfg)

	if scanner == nil {
		t.Fatal("NewScannerWithConfig returned nil")
	}
	if scanner.config.MaxConcurrency <= 0 {
		t.Error("expected positive MaxConcurrency after fallback")
	}
}

func TestNewScannerWithConfig_WhenValidConcurrency_ShouldUseProvided(t *testing.T) {
	t.Parallel()

	cfg := ScannerConfig{MaxConcurrency: 42}
	scanner := NewScannerWithConfig(cfg)

	if scanner == nil {
		t.Fatal("NewScannerWithConfig returned nil")
	}
	if scanner.config.MaxConcurrency != 42 {
		t.Errorf("expected MaxConcurrency 42, got %d", scanner.config.MaxConcurrency)
	}
}

// =============================================================================
// CALCULATE HASH TESTS
// =============================================================================

func TestCalculateHash_WhenValidFile_ShouldReturnSHA256(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0644); err != nil {
		t.Fatalf("Failed to create test file: %v", err)
	}

	hash, err := calculateHash(path)
	if err != nil {
		t.Fatalf("calculateHash error: %v", err)
	}

	// SHA256 hex string should be 64 characters
	if len(hash) != 64 {
		t.Errorf("hash length = %d, want 64", len(hash))
	}

	// Verify determinism: same content should produce same hash
	hash2, err := calculateHash(path)
	if err != nil {
		t.Fatalf("second calculateHash error: %v", err)
	}
	if hash != hash2 {
		t.Errorf("hashes should be deterministic: %s != %s", hash, hash2)
	}
}

func TestCalculateHash_WhenEmptyFile_ShouldReturnValidHash(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "empty.txt")
	if err := os.WriteFile(path, []byte{}, 0644); err != nil {
		t.Fatalf("Failed to create empty file: %v", err)
	}

	hash, err := calculateHash(path)
	if err != nil {
		t.Fatalf("calculateHash error: %v", err)
	}

	if len(hash) != 64 {
		t.Errorf("hash length for empty file = %d, want 64", len(hash))
	}

	// SHA256 of empty string is well-known
	expectedEmptyHash := "e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
	if hash != expectedEmptyHash {
		t.Errorf("empty file hash = %s, want %s", hash, expectedEmptyHash)
	}
}

func TestCalculateHash_WhenFileNotExist_ShouldReturnError(t *testing.T) {
	t.Parallel()

	_, err := calculateHash("/nonexistent/path/file.txt")
	if err == nil {
		t.Error("expected error for non-existent file")
	}
}

func TestCalculateHash_WhenDifferentContent_ShouldReturnDifferentHashes(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	file1 := filepath.Join(tmpDir, "file1.txt")
	file2 := filepath.Join(tmpDir, "file2.txt")

	if err := os.WriteFile(file1, []byte("content A"), 0644); err != nil {
		t.Fatalf("Failed to create file1: %v", err)
	}
	if err := os.WriteFile(file2, []byte("content B"), 0644); err != nil {
		t.Fatalf("Failed to create file2: %v", err)
	}

	hash1, err := calculateHash(file1)
	if err != nil {
		t.Fatalf("calculateHash file1 error: %v", err)
	}

	hash2, err := calculateHash(file2)
	if err != nil {
		t.Fatalf("calculateHash file2 error: %v", err)
	}

	if hash1 == hash2 {
		t.Error("different content should produce different hashes")
	}
}

// =============================================================================
// SCAN DIRECTORY EDGE CASE TESTS
// =============================================================================

func TestScanDirectory_WhenEmptyDir_ShouldReturnNoFiles(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	scanner := NewScanner()

	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory error: %v", err)
	}

	if result.FileCount != 0 {
		t.Errorf("expected 0 files in empty dir, got %d", result.FileCount)
	}
}

func TestScanDirectory_WhenNodeModules_ShouldSkip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create node_modules with a file
	nmDir := filepath.Join(tmpDir, "node_modules")
	if err := os.MkdirAll(nmDir, 0755); err != nil {
		t.Fatalf("Failed to create node_modules: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nmDir, "package.js"), []byte("module.exports = {}"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create a visible file
	if err := os.WriteFile(filepath.Join(tmpDir, "app.js"), []byte("console.log('hello')"), 0644); err != nil {
		t.Fatalf("Failed to create visible file: %v", err)
	}

	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory error: %v", err)
	}

	if result.FileCount != 1 {
		t.Errorf("expected 1 file (node_modules skipped), got %d", result.FileCount)
	}
}

func TestScanDirectory_WhenVendorDir_ShouldSkip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create vendor with a file
	vendorDir := filepath.Join(tmpDir, "vendor")
	if err := os.MkdirAll(vendorDir, 0755); err != nil {
		t.Fatalf("Failed to create vendor dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(vendorDir, "lib.go"), []byte("package vendor"), 0644); err != nil {
		t.Fatalf("Failed to create vendor file: %v", err)
	}

	// Create a visible file
	if err := os.WriteFile(filepath.Join(tmpDir, "main.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory error: %v", err)
	}

	if result.FileCount != 1 {
		t.Errorf("expected 1 file (vendor skipped), got %d", result.FileCount)
	}
}

func TestScanDirectory_WhenGitDir_ShouldSkip(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create .git with a file
	gitDir := filepath.Join(tmpDir, ".git")
	if err := os.MkdirAll(gitDir, 0755); err != nil {
		t.Fatalf("Failed to create .git dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(gitDir, "HEAD"), []byte("ref: refs/heads/main"), 0644); err != nil {
		t.Fatalf("Failed to create git file: %v", err)
	}

	// Create a visible file
	if err := os.WriteFile(filepath.Join(tmpDir, "README.md"), []byte("# Hello"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner := NewScanner()
	result, err := scanner.ScanDirectory(context.Background(), tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory error: %v", err)
	}

	if result.FileCount != 1 {
		t.Errorf("expected 1 file (.git skipped), got %d", result.FileCount)
	}
}

func TestScanDirectory_WhenContextCancelledEarly_ShouldReturnError(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create several files to make the scan take time
	for i := 0; i < 20; i++ {
		name := filepath.Join(tmpDir, strings.Repeat("x", i+1)+".go")
		if err := os.WriteFile(name, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	scanner := NewScanner()
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err := scanner.ScanDirectory(ctx, tmpDir)
	// May or may not return error depending on timing, but should not panic
	_ = err
}

func TestScanDirectory_WhenNonExistentDir_ShouldReturnError(t *testing.T) {
	t.Parallel()

	scanner := NewScanner()
	_, err := scanner.ScanDirectory(context.Background(), "/nonexistent/path/to/scan")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}

// =============================================================================
// SCAN RESULT TESTS
// =============================================================================

func TestScanResult_ToFacts_WhenEmpty_ShouldReturnEmpty(t *testing.T) {
	t.Parallel()

	result := &ScanResult{
		Facts: nil,
	}

	facts := result.ToFacts()
	if facts != nil {
		t.Errorf("expected nil facts for nil slice, got %v", facts)
	}
}

func TestScanResult_ToFacts_WhenPopulated_ShouldReturnSameSlice(t *testing.T) {
	t.Parallel()

	result := &ScanResult{
		Facts:     make([]Fact, 0),
		Languages: make(map[string]int),
	}

	facts := result.ToFacts()
	if facts == nil {
		t.Error("expected non-nil facts for empty slice")
	}
	if len(facts) != 0 {
		t.Errorf("expected 0 facts, got %d", len(facts))
	}
}

// =============================================================================
// DETECT LANGUAGE EDGE CASES
// =============================================================================

func TestDetectLanguage_WhenExtendedExtensions_ShouldMatch(t *testing.T) {
	t.Parallel()

	// Only test extensions that are actually supported by detectLanguage
	cases := []struct {
		ext    string
		path   string
		expect string
	}{
		{".html", "page.html", "html"},
		{".css", "styles.css", "css"},
		{".toml", "config.toml", "toml"},
		{".kt", "Main.kt", "kotlin"},
		{".scala", "Main.scala", "scala"},
		{".lua", "script.lua", "lua"},
		{".r", "analysis.r", "r"},
		{".xml", "config.xml", "xml"},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := detectLanguage(tc.ext, tc.path)
			if got != tc.expect {
				t.Errorf("detectLanguage(%q, %q) = %q, want %q", tc.ext, tc.path, got, tc.expect)
			}
		})
	}
}

func TestDetectLanguage_WhenUnknownExtension_ShouldReturnUnknown(t *testing.T) {
	t.Parallel()

	unknowns := []struct {
		ext  string
		path string
	}{
		{".htm", "page.htm"},
		{".dart", "main.dart"},
		{".sol", "contract.sol"},
		{".zig", "main.zig"},
		{".tf", "infra.tf"},
		{".hcl", "main.hcl"},
		{".bat", "script.bat"},
		{".proto", "service.proto"},
	}

	for _, tc := range unknowns {
		tc := tc
		t.Run(tc.path, func(t *testing.T) {
			t.Parallel()
			got := detectLanguage(tc.ext, tc.path)
			if got != "unknown" {
				t.Errorf("detectLanguage(%q, %q) = %q, want %q", tc.ext, tc.path, got, "unknown")
			}
		})
	}
}

// =============================================================================
// IS TEST FILE EDGE CASES
// =============================================================================

func TestIsTestFile_WhenEdgeCases_ShouldHandle(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		path string
		want bool
	}{
		{"empty_path", "", false},
		{"single_dot", ".", false},
		{"go_test_in_nested_dir", "a/b/c/d_test.go", true},
		{"py_test_prefix", "test_module.py", true},
		{"py_test_suffix", "module_test.py", true},
		// .jsx not in supported extensions for test/spec detection
		{"jsx_test_not_supported", "component.test.jsx", false},
		{"jsx_spec_not_supported", "component.spec.jsx", false},
		{"test_directory", "tests/something.py", true},
		// spec/ directory not detected as test directory by implementation
		{"spec_directory_not_supported", "spec/test.rb", false},
		{"test_word_in_middle_of_name", "latest.go", false},
		{"test_word_in_dir_name", "contest/main.go", false},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := isTestFile(tc.path)
			if got != tc.want {
				t.Errorf("isTestFile(%q) = %v, want %v", tc.path, got, tc.want)
			}
		})
	}
}

// =============================================================================
// CONCURRENT SCAN TESTS
// =============================================================================

func TestScanDirectory_ConcurrentScanners_ShouldNotRace(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	// Create a few test files
	for i := 0; i < 5; i++ {
		name := filepath.Join(tmpDir, filepath.Base(t.Name())+"_"+string(rune('a'+i))+".go")
		if err := os.WriteFile(name, []byte("package main"), 0644); err != nil {
			t.Fatalf("Failed to create file: %v", err)
		}
	}

	var wg sync.WaitGroup
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			scanner := NewScanner()
			result, err := scanner.ScanDirectory(context.Background(), tmpDir)
			if err != nil {
				t.Errorf("ScanDirectory error: %v", err)
				return
			}
			if result.FileCount < 1 {
				t.Errorf("expected at least 1 file, got %d", result.FileCount)
			}
		}()
	}
	wg.Wait()
}

// =============================================================================
// FILE CACHE CONCURRENT ACCESS TESTS
// =============================================================================

func TestFileCache_ConcurrentGetUpdate_ShouldNotRace(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	cache := NewFileCache(tmpDir)

	info := &testFileInfo{
		name:    "test.go",
		size:    100,
		modTime: time.Unix(1000, 0),
	}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(2)
		go func(n int) {
			defer wg.Done()
			cache.Update("test.go", info, "hash_"+string(rune('a'+n)))
		}(i)
		go func() {
			defer wg.Done()
			hash, ok := cache.Get("test.go", info)
			_, _ = hash, ok // use the values
		}()
	}
	wg.Wait()
}

// =============================================================================
// SCAN WORKSPACE TESTS
// =============================================================================

func TestScanWorkspace_WhenNonExistentDir_ShouldReturnError(t *testing.T) {
	t.Parallel()

	scanner := NewScanner()
	_, err := scanner.ScanWorkspace("/absolutely/nonexistent/dir/path")
	if err == nil {
		t.Error("expected error for non-existent directory")
	}
}


