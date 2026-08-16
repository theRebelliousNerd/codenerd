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
	// TODO: Edge Case - Null/Undefined/Empty: Test with explicitly nil map arguments.
	// TODO: Edge Case - Type Coercion: Test with non-string types for command (e.g., args["command"] = 123).
	// TODO: Edge Case - Null/Undefined/Empty: Test with empty command string.
	t.Parallel()

	_, err := executeRunCommand(context.Background(), map[string]any{})
	if err == nil {
		t.Error("expected error for missing command")
	}
}

func TestRunCommandTool_Execute_Success(t *testing.T) {
	// TODO: Edge Case - State Conflict: Test concurrent executions of compound commands to ensure no race conditions on shared temporary files or environment variables.
	// TODO: Edge Case - Extreme: Test with an extremely large command string to verify ARG_MAX limits and buffer handling.
	// TODO: Edge Case - State Conflict: Test execution when the shell binary (pwsh) is suddenly removed or permissions altered.
	// Mock exec
	oldExec := execCommandContext
	execCommandContext = fakeExecCommandContext
	defer func() { execCommandContext = oldExec }()

	// Pin the PATH lookup as well, or this test measures the terminal it was
	// launched from rather than the code.
	//
	// executeRunCommand consults execLookPath first: when the binary is absent
	// it serves a Go builtin (builtins.go handles echo, ls, cat, wc, grep, ...)
	// and returns BEFORE execCommandContext is ever called, so the mock above is
	// bypassed entirely. Windows has no echo.exe, but Git Bash puts one on PATH
	// — so this test passed under Git Bash and failed under PowerShell on the
	// same commit, reporting "got: test" from the real builtin.
	//
	// Reporting a successful lookup keeps the turn on the exec path this test is
	// actually about. The builtin fallback has its own tests.
	oldLookPath := execLookPath
	execLookPath = func(file string) (string, error) { return file, nil }
	defer func() { execLookPath = oldLookPath }()

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

	// Same reason as TestRunCommandTool_Execute_Success: without pinning the
	// lookup, an absent echo.exe diverts the turn into the Go builtin and the
	// exec mock is never reached. This test happens to pass either way today,
	// which is exactly why it is worth pinning now rather than after it breaks.
	oldLookPath := execLookPath
	execLookPath = func(file string) (string, error) { return file, nil }
	defer func() { execLookPath = oldLookPath }()

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
	// TODO: Edge Case - Null/Undefined/Empty: Test with explicitly nil map arguments.
	// TODO: Edge Case - Null/Undefined/Empty: Test with empty string for operation (e.g., args["operation"] = "").
	// TODO: Edge Case - Type Coercion: Test with non-string types for operation (e.g., args["operation"] = 123).
	// TODO: Edge Case - Extreme: Test with extremely large operation name string.
	// TODO: Edge Case - State Conflict: Test execution when the working directory has been deleted concurrently.
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

// =============================================================================
// IS-COMPOUND-COMMAND TESTS
// =============================================================================

func TestIsCompoundCommand(t *testing.T) {
	cases := []struct {
		name     string
		input    string
		expected bool
	}{
		{"simple", "echo hello", false},
		{"and_and", "echo a && echo b", true},
		{"or_or", "false || echo ok", true},
		{"pipe", "echo a | grep a", true},
		{"semicolon", "echo a; echo b", true},
		{"newline", "echo a\necho b", true},
		{"redirect_in", "cat < file.txt", true},
		{"redirect_out", "echo hi > file.txt", true},
		{"stderr_redirect", "echo hi 2>&1", true},
		{"quoted_and", "echo 'a && b'", false},
		{"quoted_pipe", "echo \"a | b\"", false},
		{"quoted_semicolon", "echo 'a; b'", false},
		{"quoted_redirect", "echo 'a > b'", false},
		{"quoted_newline_single", "echo 'a\nb'", false},
		{"quoted_regex_pipe", "grep 'a|b' file.txt", false},
		{"quoted_regex_pipe_double", "grep \"a|b\" file.txt", false},
		{"backslash_escaped_single", "echo 'a\\' && b'", false},
		{"backslash_escaped_double", "echo \"a\\\" && b\"", false},
		{"backtick_escaped_single", "echo 'a`'b'", false},
		{"backtick_escaped_double", "echo \"a`\"b\"", false},
		{"carriage_return", "echo a\recho b", true},
		{"crlf_newline", "echo a\r\necho b", true},
		{"mixed_quoted_and_real", "echo 'a && b' && echo c", true},
		{"empty", "", false},
		{"lone_ampersand", "echo a & echo b", false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := isCompoundCommand(tc.input)
			if got != tc.expected {
				t.Errorf("isCompoundCommand(%q) = %v, want %v", tc.input, got, tc.expected)
			}
		})
	}
}

