package shell

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)

// RunCommandTool returns a tool for executing shell commands.
func RunCommandTool() *tools.Tool {
	return &tools.Tool{
		Name:        "run_command",
		Description: "Execute a shell command and return its output",
		Category:    tools.CategoryCode,
		Priority:    70,
		Execute:     executeRunCommand,
		Schema: tools.ToolSchema{
			Required: []string{"command"},
			Properties: map[string]tools.Property{
				"command": {
					Type:        "string",
					Description: "The command to execute",
				},
				"working_dir": {
					Type:        "string",
					Description: "Working directory for the command",
				},
				"timeout_seconds": {
					Type:        "integer",
					Description: "Timeout in seconds (default: 60)",
					Default:     60,
				},
				"env": {
					Type:        "object",
					Description: "Additional environment variables",
				},
			},
		},
	}
}

func executeRunCommand(ctx context.Context, args map[string]any) (string, error) {
	command, _ := args["command"].(string)
	if command == "" {
		return "", fmt.Errorf("command is required")
	}

	workingDir := ""
	if wd, ok := args["working_dir"].(string); ok {
		workingDir = wd
	}

	timeout := 60
	if t, ok := args["timeout_seconds"].(int); ok && t > 0 {
		timeout = t
	}

	logging.VirtualStoreDebug("run_command: cmd=%s, dir=%s, timeout=%ds", command, workingDir, timeout)

	// Create command based on OS
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(ctx, "cmd", "/C", command)
	} else {
		cmd = exec.CommandContext(ctx, "sh", "-c", command)
	}

	if workingDir != "" {
		cmd.Dir = workingDir
	}

	// Set environment
	cmd.Env = os.Environ()
	if envMap, ok := args["env"].(map[string]any); ok {
		for k, v := range envMap {
			if vs, ok := v.(string); ok {
				cmd.Env = append(cmd.Env, fmt.Sprintf("%s=%s", k, vs))
			}
		}
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()
	cmd = exec.CommandContext(execCtx, cmd.Path, cmd.Args[1:]...)
	cmd.Dir = workingDir
	cmd.Env = os.Environ()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n--- stderr ---\n"
		}
		output += stderr.String()
	}

	// Truncate if too long
	if len(output) > 50000 {
		output = output[:50000] + "\n...[truncated]"
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("command timed out after %d seconds", timeout)
		}
		logging.VirtualStore("run_command failed: %s (%v)", command, err)
		return output, fmt.Errorf("command failed: %w\nOutput:\n%s", err, output)
	}

	logging.VirtualStore("run_command completed: %s (%d bytes output)", command, len(output))
	return output, nil
}

// BashTool returns a tool for executing bash scripts.
func BashTool() *tools.Tool {
	return &tools.Tool{
		Name:        "bash",
		Description: "Execute a bash script",
		Category:    tools.CategoryCode,
		Priority:    70,
		Execute:     executeBash,
		Schema: tools.ToolSchema{
			Required: []string{"script"},
			Properties: map[string]tools.Property{
				"script": {
					Type:        "string",
					Description: "The bash script to execute",
				},
				"working_dir": {
					Type:        "string",
					Description: "Working directory for the script",
				},
				"timeout_seconds": {
					Type:        "integer",
					Description: "Timeout in seconds (default: 60)",
					Default:     60,
				},
			},
		},
	}
}

func executeBash(ctx context.Context, args map[string]any) (string, error) {
	script, _ := args["script"].(string)
	if script == "" {
		return "", fmt.Errorf("script is required")
	}

	// On Windows, try to use Git Bash or WSL
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Try Git Bash first
		bashPath := findBashWindows()
		if bashPath != "" {
			cmd = exec.CommandContext(ctx, bashPath, "-c", script)
		} else {
			// Fall back to cmd with basic interpretation
			return executeRunCommand(ctx, map[string]any{
				"command":         script,
				"working_dir":     args["working_dir"],
				"timeout_seconds": args["timeout_seconds"],
			})
		}
	} else {
		cmd = exec.CommandContext(ctx, "bash", "-c", script)
	}

	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		cmd.Dir = wd
	}

	timeout := 60
	if t, ok := args["timeout_seconds"].(int); ok && t > 0 {
		timeout = t
	}

	logging.VirtualStoreDebug("bash: script_len=%d, timeout=%ds", len(script), timeout)

	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String()
	if stderr.Len() > 0 {
		if output != "" {
			output += "\n--- stderr ---\n"
		}
		output += stderr.String()
	}

	if len(output) > 50000 {
		output = output[:50000] + "\n...[truncated]"
	}

	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("script timed out after %d seconds", timeout)
		}
		return output, fmt.Errorf("script failed: %w", err)
	}

	logging.VirtualStore("bash completed: (%d bytes output)", len(output))
	return output, nil
}

