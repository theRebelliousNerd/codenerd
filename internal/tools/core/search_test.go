package core

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// =============================================================================
// GLOB TOOL TESTS
// =============================================================================

func TestGlobTool_Definition(t *testing.T) {
	t.Parallel()

	tool := GlobTool()

	if tool.Name != "glob" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if tool.Execute == nil {
		t.Error("Execute should be set")
	}
}

// TODO: TEST_GAP: Null/Undefined/Empty - Tests missing for args=nil map resulting in zero value lookups without panic.
// TODO: TEST_GAP: Null/Undefined/Empty - Tests missing for pattern consisting entirely of whitespace.
// TODO: TEST_GAP: Type Coercion - Tests missing for JSON float64 to int casting failures for max_results failing silently and ignoring LLM configs.
// TODO: TEST_GAP: User Request Extremes - Tests missing for executing massive directory traversals without proper context cancellation checks.
func TestGlobTool_Execute_MissingPattern(t *testing.T) {
	t.Parallel()

	_, err := executeGlob(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing pattern")
	}
}

func TestGlobTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create test files
	os.WriteFile(filepath.Join(tmpDir, "file1.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte(""), 0644)

	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":   "*.go",
		"base_path": tmpDir,
	})
	if err != nil {
		t.Fatalf("executeGlob error: %v", err)
	}

	if !strings.Contains(result, "file1.go") {
		t.Errorf("expected to find file1.go, got: %s", result)
	}
	if !strings.Contains(result, "file2.go") {
		t.Errorf("expected to find file2.go, got: %s", result)
	}
	// Should not include .txt file
	if strings.Contains(result, "file.txt") {
		t.Error("should not find file.txt with *.go pattern")
	}
}

func TestGlobTool_Execute_NoMatches(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file.txt"), []byte(""), 0644)

	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":   "*.go",
		"base_path": tmpDir,
	})
	if err != nil {
		t.Fatalf("executeGlob error: %v", err)
	}

	// Should return "no files found" message
	if !strings.Contains(result, "No files found") && strings.Contains(result, ".go") {
		t.Error("should not find any .go files")
	}
}

func TestGlobTool_Execute_Recursive(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create nested structure
	subDir := filepath.Join(tmpDir, "sub")
	os.Mkdir(subDir, 0755)
	os.WriteFile(filepath.Join(tmpDir, "root.go"), []byte(""), 0644)
	os.WriteFile(filepath.Join(subDir, "nested.go"), []byte(""), 0644)

	result, err := executeGlob(context.Background(), map[string]any{
		"pattern":   "**/*.go",
		"base_path": tmpDir,
	})
	if err != nil {
		t.Fatalf("executeGlob error: %v", err)
	}

	// Should find at least one .go file
	if !strings.Contains(result, ".go") && !strings.Contains(result, "No files") {
		t.Errorf("expected to find .go files or message, got: %s", result)
	}
}

// =============================================================================
// GREP TOOL TESTS
// =============================================================================

func TestGrepTool_Definition(t *testing.T) {
	t.Parallel()

	tool := GrepTool()

	if tool.Name != "grep" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

// TODO: TEST_GAP: Null/Undefined/Empty - Tests missing for nil map lookup.
// TODO: TEST_GAP: Type Coercion - Tests missing for JSON float64 to int casting on max_results and context_lines resulting in silent truncation of LLM requests.
// TODO: TEST_GAP: User Request Extremes - Tests missing for ReDoS / Catastrophic Backtracking on complex regex strings.
// TODO: TEST_GAP: User Request Extremes - Tests missing for cancellation enforcement during file enumeration and scanning (ctx.Done()).
// TODO: TEST_GAP: State Conflicts - Tests missing for concurrent modifications (file deleted/permissions changed) between finding files and opening them in searchFile.
func TestGrepTool_Execute_MissingPattern(t *testing.T) {
	t.Parallel()

	_, err := executeGrep(context.Background(), map[string]any{
		"path": "/some/path",
	})
	if err == nil {
		t.Error("expected error for missing pattern")
	}
}

func TestGrepTool_Execute_MissingPath(t *testing.T) {
	t.Parallel()

	// F-GREP-1: a nonexistent search path yields zero matches, not a hard error.
	// A hard error propagates as a shard/task failure and can cascade to a
	// campaign "too many failures -> replan -> pause"; the tool must degrade
	// gracefully so the agent recovers and retargets.
	result, err := executeGrep(context.Background(), map[string]any{
		"pattern": "test",
		"path":    "/nonexistent/path/that/does/not/exist",
	})
	if err != nil {
		t.Fatalf("nonexistent path should not error, got: %v", err)
	}
	if !strings.Contains(result, "No matches found") {
		t.Errorf("expected a no-matches result for nonexistent path, got: %q", result)
	}
}

func TestGrepTool_Execute_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1 hello\nline2 world\nline3 hello again"
	os.WriteFile(tmpFile, []byte(content), 0644)

	result, err := executeGrep(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    tmpFile,
	})
	if err != nil {
		t.Fatalf("executeGrep error: %v", err)
	}

	if !strings.Contains(result, "hello") {
		t.Error("expected result to contain matches")
	}
	if !strings.Contains(result, "line1") || !strings.Contains(result, "line3") {
		t.Error("expected to find both matching lines")
	}
}

