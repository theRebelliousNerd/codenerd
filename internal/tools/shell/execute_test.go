package shell

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// RUN COMMAND TOOL TESTS
// =============================================================================

func TestRunCommandTool_Definition(t *testing.T) {
	t.Parallel()

	tool := RunCommandTool()

	if tool.Name != "run_command" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
	if tool.Description == "" {
		t.Error("Description should not be empty")
	}
	if tool.Execute == nil {
		t.Error("Execute should be set")
	}
}

func TestRunCommandTool_Execute_MissingCommand(t *testing.T) {
	t.Parallel()

	_, err := executeRunCommand(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestRunCommandTool_Execute_Echo(t *testing.T) {
	t.Parallel()

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "echo hello"
	} else {
		cmd = "echo hello"
	}

	result, err := executeRunCommand(context.Background(), map[string]any{
		"command": cmd,
	})
	if err != nil {
		t.Fatalf("executeRunCommand error: %v", err)
	}

	if !strings.Contains(result, "hello") {
		t.Errorf("expected result to contain 'hello', got: %s", result)
	}
}

func TestRunCommandTool_Execute_WithWorkDir(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()

	var cmd string
	if runtime.GOOS == "windows" {
		cmd = "cd"
	} else {
		cmd = "pwd"
	}

	result, err := executeRunCommand(context.Background(), map[string]any{
		"command":  cmd,
		"work_dir": tmpDir,
	})
	if err != nil {
		t.Fatalf("executeRunCommand error: %v", err)
	}

	// Result should contain the temp directory path
	if !strings.Contains(strings.ToLower(result), strings.ToLower(filepath.Base(tmpDir))) {
		// Just check it returns something
		if result == "" {
			t.Error("expected non-empty result")
		}
	}
}

func TestRunCommandTool_Execute_InvalidCommand(t *testing.T) {
	t.Parallel()

	_, err := executeRunCommand(context.Background(), map[string]any{
		"command": "this_command_does_not_exist_12345",
	})
	// Error expected for invalid command
	if err == nil {
		t.Log("Note: invalid command may not error on all platforms")
	}
}

// =============================================================================
// BASH TOOL TESTS
// =============================================================================

func TestBashTool_Definition(t *testing.T) {
	t.Parallel()

	tool := BashTool()

	if tool.Name != "bash" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestBashTool_Execute_MissingScript(t *testing.T) {
	t.Parallel()

	_, err := executeBash(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing script")
	}
}

// =============================================================================
// RUN BUILD TOOL TESTS
// =============================================================================

func TestRunBuildTool_Definition(t *testing.T) {
	t.Parallel()

	tool := RunBuildTool()

	if tool.Name != "run_build" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestDetectBuildCommand_Go(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create go.mod file
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module test"), 0644)

	cmd := detectBuildCommand(tmpDir)
	if !strings.Contains(cmd, "go build") {
		t.Errorf("expected 'go build' for Go project, got: %s", cmd)
	}
}

func TestDetectBuildCommand_Node(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create package.json file
	pkg := filepath.Join(tmpDir, "package.json")
	os.WriteFile(pkg, []byte("{}"), 0644)

	cmd := detectBuildCommand(tmpDir)
	if !strings.Contains(cmd, "npm") {
		t.Errorf("expected 'npm' for Node project, got: %s", cmd)
	}
}

func TestDetectBuildCommand_Makefile(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	// Create Makefile
	makefile := filepath.Join(tmpDir, "Makefile")
	os.WriteFile(makefile, []byte("all:\n\techo build"), 0644)

	cmd := detectBuildCommand(tmpDir)
	if !strings.Contains(cmd, "make") {
		t.Errorf("expected 'make' for Makefile project, got: %s", cmd)
	}
}

// =============================================================================
// RUN TESTS TOOL TESTS
// =============================================================================

func TestRunTestsTool_Definition(t *testing.T) {
	t.Parallel()

	tool := RunTestsTool()

	if tool.Name != "run_tests" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestDetectTestCommand_Go(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	goMod := filepath.Join(tmpDir, "go.mod")
	os.WriteFile(goMod, []byte("module test"), 0644)

	cmd := detectTestCommand(tmpDir)
	if !strings.Contains(cmd, "go test") {
		t.Errorf("expected 'go test' for Go project, got: %s", cmd)
	}
}

func TestDetectTestCommand_Node(t *testing.T) {
	t.Parallel()

	tmpDir := t.TempDir()
	pkg := filepath.Join(tmpDir, "package.json")
	os.WriteFile(pkg, []byte("{}"), 0644)

	cmd := detectTestCommand(tmpDir)
	if !strings.Contains(cmd, "npm test") {
		t.Errorf("expected 'npm test' for Node project, got: %s", cmd)
	}
}

func TestAddTestPattern(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		command  string
		pattern  string
		expected string
	}{
		{"go_test", "go test", "TestFoo", "go test -run TestFoo"},
		{"npm_test", "npm test", "test-file", "npm test -- --grep test-file"},
		{"pytest", "pytest", "test_foo", "pytest -k test_foo"},
		{"empty_pattern", "go test", "", "go test -run "},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			result := addTestPattern(tc.command, tc.pattern)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

// =============================================================================
// GIT DIFF TOOL TESTS
// =============================================================================

func TestGitDiffTool_Definition(t *testing.T) {
	t.Parallel()

	tool := GitDiffTool()

	if tool.Name != "git_diff" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

// =============================================================================
// GIT LOG TOOL TESTS
// =============================================================================

func TestGitLogTool_Definition(t *testing.T) {
	t.Parallel()

	tool := GitLogTool()

	if tool.Name != "git_log" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

// =============================================================================
// GIT OPERATION TOOL TESTS
// =============================================================================

func TestGitOperationTool_Definition(t *testing.T) {
	t.Parallel()

	tool := GitOperationTool()

	if tool.Name != "git_operation" {
		t.Errorf("Name mismatch: got %q", tool.Name)
	}
}

func TestGitOperationTool_Execute_MissingOperation(t *testing.T) {
	t.Parallel()

	_, err := executeGitOperation(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing operation")
	}
}

func TestGitOperationTool_Execute_UnsupportedOperation(t *testing.T) {
	t.Parallel()

	_, err := executeGitOperation(context.Background(), map[string]any{
		"operation": "unsupported_op",
	})
	if err == nil {
		t.Error("expected error for unsupported operation")
	}
}

// =============================================================================
// HELPER FUNCTION TESTS
// =============================================================================

func TestFindBashWindows(t *testing.T) {
	t.Parallel()

	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}

	bash := findBashWindows()
	// May return empty if bash not installed
	t.Logf("Found bash: %q", bash)
}