// findBashWindows finds a bash executable on Windows.
func findBashWindows() string {
	// Common locations for Git Bash
	paths := []string{
		"C:\\Program Files\\Git\\bin\\bash.exe",
		"C:\\Program Files (x86)\\Git\\bin\\bash.exe",
		os.Getenv("LOCALAPPDATA") + "\\Programs\\Git\\bin\\bash.exe",
	}

	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			return p
		}
	}

	// Try to find in PATH
	if path, err := exec.LookPath("bash"); err == nil {
		return path
	}

	return ""
}

// RunBuildTool returns a tool for running project builds.
func RunBuildTool() *tools.Tool {
	return &tools.Tool{
		Name:        "run_build",
		Description: "Run the project build command",
		Category:    tools.CategoryCode,
		Priority:    75,
		Execute:     executeRunBuild,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"working_dir": {
					Type:        "string",
					Description: "Project directory (default: current directory)",
				},
				"command": {
					Type:        "string",
					Description: "Custom build command (auto-detected if not specified)",
				},
				"timeout_seconds": {
					Type:        "integer",
					Description: "Timeout in seconds (default: 300)",
					Default:     300,
				},
			},
		},
	}
}

func executeRunBuild(ctx context.Context, args map[string]any) (string, error) {
	workingDir := "."
	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		workingDir = wd
	}

	command, _ := args["command"].(string)
	if command == "" {
		// Auto-detect build command
		command = detectBuildCommand(workingDir)
		if command == "" {
			return "", fmt.Errorf("could not detect build command, please specify one")
		}
	}

	logging.VirtualStoreDebug("run_build: cmd=%s, dir=%s", command, workingDir)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     workingDir,
		"timeout_seconds": args["timeout_seconds"],
	})
}

// detectBuildCommand detects the appropriate build command for a project.
func detectBuildCommand(dir string) string {
	// Check for various build files
	checks := []struct {
		file    string
		command string
	}{
		{"go.mod", "go build ./..."},
		{"Cargo.toml", "cargo build"},
		{"package.json", "npm run build"},
		{"Makefile", "make"},
		{"build.gradle", "./gradlew build"},
		{"pom.xml", "mvn package"},
		{"CMakeLists.txt", "cmake --build ."},
		{"setup.py", "python setup.py build"},
		{"pyproject.toml", "python -m build"},
	}

	for _, check := range checks {
		if _, err := os.Stat(dir + "/" + check.file); err == nil {
			return check.command
		}
	}

	return ""
}

// RunTestsTool returns a tool for running project tests.
func RunTestsTool() *tools.Tool {
	return &tools.Tool{
		Name:        "run_tests",
		Description: "Run the project test suite",
		Category:    tools.CategoryTest,
		Priority:    75,
		Execute:     executeRunTests,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"working_dir": {
					Type:        "string",
					Description: "Project directory (default: current directory)",
				},
				"command": {
					Type:        "string",
					Description: "Custom test command (auto-detected if not specified)",
				},
				"pattern": {
					Type:        "string",
					Description: "Test pattern/filter to run specific tests",
				},
				"timeout_seconds": {
					Type:        "integer",
					Description: "Timeout in seconds (default: 600)",
					Default:     600,
				},
			},
		},
	}
}

func executeRunTests(ctx context.Context, args map[string]any) (string, error) {
	workingDir := "."
	if wd, ok := args["working_dir"].(string); ok && wd != "" {
		workingDir = wd
	}

	command, _ := args["command"].(string)
	pattern, _ := args["pattern"].(string)

	if command == "" {
		// Auto-detect test command
		command = detectTestCommand(workingDir)
		if command == "" {
			return "", fmt.Errorf("could not detect test command, please specify one")
		}
	}

	// Add pattern if specified
	if pattern != "" {
		command = addTestPattern(command, pattern)
	}

	logging.VirtualStoreDebug("run_tests: cmd=%s, dir=%s", command, workingDir)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     workingDir,
		"timeout_seconds": args["timeout_seconds"],
	})
}

