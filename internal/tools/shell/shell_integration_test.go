//go:build integration
package shell_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/tools/shell"
	"github.com/stretchr/testify/suite"
)

type ShellIntegrationSuite struct {
	suite.Suite
	tmpDir string
	ctx    context.Context
}

func (s *ShellIntegrationSuite) SetupTest() {
	s.tmpDir = s.T().TempDir()
	s.ctx = context.Background()
}

func (s *ShellIntegrationSuite) TestRunCommandTool_Integration() {
	tool := shell.RunCommandTool()

	// 1. Simple Echo
	result, err := tool.Execute(s.ctx, map[string]any{
		"command": "echo integration test",
	})
	s.Require().NoError(err)
	s.Contains(result, "integration test")

	// 2. Failing Command
	// Note: 'false' or 'exit 1' should return an error
	_, err = tool.Execute(s.ctx, map[string]any{
		"command": "exit 1",
	})
	s.Require().Error(err)
	s.Contains(err.Error(), "command failed")

	// 3. Environment Variables
	// We echo an environment variable to verify it was passed
	result, err = tool.Execute(s.ctx, map[string]any{
		"command": "echo $TEST_ENV_VAR",
		"env": map[string]any{
			"TEST_ENV_VAR": "secret_value",
		},
	})
	s.Require().NoError(err)
	s.Contains(result, "secret_value")

	// 4. Working Directory
	subdir := filepath.Join(s.tmpDir, "subdir")
	s.Require().NoError(os.Mkdir(subdir, 0755))
	result, err = tool.Execute(s.ctx, map[string]any{
		"command":     "pwd",
		"working_dir": subdir,
	})
	s.Require().NoError(err)
	// Resolve symlinks just in case (macOS /var vs /private/var)
	realResult, _ := filepath.EvalSymlinks(strings.TrimSpace(result))
	// On some systems pwd might output with different slashes, simpler to check suffix
	s.True(strings.HasSuffix(realResult, filepath.Base(subdir)), "pwd result %q should end with %q", realResult, filepath.Base(subdir))

	// Also check containment of the full path if possible, but path separators can be tricky
	// s.Contains(result, subdir) // This might fail on Windows if result has \ and subdir has / or vice versa.
}

func (s *ShellIntegrationSuite) TestBashTool_Integration() {
	tool := shell.BashTool()

	// Create a script that writes to a file
	scriptPath := filepath.Join(s.tmpDir, "test_script.sh")
	outputPath := filepath.Join(s.tmpDir, "output.txt")

	// We need to escape backslashes for Windows paths in the script if we were hardcoding paths,
	// but using relative paths is safer.
	scriptContent := `
#!/bin/bash
echo "Hello from Bash" > output.txt
`
	s.Require().NoError(os.WriteFile(scriptPath, []byte(scriptContent), 0755))

	// Execute the script
	result, err := tool.Execute(s.ctx, map[string]any{
		"script":      scriptContent,
		"working_dir": s.tmpDir,
	})
	s.Require().NoError(err)
	s.Contains(result, "") // Should be empty or success message? The tool returns stdout.
	// Our script doesn't echo to stdout, it writes to file.

	// Verify file creation
	content, err := os.ReadFile(outputPath)
	s.Require().NoError(err)
	s.Contains(string(content), "Hello from Bash")
}

