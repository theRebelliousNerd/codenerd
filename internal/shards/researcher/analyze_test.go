package researcher

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractWorkspacePath(t *testing.T) {
	r := NewResearcherShard()

	// Create a temp directory for testing
	tmpDir, err := os.MkdirTemp("", "researcher_test")
	if err != nil {
		t.Fatalf("Failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a subdirectory
	subDir := filepath.Join(tmpDir, "internal", "autopoiesis")
	if err := os.MkdirAll(subDir, 0755); err != nil {
		t.Fatalf("Failed to create subdir: %v", err)
	}

	tests := []struct {
		name     string
		task     string
		wantEmpty bool // true if we expect empty string (fallback to cwd)
	}{
		{
			name:     "empty task",
			task:     "",
			wantEmpty: false, // returns "" which triggers cwd fallback
		},
		{
			name:     "dot task",
			task:     ".",
			wantEmpty: false, // returns "." which triggers cwd fallback
		},
		{
			name:     "review file with invalid path",
			task:     "review file:nonexistent/path",
			wantEmpty: true, // should return "" because path doesn't exist
		},
		{
			name:     "research command with analyze keyword",
			task:     "research: analyze the codebase for proper API usage",
			wantEmpty: true, // not a valid path, should return ""
		},
		{
			name:     "security scan command",
			task:     "security_scan files:a.go,b.go",
			wantEmpty: true, // comma-separated files, not a directory
		},
		{
			name:     "plain text research query",
			task:     "how to implement authentication in Go",
			wantEmpty: true, // not a path at all
		},
		{
			name:     "valid directory directly",
			task:     tmpDir,
			wantEmpty: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := r.extractWorkspacePath(tt.task)

			if tt.wantEmpty && result != "" {
				t.Errorf("extractWorkspacePath(%q) = %q, want empty string", tt.task, result)
			}

			// For valid directory test
			if tt.task == tmpDir && result != tmpDir {
				t.Errorf("extractWorkspacePath(%q) = %q, want %q", tt.task, result, tmpDir)
			}
		})
	}
}

func TestExtractWorkspacePathWithRealAutopoiesisDir(t *testing.T) {
	r := NewResearcherShard()

	// Test with the actual task format that caused the bug
	// "review file:internal\autopoiesis" should extract "internal\autopoiesis"
	// but since that's a relative path, it may not exist from test context
	// The key is that command-style tasks with "file:" prefix get parsed correctly

	testCases := []struct {
		task     string
		desc     string
	}{
		{"review file:internal/autopoiesis", "review with file: prefix"},
		{"security_scan file:src/main.go", "security scan with file: prefix"},
		{"research: analyze the codebase for API patterns", "research command"},
		{"review file:c:/some/absolute/path", "absolute path (doesn't exist)"},
	}

	for _, tc := range testCases {
		result := r.extractWorkspacePath(tc.task)
		t.Logf("%s: task=%q -> result=%q", tc.desc, tc.task, result)

		// For non-existent paths, should return empty (fallback to cwd)
		// The important thing is it doesn't crash or return the command string
		if result == tc.task {
			t.Errorf("extractWorkspacePath should NOT return the raw task string for %q", tc.task)
		}
	}
}

func TestAnalyzeCodebaseFallsBackToCwd(t *testing.T) {
	// This test verifies that analyzeCodebase falls back to cwd for invalid workspaces
	r := NewResearcherShard()

	// Test with an invalid workspace path (command-style task)
	invalidWorkspace := "review file:nonexistent/path"
	extractedPath := r.extractWorkspacePath(invalidWorkspace)

	if extractedPath != "" {
		t.Errorf("extractWorkspacePath should return empty for invalid path, got %q", extractedPath)
	}

	// Verify the fallback logic in analyzeCodebase would work
	// (We don't call analyzeCodebase directly as it requires LLM and scanner)
	workspace := extractedPath
	if workspace == "" || workspace == "." {
		workspace, _ = os.Getwd()
	}

	// Verify workspace is now a valid directory
	info, err := os.Stat(workspace)
	if err != nil {
		t.Errorf("Fallback workspace should exist: %v", err)
	}
	if !info.IsDir() {
		t.Errorf("Fallback workspace should be a directory")
	}
}