// detectTestCommand detects the appropriate test command for a project.
func detectTestCommand(dir string) string {
	checks := []struct {
		file    string
		command string
	}{
		{"go.mod", "go test ./..."},
		{"Cargo.toml", "cargo test"},
		{"package.json", "npm test"},
		{"pytest.ini", "pytest"},
		{"setup.py", "python -m pytest"},
		{"pyproject.toml", "pytest"},
		{"build.gradle", "./gradlew test"},
		{"pom.xml", "mvn test"},
	}

	for _, check := range checks {
		if _, err := os.Stat(dir + "/" + check.file); err == nil {
			return check.command
		}
	}

	return ""
}

// addTestPattern adds a test pattern to the command.
func addTestPattern(command, pattern string) string {
	if strings.HasPrefix(command, "go test") {
		return command + " -run " + pattern
	}
	if strings.HasPrefix(command, "pytest") {
		return command + " -k " + pattern
	}
	if strings.HasPrefix(command, "npm test") {
		return command + " -- --grep " + pattern
	}
	if strings.HasPrefix(command, "cargo test") {
		return command + " " + pattern
	}
	return command + " " + pattern
}

// GitDiffTool returns a tool for viewing git diffs.
func GitDiffTool() *tools.Tool {
	return &tools.Tool{
		Name:        "git_diff",
		Description: "Show git diff for files or commits",
		Category:    tools.CategoryCode,
		Priority:    70,
		Execute:     executeGitDiff,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "File or directory path to diff (optional)",
				},
				"staged": {
					Type:        "boolean",
					Description: "Show staged changes only (--cached)",
					Default:     false,
				},
				"commit": {
					Type:        "string",
					Description: "Compare against specific commit or range (e.g., HEAD~3, main..feature)",
				},
				"working_dir": {
					Type:        "string",
					Description: "Working directory (default: current directory)",
				},
			},
		},
	}
}

func executeGitDiff(ctx context.Context, args map[string]any) (string, error) {
	cmdArgs := []string{"diff"}

	// Add --cached for staged changes
	if staged, ok := args["staged"].(bool); ok && staged {
		cmdArgs = append(cmdArgs, "--cached")
	}

	// Add commit reference
	if commit, ok := args["commit"].(string); ok && commit != "" {
		cmdArgs = append(cmdArgs, commit)
	}

	// Add path
	if path, ok := args["path"].(string); ok && path != "" {
		cmdArgs = append(cmdArgs, "--", path)
	}

	command := "git " + strings.Join(cmdArgs, " ")

	logging.VirtualStoreDebug("git_diff: cmd=%s", command)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     args["working_dir"],
		"timeout_seconds": 60,
	})
}

// GitLogTool returns a tool for viewing git history.
func GitLogTool() *tools.Tool {
	return &tools.Tool{
		Name:        "git_log",
		Description: "Show git commit history",
		Category:    tools.CategoryCode,
		Priority:    70,
		Execute:     executeGitLog,
		Schema: tools.ToolSchema{
			Required: []string{},
			Properties: map[string]tools.Property{
				"path": {
					Type:        "string",
					Description: "File or directory path to show history for (optional)",
				},
				"count": {
					Type:        "integer",
					Description: "Number of commits to show (default: 10)",
					Default:     10,
				},
				"format": {
					Type:        "string",
					Description: "Output format: oneline, short, medium, full (default: medium)",
					Default:     "medium",
				},
				"since": {
					Type:        "string",
					Description: "Show commits since date (e.g., '1 week ago', '2024-01-01')",
				},
				"author": {
					Type:        "string",
					Description: "Filter by author name or email",
				},
				"working_dir": {
					Type:        "string",
					Description: "Working directory (default: current directory)",
				},
			},
		},
	}
}

