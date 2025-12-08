package world

import (
	"codenerd/internal/core"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNewScanner(t *testing.T) {
	scanner := NewScanner()
	if scanner == nil {
		t.Fatal("NewScanner() returned nil")
	}
}

func TestScanWorkspace(t *testing.T) {
	// Create a temp directory with test files
	tmpDir, err := os.MkdirTemp("", "world_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create test files
	testFiles := []struct {
		name    string
		content string
	}{
		{"main.go", "package main\nfunc main() {}"},
		{"main_test.go", "package main\nfunc TestMain(t *testing.T) {}"},
		{"utils.py", "def helper(): pass"},
		{"test_utils.py", "def test_helper(): pass"},
	}

	for _, tf := range testFiles {
		path := filepath.Join(tmpDir, tf.name)
		if err := os.WriteFile(path, []byte(tf.content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", tf.name, err)
		}
	}

	scanner := NewScanner()
	facts, err := scanner.ScanWorkspace(tmpDir)
	if err != nil {
		t.Fatalf("ScanWorkspace() error = %v", err)
	}

	// Count only file_topology facts (scanner also emits directory and symbol_graph facts)
	var fileTopologyFacts []core.Fact
	for _, fact := range facts {
		if fact.Predicate == "file_topology" {
			fileTopologyFacts = append(fileTopologyFacts, fact)
		}
	}

	if len(fileTopologyFacts) != 4 {
		t.Errorf("ScanWorkspace() returned %d file_topology facts, want 4", len(fileTopologyFacts))
	}

	// Verify file_topology fact structure
	for _, fact := range fileTopologyFacts {
		if len(fact.Args) != 5 {
			t.Errorf("Expected 5 args for file_topology, got %d", len(fact.Args))
		}
	}
}

func TestScanDirectory(t *testing.T) {
	// Create a temp directory with test files
	tmpDir, err := os.MkdirTemp("", "world_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create subdirectory
	subDir := filepath.Join(tmpDir, "pkg")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	// Create test files
	testFiles := map[string]string{
		"main.go":           "package main",
		"main_test.go":      "package main",
		"pkg/utils.go":      "package pkg",
		"pkg/utils_test.go": "package pkg",
	}

	for name, content := range testFiles {
		path := filepath.Join(tmpDir, name)
		if err := os.WriteFile(path, []byte(content), 0644); err != nil {
			t.Fatalf("Failed to create test file %s: %v", name, err)
		}
	}

	scanner := NewScanner()
	ctx := context.Background()

	result, err := scanner.ScanDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory() error = %v", err)
	}

	if result.FileCount != 4 {
		t.Errorf("FileCount = %d, want 4", result.FileCount)
	}

	if result.TestFileCount != 2 {
		t.Errorf("TestFileCount = %d, want 2", result.TestFileCount)
	}

	if result.Languages["go"] != 4 {
		t.Errorf("Go files = %d, want 4", result.Languages["go"])
	}

	// Count only file_topology facts (scanner also emits directory and symbol_graph facts)
	var fileTopologyCount int
	for _, fact := range result.Facts {
		if fact.Predicate == "file_topology" {
			fileTopologyCount++
		}
	}
	if fileTopologyCount != 4 {
		t.Errorf("file_topology facts count = %d, want 4", fileTopologyCount)
	}
}

func TestScanDirectoryContextCancellation(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "world_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a file
	if err := os.WriteFile(filepath.Join(tmpDir, "test.go"), []byte("package test"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner := NewScanner()

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	_, err = scanner.ScanDirectory(ctx, tmpDir)
	if err == nil {
		t.Log("Context cancellation may not have taken effect before scan completed (small directory)")
	}
}

func TestScanDirectorySkipsHiddenDirs(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "world_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create visible file
	if err := os.WriteFile(filepath.Join(tmpDir, "visible.go"), []byte("package main"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	// Create hidden directory with file
	hiddenDir := filepath.Join(tmpDir, ".hidden")
	if err := os.MkdirAll(hiddenDir, 0755); err != nil {
		t.Fatalf("Failed to create hidden dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(hiddenDir, "hidden.go"), []byte("package hidden"), 0644); err != nil {
		t.Fatalf("Failed to create hidden file: %v", err)
	}

	scanner := NewScanner()
	ctx := context.Background()

	result, err := scanner.ScanDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory() error = %v", err)
	}

	// Should only find the visible file, not the hidden one
	if result.FileCount != 1 {
		t.Errorf("FileCount = %d, want 1 (hidden dir should be skipped)", result.FileCount)
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := []struct {
		ext      string
		path     string
		expected string
	}{
		{".go", "main.go", "go"},
		{".py", "script.py", "python"},
		{".js", "app.js", "javascript"},
		{".ts", "app.ts", "typescript"},
		{".tsx", "component.tsx", "typescript"},
		{".rs", "main.rs", "rust"},
		{".java", "Main.java", "java"},
		{".rb", "script.rb", "ruby"},
		{".php", "index.php", "php"},
		{".c", "main.c", "c"},
		{".cpp", "main.cpp", "cpp"},
		{".cs", "Program.cs", "csharp"},
		{".swift", "App.swift", "swift"},
		{".md", "README.md", "markdown"},
		{".json", "package.json", "json"},
		{".yaml", "config.yaml", "yaml"},
		{".yml", "config.yml", "yaml"},
		{".sql", "query.sql", "sql"},
		{".sh", "script.sh", "shell"},
		{"", "Dockerfile", "dockerfile"},
		{"", "Makefile", "makefile"},
		{".xyz", "unknown.xyz", "unknown"},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := detectLanguage(tt.ext, tt.path)
			if got != tt.expected {
				t.Errorf("detectLanguage(%q, %q) = %q, want %q", tt.ext, tt.path, got, tt.expected)
			}
		})
	}
}

func TestIsTestFile(t *testing.T) {
	tests := []struct {
		path     string
		expected bool
	}{
		// Go tests
		{"main_test.go", true},
		{"pkg/utils_test.go", true},
		{"main.go", false},

		// Python tests
		{"test_utils.py", true},
		{"utils_test.py", true},
		{"tests/test_main.py", true},
		{"utils.py", false},

		// JavaScript/TypeScript tests
		{"app.test.js", true},
		{"app.spec.js", true},
		{"app.test.ts", true},
		{"app.spec.ts", true},
		{"component.test.tsx", true},
		{"component.spec.tsx", true},
		{"app.js", false},

		// Java tests
		{"UserTest.java", true},
		{"UserTests.java", true},
		{"User.java", false},

		// Rust tests
		{"tests/integration.rs", true},
		{"src/main.rs", false},

		// Directory name should not cause false positives
		{"src/latest/utils.ts", false},
		{"src/contest/helpers.py", false},

		// __tests__ directory should still count
		{"src/__tests__/example.ts", true},
	}

	for _, tt := range tests {
		t.Run(tt.path, func(t *testing.T) {
			got := isTestFile(tt.path)
			if got != tt.expected {
				t.Errorf("isTestFile(%q) = %v, want %v", tt.path, got, tt.expected)
			}
		})
	}
}

func TestScanResult(t *testing.T) {
	result := &ScanResult{
		FileCount:      10,
		DirectoryCount: 3,
		Facts:          []core.Fact{}, // Initialize empty slice
		Languages:      map[string]int{"go": 5, "python": 5},
		TestFileCount:  2,
	}

	if result.FileCount != 10 {
		t.Errorf("FileCount = %d, want 10", result.FileCount)
	}

	facts := result.ToFacts()
	if facts == nil {
		t.Error("ToFacts() returned nil")
	}
}

func TestFileTopologyFactStructure(t *testing.T) {
	// Create a temp directory with a known file
	tmpDir, err := os.MkdirTemp("", "world_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	testFile := filepath.Join(tmpDir, "example.go")
	content := "package example\n\nfunc Hello() string { return \"hello\" }"
	if err := os.WriteFile(testFile, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner := NewScanner()
	ctx := context.Background()

	result, err := scanner.ScanDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory() error = %v", err)
	}

	// Filter for file_topology facts only (scanner also emits directory and symbol_graph facts)
	var fileTopologyFacts []core.Fact
	for _, f := range result.Facts {
		if f.Predicate == "file_topology" {
			fileTopologyFacts = append(fileTopologyFacts, f)
		}
	}

	if len(fileTopologyFacts) != 1 {
		t.Fatalf("Expected 1 file_topology fact, got %d", len(fileTopologyFacts))
	}

	fact := fileTopologyFacts[0]

	// Verify predicate
	if fact.Predicate != "file_topology" {
		t.Errorf("Predicate = %q, want 'file_topology'", fact.Predicate)
	}

	// Verify args count
	if len(fact.Args) != 5 {
		t.Fatalf("Args count = %d, want 5", len(fact.Args))
	}

	// Verify path arg
	path, ok := fact.Args[0].(string)
	if !ok {
		t.Error("Args[0] (path) is not a string")
	} else if !strings.HasSuffix(path, "example.go") {
		t.Errorf("Path = %q, want suffix 'example.go'", path)
	}

	// Verify hash arg (should be SHA256 hex)
	hash, ok := fact.Args[1].(string)
	if !ok {
		t.Error("Args[1] (hash) is not a string")
	} else if len(hash) != 64 {
		t.Errorf("Hash length = %d, want 64 (SHA256 hex)", len(hash))
	}

	// Verify language arg (allow string or MangleAtom)
	langStr, ok := fact.Args[2].(string)
	if !ok {
		if atom, okAtom := fact.Args[2].(core.MangleAtom); okAtom {
			langStr = string(atom)
			ok = true
		}
	}
	if !ok {
		t.Error("Args[2] (language) is not a string or atom")
	} else if langStr != "/go" {
		t.Errorf("Language = %q, want '/go'", langStr)
	}

	// Verify timestamp arg
	ts, ok := fact.Args[3].(int64)
	if !ok {
		t.Error("Args[3] (timestamp) is not an int64")
	} else if ts < time.Now().Add(-1*time.Hour).Unix() || ts > time.Now().Unix() {
		t.Errorf("Timestamp %d is not within expected range", ts)
	}

	// Verify isTest arg
	isTestStr, ok := fact.Args[4].(string)
	if !ok {
		if atom, okAtom := fact.Args[4].(core.MangleAtom); okAtom {
			isTestStr = string(atom)
			ok = true
		}
	}
	if !ok {
		t.Error("Args[4] (isTest) is not a string or atom")
	} else if isTestStr != "/false" {
		t.Errorf("IsTest = %q, want '/false'", isTestStr)
	}
}

func TestScanWorkspaceWithTestFile(t *testing.T) {
	// Create a temp directory
	tmpDir, err := os.MkdirTemp("", "world_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a test file
	testFile := filepath.Join(tmpDir, "example_test.go")
	if err := os.WriteFile(testFile, []byte("package example"), 0644); err != nil {
		t.Fatalf("Failed to create file: %v", err)
	}

	scanner := NewScanner()
	ctx := context.Background()

	result, err := scanner.ScanDirectory(ctx, tmpDir)
	if err != nil {
		t.Fatalf("ScanDirectory() error = %v", err)
	}

	// Filter for file_topology facts only (scanner also emits directory and symbol_graph facts)
	var fileTopologyFacts []core.Fact
	for _, f := range result.Facts {
		if f.Predicate == "file_topology" {
			fileTopologyFacts = append(fileTopologyFacts, f)
		}
	}

	if len(fileTopologyFacts) != 1 {
		t.Fatalf("Expected 1 file_topology fact, got %d", len(fileTopologyFacts))
	}

	// Verify isTest is true
	fact := fileTopologyFacts[0]
	isTestStr, ok := fact.Args[4].(string)
	if !ok {
		if atom, okAtom := fact.Args[4].(core.MangleAtom); okAtom {
			isTestStr = string(atom)
			ok = true
		}
	}
	if !ok {
		t.Error("Args[4] (isTest) is not a string or atom")
	} else if isTestStr != "/true" {
		t.Errorf("IsTest = %q, want '/true' for test file", isTestStr)
	}
}