// =============================================================================
// RUN COMMAND COMPOUND ROUTING TESTS
// =============================================================================

func TestRunCommandTool_CompoundRouting(t *testing.T) {
	cases := []struct {
		name        string
		command     string
		wantShell   bool
		wantArgsSub string
	}{
		{"simple_no_shell", "echo hello", false, ""},
		{"quoted_operator_no_shell", "echo 'a && b'", false, ""},
		{"quoted_pipe_no_shell", "echo \"a | b\"", false, ""},
		{"and_and_shell", "echo a && echo b", true, "echo a && echo b"},
		{"pipe_shell", "echo a | cat", true, "echo a | cat"},
		{"semicolon_shell", "echo a; echo b", true, "echo a; echo b"},
		{"newline_shell", "echo a\necho b", true, "echo a"},
		{"redirect_shell", "echo hi > /tmp/x", true, "echo hi > /tmp/x"},
		{"stderr_redirect_shell", "echo hi 2>&1", true, "echo hi 2>&1"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var gotCmd string
			var gotArgs []string
			oldExec := execCommandContext
			execCommandContext = func(ctx context.Context, command string, args ...string) *exec.Cmd {
				gotCmd = command
				gotArgs = args
				return fakeExecCommandContext(ctx, command, args...)
			}
			defer func() { execCommandContext = oldExec }()

			oldLookPath := execLookPath
			execLookPath = func(file string) (string, error) { return file, nil }
			defer func() { execLookPath = oldLookPath }()

			os.Setenv("GO_WANT_HELPER_PROCESS", "1")
			defer os.Unsetenv("GO_WANT_HELPER_PROCESS")
			os.Setenv("MOCK_OUTPUT", "ok")
			defer os.Unsetenv("MOCK_OUTPUT")

			_, err := executeRunCommand(context.Background(), map[string]any{
				"command": tc.command,
			})
			if err != nil {
				t.Fatalf("executeRunCommand error: %v", err)
			}

			isShell := gotCmd == "sh" || gotCmd == "pwsh" || gotCmd == "powershell" || strings.Contains(gotCmd, "pwsh") || strings.Contains(gotCmd, "powershell")
			// On non-Windows the shell path is "sh"; on Windows it is pwsh/powershell.
			// Accept either as "shell routed".
			if tc.wantShell && !isShell {
				t.Errorf("expected shell routing for %q, got cmd %q args %v", tc.command, gotCmd, gotArgs)
			}
			if !tc.wantShell && isShell {
				t.Errorf("expected direct exec for %q, got shell cmd %q args %v", tc.command, gotCmd, gotArgs)
			}
			if tc.wantShell && tc.wantArgsSub != "" {
				joined := strings.Join(gotArgs, " ")
				if !strings.Contains(joined, tc.wantArgsSub) {
					t.Errorf("expected args to contain %q, got %v", tc.wantArgsSub, gotArgs)
				}
			}
			if tc.wantShell {
				// Verify -c or -Command is present in args
				hasFlag := false
				for _, a := range gotArgs {
					if a == "-c" || a == "-Command" {
						hasFlag = true
						break
					}
				}
				if !hasFlag {
					t.Errorf("expected shell flag -c or -Command in args, got %v", gotArgs)
				}
			}
		})
	}
}

func TestRunCommandTool_WindowsCompoundRegression(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("Windows-only test")
	}
	if _, err := exec.LookPath("pwsh"); err != nil {
		t.Skip("pwsh not available")
	}
	result, err := executeRunCommand(context.Background(), map[string]any{
		"command": "Write-Output MARKER_ONE && Write-Output MARKER_TWO",
	})
	if err != nil {
		t.Fatalf("executeRunCommand error: %v", err)
	}
	if !strings.Contains(result, "MARKER_ONE") {
		t.Errorf("expected MARKER_ONE in output, got %q", result)
	}
	if !strings.Contains(result, "MARKER_TWO") {
		t.Errorf("expected MARKER_TWO in output, got %q", result)
	}
}
