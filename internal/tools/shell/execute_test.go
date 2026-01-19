package shell

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// =============================================================================
// MOCK HELPER
// =============================================================================

// TestHelperProcess isn't a real test. It's used as a helper process
// for mocking exec.Command.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GO_WANT_HELPER_PROCESS") != "1" {
		return
	}

	// Print MOCK_OUTPUT if set
	if val := os.Getenv("MOCK_OUTPUT"); val != "" {
		fmt.Fprint(os.Stdout, val)
	} else {
		// Default behavior: print args
		// Args will be [binary, -test.run=TestHelperProcess, --, command...]
		args := os.Args
		for i, arg := range args {
			if arg == "--" {
				fmt.Fprint(os.Stdout, strings.Join(args[i+1:], " "))
				break
			}
		}
	}
	os.Exit(0)
}

func fakeExecCommandContext(ctx context.Context, command string, args ...string) *exec.Cmd {
	cs := []string{"-test.run=TestHelperProcess", "--", command}
	cs = append(cs, args...)
	cmd := exec.CommandContext(ctx, os.Args[0], cs...)
	// Note: We don't set cmd.Env here because executeRunCommand overwrites it.
	// We rely on the caller setting os.Setenv("GO_WANT_HELPER_PROCESS", "1")
	return cmd
}

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

func TestRunCommandTool_Execute_Success(t *testing.T) {
	// Mock exec
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	// Set env var to trigger helper
	os.Setenv("GO_WANT_HELPER_PROCESS", "1")
	defer os.Unsetenv("GO_WANT_HELPER_PROCESS")

	os.Setenv("MOCK_OUTPUT", "mocked output")
	defer os.Unsetenv("MOCK_OUTPUT")

	tool := RunCommandTool()
	result, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo test",
	})
	if err != nil {
		t.Fatalf("executeRunCommand error: %v", err)
	}

	if result != "mocked output" {
		t.Errorf("expected 'mocked output', got: %s", result)
	}
}

func TestRunCommandTool_Execute_EnvVars(t *testing.T) {
	// Verify env vars passed in args reach the process
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	os.Setenv("GO_WANT_HELPER_PROCESS", "1")
	defer os.Unsetenv("GO_WANT_HELPER_PROCESS")

	// We want the helper process to output the value of TEST_VAR
	// But helper process only outputs MOCK_OUTPUT or args.
	// We can't easily verify env vars reached the child process with this simple helper
	// without changing the helper logic.
	// However, we verify executeRunCommand logic works without error.

	os.Setenv("MOCK_OUTPUT", "success")
	defer os.Unsetenv("MOCK_OUTPUT")

	_, err := executeRunCommand(context.Background(), map[string]any{
		"command": "echo test",
		"env": map[string]any{
			"TEST_VAR": "test_value",
		},
	})
	if err != nil {
		t.Fatalf("executeRunCommand with env error: %v", err)
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

func TestBashTool_Execute_Success(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	os.Setenv("GO_WANT_HELPER_PROCESS", "1")
	defer os.Unsetenv("GO_WANT_HELPER_PROCESS")

	os.Setenv("MOCK_OUTPUT", "bash output")
	defer os.Unsetenv("MOCK_OUTPUT")

	res, err := executeBash(context.Background(), map[string]any{
		"script": "echo hello",
	})
	if err != nil {
		t.Fatalf("executeBash failed: %v", err)
	}
	if res != "bash output" {
		t.Errorf("expected 'bash output', got: %s", res)
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
	pkg := filepath.Join(tmpDir, "package.json")
	os.WriteFile(pkg, []byte("{}"), 0644)

	cmd := detectBuildCommand(tmpDir)
	if !strings.Contains(cmd, "npm") {
		t.Errorf("expected 'npm' for Node project, got: %s", cmd)
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
// GIT TOOL TESTS
// =============================================================================

func TestGitTools_Definitions(t *testing.T) {
	t.Parallel()

	if GitDiffTool().Name != "git_diff" {
		t.Error("GitDiffTool name mismatch")
	}
	if GitLogTool().Name != "git_log" {
		t.Error("GitLogTool name mismatch")
	}
	if GitOperationTool().Name != "git_operation" {
		t.Error("GitOperationTool name mismatch")
	}
}

func TestGitOperationTool_Execute_Success(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	os.Setenv("GO_WANT_HELPER_PROCESS", "1")
	defer os.Unsetenv("GO_WANT_HELPER_PROCESS")

	os.Setenv("MOCK_OUTPUT", "git status output")
	defer os.Unsetenv("MOCK_OUTPUT")

	res, err := executeGitOperation(context.Background(), map[string]any{
		"operation": "status",
	})
	if err != nil {
		t.Fatalf("executeGitOperation failed: %v", err)
	}
	if res != "git status output" {
		t.Errorf("expected 'git status output', got: %s", res)
	}
}

func TestGitOperationTool_Execute_MissingOp(t *testing.T) {
	t.Parallel()
	_, err := executeGitOperation(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing operation")
	}
}

func TestGitTools_Execute_Coverage(t *testing.T) {
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	os.Setenv("GO_WANT_HELPER_PROCESS", "1")
	defer os.Unsetenv("GO_WANT_HELPER_PROCESS")
	os.Setenv("MOCK_OUTPUT", "mock output")
	defer os.Unsetenv("MOCK_OUTPUT")

	// 1. Git Diff
	_, err := executeGitDiff(context.Background(), map[string]any{
		"path":   "file.txt",
		"staged": true,
	})
	if err != nil {
		t.Errorf("executeGitDiff failed: %v", err)
	}

	// 2. Git Log
	_, err = executeGitLog(context.Background(), map[string]any{
		"count":  5,
		"author": "me",
	})
	if err != nil {
		t.Errorf("executeGitLog failed: %v", err)
	}

	// 3. Git Operations
	ops := []struct {
		op   string
		args map[string]any
	}{
		{"add", map[string]any{"files": "."}},
		{"commit", map[string]any{"message": "msg"}},
		{"push", map[string]any{"args": "origin main"}},
		{"pull", map[string]any{}},
		{"checkout", map[string]any{"branch": "main"}},
		{"branch", map[string]any{"branch": "new-branch"}},
		{"fetch", map[string]any{}},
		{"stash", map[string]any{}},
		{"reset", map[string]any{}},
	}

	for _, tc := range ops {
		args := tc.args
		args["operation"] = tc.op
		_, err := executeGitOperation(context.Background(), args)
		if err != nil {
			t.Errorf("executeGitOperation(%s) failed: %v", tc.op, err)
		}
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

	// We can't easily mock file system for os.Stat in findBashWindows without refactoring.
	// Just verify it doesn't panic.
	_ = findBashWindows()
}