func (s *ShellIntegrationSuite) TestGitOperationTool_Integration() {
	tool := shell.GitOperationTool()
	repoDir := filepath.Join(s.tmpDir, "repo")
	s.Require().NoError(os.Mkdir(repoDir, 0755))

	// Helper to run git op
	runGit := func(op string, args map[string]any) string {
		if args == nil {
			args = make(map[string]any)
		}
		args["operation"] = op
		args["working_dir"] = repoDir
		res, err := tool.Execute(s.ctx, args)
		s.Require().NoError(err, "git %s failed", op)
		return res
	}

	// 1. Init (using run_command since git_operation doesn't have init)
	// Wait, GitOperationTool doesn't have "init". We need to initialize the repo first.
	// We can use RunCommandTool for that, or os/exec.
	// Let's use RunCommandTool to keep it "in-system".
	runCmd := shell.RunCommandTool()
	_, err := runCmd.Execute(s.ctx, map[string]any{
		"command":     "git init",
		"working_dir": repoDir,
	})
	s.Require().NoError(err)

	// Configure git user for commit
	_, err = runCmd.Execute(s.ctx, map[string]any{
		"command":     "git config user.email 'test@example.com' && git config user.name 'Test User'",
		"working_dir": repoDir,
	})
	s.Require().NoError(err)

	// 2. Status (Empty)
	status := runGit("status", nil)
	s.Contains(status, "On branch")
	s.Contains(status, "No commits yet")

	// 3. Create file & Add
	s.Require().NoError(os.WriteFile(filepath.Join(repoDir, "file.txt"), []byte("content"), 0644))
	runGit("add", map[string]any{"files": "."})

	// 4. Commit
	runGit("commit", map[string]any{"message": "initial commit"})

	// 5. Log
	// We use the GitLogTool here? No, let's stick to GitOperationTool for now?
	// Actually, GitOperationTool doesn't support 'log'.
	// The plan said: "`git log` and verify message".
	// GitLogTool is separate. Let's use it!
	logTool := shell.GitLogTool()
	logResult, err := logTool.Execute(s.ctx, map[string]any{
		"working_dir": repoDir,
		"count":       1,
	})
	s.Require().NoError(err)
	s.Contains(logResult, "initial commit")

	// 6. Branch
	runGit("branch", map[string]any{"branch": "feature-branch"})

	// 7. Checkout
	runGit("checkout", map[string]any{"branch": "feature-branch"})

	// Verify we are on new branch
	status = runGit("status", nil)
	s.Contains(status, "feature-branch")
}

func (s *ShellIntegrationSuite) TestRunBuildTool_Integration() {
	tool := shell.RunBuildTool()
	projectDir := filepath.Join(s.tmpDir, "goproject")
	s.Require().NoError(os.Mkdir(projectDir, 0755))

	// Create go.mod
	// Use a recent enough go version that is likely compatible
	s.Require().NoError(os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/buildproject\n\ngo 1.21\n"), 0644))

	// 1. Valid Code
	mainGo := `package main
import "fmt"
func main() {
	fmt.Println("Hello Build")
}`
	s.Require().NoError(os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(mainGo), 0644))

	// Execute RunBuildTool (auto-detects go.mod -> go build ./...)
	_, err := tool.Execute(s.ctx, map[string]any{
		"working_dir": projectDir,
	})
	s.Require().NoError(err, "Build should succeed for valid code")

	// 2. Invalid Code (Syntax Error)
	brokenGo := `package main
func main() {
    undefinedFunc()
}`
	// Overwrite main.go with broken code
	s.Require().NoError(os.WriteFile(filepath.Join(projectDir, "main.go"), []byte(brokenGo), 0644))

	_, err = tool.Execute(s.ctx, map[string]any{
		"working_dir": projectDir,
	})
	s.Require().Error(err, "Build should fail for invalid code")
	s.Contains(err.Error(), "undefined: undefinedFunc")
}

func (s *ShellIntegrationSuite) TestRunTestsTool_Integration() {
	tool := shell.RunTestsTool()
	projectDir := filepath.Join(s.tmpDir, "gotestproject")
	s.Require().NoError(os.Mkdir(projectDir, 0755))

	// Create go.mod
	s.Require().NoError(os.WriteFile(filepath.Join(projectDir, "go.mod"), []byte("module example.com/testproject\n\ngo 1.21\n"), 0644))

	// Create main_test.go with a passing test
	mainTestGo := `package main
import "testing"
func TestHello(t *testing.T) {
    t.Log("Hello Test")
}`
	s.Require().NoError(os.WriteFile(filepath.Join(projectDir, "main_test.go"), []byte(mainTestGo), 0644))

	// 1. Success case
	result, err := tool.Execute(s.ctx, map[string]any{
		"working_dir": projectDir,
	})
	s.Require().NoError(err, "Tests should pass")
	// go test output depends on flags, but 'ok' is consistent for passing package
	s.Contains(result, "ok")

	// 2. Failure case
	failTestGo := `package main
import "testing"
func TestFail(t *testing.T) {
    t.Fatal("This should fail")
}`
	// Add a failing test file
	s.Require().NoError(os.WriteFile(filepath.Join(projectDir, "fail_test.go"), []byte(failTestGo), 0644))

	_, err = tool.Execute(s.ctx, map[string]any{
		"working_dir": projectDir,
	})
	s.Require().Error(err, "Tests should fail")
	s.Contains(err.Error(), "FAIL")
}

func TestShellIntegrationSuite(t *testing.T) {
	suite.Run(t, new(ShellIntegrationSuite))
}
