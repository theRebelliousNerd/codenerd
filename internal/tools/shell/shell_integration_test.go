//go:build integration
package shell_test

import (
	"context"
	"os/exec"
	"strings"
	"testing"

	"codenerd/internal/tools/shell"
	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"
)

func TestMain(m *testing.M) {
	goleak.VerifyTestMain(m)
}

func TestBashTool_Integration(t *testing.T) {
	t.Parallel()

	// 1. Create a temporary directory
	tempDir := t.TempDir()

	// 2. Instantiate the tool
	tool := shell.BashTool()

	// 3. Execute a script
	script := "echo 'hello from bash'"
	output, err := tool.Execute(context.Background(), map[string]any{
		"script":      script,
		"working_dir": tempDir,
	})

	// 4. Verify results
	require.NoError(t, err)
	require.Contains(t, output, "hello from bash")
}

func TestRunCommandTool_Integration(t *testing.T) {
	t.Parallel()

	// 1. Instantiate the tool
	tool := shell.RunCommandTool()

	// 2. Execute a command
	output, err := tool.Execute(context.Background(), map[string]any{
		"command": "echo 'hello from command'",
	})

	// 3. Verify results
	require.NoError(t, err)
	require.Contains(t, output, "hello from command")
}

func TestGitOperationTool_Integration(t *testing.T) {
	t.Parallel()

	// 1. Create a temporary directory for the git repo
	tempDir := t.TempDir()

	// 2. Initialize a git repo using real git command (setup)
	cmd := exec.Command("git", "init")
	cmd.Dir = tempDir
	err := cmd.Run()
	require.NoError(t, err, "failed to git init")

	// 3. Configure user for git (needed in some envs)
	cmd = exec.Command("git", "config", "user.email", "test@example.com")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())
	cmd = exec.Command("git", "config", "user.name", "Test User")
	cmd.Dir = tempDir
	require.NoError(t, cmd.Run())

	// 4. Instantiate the tool
	tool := shell.GitOperationTool()

	// 5. Execute git status
	output, err := tool.Execute(context.Background(), map[string]any{
		"operation":   "status",
		"working_dir": tempDir,
	})

	// 6. Verify results
	require.NoError(t, err)
	// Output should indicate branch and no commits
	// Note: output wording depends on git version but usually contains "On branch" or "nothing to commit"
	require.True(t, strings.Contains(output, "On branch") || strings.Contains(output, "No commits yet") || strings.Contains(output, "nothing to commit"), "Unexpected git status output: %s", output)
}