func executeGitLog(ctx context.Context, args map[string]any) (string, error) {
	cmdArgs := []string{"log"}

	// Add count
	count := 10
	if c, ok := args["count"].(int); ok && c > 0 {
		count = c
	}
	if cf, ok := args["count"].(float64); ok && cf > 0 {
		count = int(cf)
	}
	cmdArgs = append(cmdArgs, fmt.Sprintf("-n%d", count))

	// Add format
	format := "medium"
	if f, ok := args["format"].(string); ok && f != "" {
		format = f
	}
	cmdArgs = append(cmdArgs, "--format="+format)

	// Add since filter
	if since, ok := args["since"].(string); ok && since != "" {
		cmdArgs = append(cmdArgs, "--since="+since)
	}

	// Add author filter
	if author, ok := args["author"].(string); ok && author != "" {
		cmdArgs = append(cmdArgs, "--author="+author)
	}

	// Add path
	if path, ok := args["path"].(string); ok && path != "" {
		cmdArgs = append(cmdArgs, "--", path)
	}

	command := "git " + strings.Join(cmdArgs, " ")

	logging.VirtualStoreDebug("git_log: cmd=%s", command)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     args["working_dir"],
		"timeout_seconds": 60,
	})
}

// GitOperationTool returns a tool for general git operations.
func GitOperationTool() *tools.Tool {
	return &tools.Tool{
		Name:        "git_operation",
		Description: "Execute git operations like status, add, commit, checkout, branch, push, pull",
		Category:    tools.CategoryCode,
		Priority:    70,
		Execute:     executeGitOperation,
		Schema: tools.ToolSchema{
			Required: []string{"operation"},
			Properties: map[string]tools.Property{
				"operation": {
					Type:        "string",
					Description: "Git operation: status, add, commit, checkout, branch, push, pull, fetch, stash, reset",
				},
				"args": {
					Type:        "string",
					Description: "Additional arguments for the operation (e.g., file paths, branch names, commit messages)",
				},
				"message": {
					Type:        "string",
					Description: "Commit message (for commit operation)",
				},
				"branch": {
					Type:        "string",
					Description: "Branch name (for checkout/branch operations)",
				},
				"files": {
					Type:        "string",
					Description: "Files to add/commit (space-separated, for add/commit operations)",
				},
				"working_dir": {
					Type:        "string",
					Description: "Working directory (default: current directory)",
				},
			},
		},
	}
}

func executeGitOperation(ctx context.Context, args map[string]any) (string, error) {
	operation, _ := args["operation"].(string)
	if operation == "" {
		return "", fmt.Errorf("operation is required")
	}

	var cmdArgs []string

	switch operation {
	case "status":
		cmdArgs = []string{"status"}
	case "add":
		cmdArgs = []string{"add"}
		if files, ok := args["files"].(string); ok && files != "" {
			cmdArgs = append(cmdArgs, strings.Fields(files)...)
		} else if extraArgs, ok := args["args"].(string); ok && extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		} else {
			cmdArgs = append(cmdArgs, ".") // Default to all
		}
	case "commit":
		cmdArgs = []string{"commit"}
		if msg, ok := args["message"].(string); ok && msg != "" {
			cmdArgs = append(cmdArgs, "-m", msg)
		} else {
			return "", fmt.Errorf("commit message is required")
		}
	case "checkout":
		cmdArgs = []string{"checkout"}
		if branch, ok := args["branch"].(string); ok && branch != "" {
			cmdArgs = append(cmdArgs, branch)
		} else if extraArgs, ok := args["args"].(string); ok && extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		} else {
			return "", fmt.Errorf("branch name or args required for checkout")
		}
	case "branch":
		cmdArgs = []string{"branch"}
		if branch, ok := args["branch"].(string); ok && branch != "" {
			cmdArgs = append(cmdArgs, branch)
		}
		if extraArgs, ok := args["args"].(string); ok && extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		}
	case "push":
		cmdArgs = []string{"push"}
		if extraArgs, ok := args["args"].(string); ok && extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		}
	case "pull":
		cmdArgs = []string{"pull"}
		if extraArgs, ok := args["args"].(string); ok && extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		}
	case "fetch":
		cmdArgs = []string{"fetch"}
		if extraArgs, ok := args["args"].(string); ok && extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		}
	case "stash":
		cmdArgs = []string{"stash"}
		if extraArgs, ok := args["args"].(string); ok && extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		}
	case "reset":
		cmdArgs = []string{"reset"}
		if extraArgs, ok := args["args"].(string); ok && extraArgs != "" {
			cmdArgs = append(cmdArgs, strings.Fields(extraArgs)...)
		}
	default:
		return "", fmt.Errorf("unsupported git operation: %s", operation)
	}

	command := "git " + strings.Join(cmdArgs, " ")

	logging.VirtualStoreDebug("git_operation: cmd=%s", command)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     args["working_dir"],
		"timeout_seconds": 120,
	})
}