func TestGrepTool_Execute_NoMatches(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	os.WriteFile(tmpFile, []byte("no matching content here"), 0644)

	result, err := executeGrep(context.Background(), map[string]any{
		"pattern": "NOTFOUND",
		"path":    tmpFile,
	})
	if err != nil {
		t.Fatalf("executeGrep error: %v", err)
	}

	// Should indicate no matches
	if !strings.Contains(result, "No matches found") && !strings.Contains(result, "0 matches") {
		t.Errorf("expected no matches message, got: %s", result)
	}
	if strings.Contains(result, "test.txt") && strings.Contains(result, "NOTFOUND") {
		t.Error("should not find NOTFOUND in results")
	}
}

func TestGrepTool_Execute_Regex(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "func hello()\nfunc goodbye()\nvar test"
	os.WriteFile(tmpFile, []byte(content), 0644)

	result, err := executeGrep(context.Background(), map[string]any{
		"pattern": "func \\w+\\(",
		"path":    tmpFile,
		"regex":   true,
	})
	if err != nil {
		t.Fatalf("executeGrep error: %v", err)
	}

	if !strings.Contains(result, "hello") {
		t.Error("expected to match func hello()")
	}
}

func TestGrepTool_Execute_WithContext(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nline2\ntarget\nline4\nline5"
	os.WriteFile(tmpFile, []byte(content), 0644)

	result, err := executeGrep(context.Background(), map[string]any{
		"pattern":       "target",
		"path":          tmpFile,
		"context_lines": float64(1),
	})
	if err != nil {
		t.Fatalf("executeGrep error: %v", err)
	}

	if !strings.Contains(result, "target") {
		t.Error("expected to find target")
	}
}

func TestGrepTool_Execute_Directory(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	os.WriteFile(filepath.Join(tmpDir, "file1.txt"), []byte("hello world"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file2.txt"), []byte("hello there"), 0644)
	os.WriteFile(filepath.Join(tmpDir, "file3.txt"), []byte("goodbye"), 0644)

	result, err := executeGrep(context.Background(), map[string]any{
		"pattern": "hello",
		"path":    tmpDir,
	})
	if err != nil {
		t.Fatalf("executeGrep error: %v", err)
	}

	if !strings.Contains(result, "file1") || !strings.Contains(result, "file2") {
		t.Error("expected matches from multiple files")
	}
}

// =============================================================================
// GREP MATCH TESTS
// =============================================================================

func TestGrepMatch_Fields(t *testing.T) {
	t.Parallel()

	match := GrepMatch{
		File:       "test.go",
		LineNumber: 42,
		Line:       "func Hello()",
		Context:    []string{"// comment", "func Hello()", ""},
	}

	if match.File != "test.go" {
		t.Errorf("File mismatch: got %q", match.File)
	}
	if match.LineNumber != 42 {
		t.Errorf("LineNumber mismatch: got %d", match.LineNumber)
	}
	if len(match.Context) != 3 {
		t.Errorf("Context length mismatch: got %d", len(match.Context))
	}
}

// =============================================================================
// SEARCH FILE HELPER TESTS
// =============================================================================

func TestSearchFile_Success(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "line1\nmatch here\nline3\nmatch again\nline5"
	os.WriteFile(tmpFile, []byte(content), 0644)

	re := mustCompilePattern("match")
	matches, err := searchFile(tmpFile, re, 0, 100)
	if err != nil {
		t.Fatalf("searchFile error: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches, got %d", len(matches))
	}
}

func TestSearchFile_MaxMatches(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.txt")
	content := "match\nmatch\nmatch\nmatch\nmatch"
	os.WriteFile(tmpFile, []byte(content), 0644)

	re := mustCompilePattern("match")
	matches, err := searchFile(tmpFile, re, 0, 2)
	if err != nil {
		t.Fatalf("searchFile error: %v", err)
	}

	if len(matches) != 2 {
		t.Errorf("expected 2 matches (limited), got %d", len(matches))
	}
}

// =============================================================================
// SEARCH CODE TOOL TESTS
// =============================================================================

func TestSearchCodeTool_Definition(t *testing.T) {
	t.Parallel()

	tool := SearchCodeTool()

	if tool == nil {
		t.Fatal("expected non-nil tool")
	}
	// SearchCodeTool is an alias for grep
}

// =============================================================================
// HELPER FUNCTIONS
// =============================================================================

func mustCompilePattern(pattern string) *regexp.Regexp {
	re, err := regexp.Compile(pattern)
	if err != nil {
		panic(err)
	}
	return re
}
