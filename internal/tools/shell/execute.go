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

	"github.com/kballard/go-shellquote"

	"codenerd/internal/logging"
	"codenerd/internal/tools"
)
// Variables for mocking in tests
var (
	execCommandContext = exec.CommandContext
	execLookPath       = exec.LookPath
)

// commandWaitDelay bounds how long [exec.Cmd.Wait] waits for I/O copying to
// finish after the context is cancelled or the process exits before it closes
// the pipes and returns. It bounds post-cancellation I/O wait, not command
// runtime (the context timeout bounds runtime). Without it, a grandchild that
// inherits the write end of the pipe can keep Wait blocked long after the
// deadline, because Wait waits for the background goroutine copying from the
// os.Pipe which waits for EOF.
const commandWaitDelay = 5 * time.Second

// newCommand creates a new exec.Cmd via execCommandContext and sets WaitDelay
// so that Wait does not block indefinitely on a pipe held open by a grandchild.
// All command construction in this file should go through this helper so the
// WaitDelay cannot drift out of sync between the three execution paths.
func newCommand(ctx context.Context, name string, arg ...string) *exec.Cmd {
	cmd := execCommandContext(ctx, name, arg...)
	cmd.WaitDelay = commandWaitDelay
	// WaitDelay only bounds how long Wait blocks on a pipe a grandchild holds;
	// it does not stop the grandchild. Cancellation must kill the tree.
	configureTreeKill(cmd)
	return cmd
}

// coerceInt accepts any of the shapes a JSON-decoded LLM tool argument can
// take and returns an int. See tools.CoerceInt — this package's copy was one
// of four that had drifted apart on which types they accepted.
func coerceInt(v any) (int, bool) {
	return tools.CoerceInt(v)
}

// resolveWorkingDir contains a caller-supplied working_dir to the workspace
// root, and resolves an omitted one to the workspace root rather than to the
// process working directory.
//
// Every tool in this file passed working_dir through to exec.Cmd.Dir unchecked,
// so `run_command working_dir=/ command="rm -rf tmp"` ran wherever it was
// pointed. The path guards in core/file_ops.go and codedom bounded what the
// agent could touch through the file tools; the shell tools sat next to them
// with no bound at all, which made the file-tool containment decorative — a
// contained agent could not write /etc/cron.d/x with write_file, but could
// cd there and do it with a shell one-liner.
//
// Defaulting an empty working_dir to the workspace root (rather than "" =
// process cwd) matters for the same reason relative tool paths are
// workspace-relative: -w/--workspace sets the root without chdir'ing, so cwd
// and workspace are not the same directory.
func resolveWorkingDir(ctx context.Context, raw string) (string, error) {
	root, err := tools.WorkspaceRoot(ctx)
	if err != nil {
		return "", err
	}
	dir, err := tools.ResolveWorkspaceDir(ctx, root, raw)
	if err != nil {
		return "", fmt.Errorf("working_dir rejected: %w", err)
	}
	return dir, nil
}

// isCompoundCommand reports whether s contains an unquoted shell operator and
// therefore has to run through a shell rather than a direct exec.
//
// It shares scanShell with commandStages so that the two can never disagree
// about what is quoted; see the note on scanShell for the drift this prevents.
func isCompoundCommand(s string) bool {
	compound := false
	scanShell(s, func(i int, c byte, quoted bool) bool {
		if quoted {
			return true
		}
		switch c {
		case ';', '\n', '\r', '<', '>', '|':
			compound = true
			return false
		case '&':
			if i+1 < len(s) && s[i+1] == '&' {
				compound = true
				return false
			}
		}
		return true
	})
	return compound
}

// RunCommandTool returns a tool for executing shell commands.
func RunCommandTool() *tools.Tool {
	return &tools.Tool{
		Name:          "run_command",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryAttack},
		Description:   runCommandDescription(),
		Category:      tools.CategoryCode,
		Priority:      70,
		Execute:       executeRunCommand,
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
					Description: "Timeout in seconds (default: 60; 600 when the command is a build or test toolchain invocation such as go test, go build, cargo test, npm test, make)",
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

	rawWorkingDir := ""
	if wd, ok := args["working_dir"].(string); ok {
		rawWorkingDir = wd
	}
	workingDir, err := resolveWorkingDir(ctx, rawWorkingDir)
	if err != nil {
		return "", err
	}

	timeout := defaultCommandTimeout(command)
	if t, ok := coerceInt(args["timeout_seconds"]); ok && t > 0 {
		timeout = t
	}

	logging.ToolsDebug("run_command: cmd=%s, dir=%s, timeout=%ds", command, workingDir, timeout)

	// Compound-command routing: if the command contains an unquoted shell
	// operator (&&, ||, |, ;, newline, <, >), execute via shell so the
	// operator is interpreted. Operators inside single or double quotes do
	// not trigger routing. On Windows route through pwsh then powershell
	// with -NoProfile -NonInteractive -Command; elsewhere via sh -c.
	// The upstream permission decision already gated this command, so this
	// only changes how it runs. Timeout, working directory, env, and output
	// bounds are preserved in both paths.
	if isCompoundCommand(command) {
		execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
		defer cancel()

		var cmd *exec.Cmd
		if runtime.GOOS == "windows" {
			// A model writes its pipelines with grep, head and wc. A
			// PowerShell-parented process cannot run any of them -- they are
			// not cmdlets and not on its inherited PATH -- so the command fails
			// outright and costs a whole turn to rediscover. The builtin
			// fallback below rescues these names for SIMPLE commands only;
			// anything containing a pipe is routed here first and never reaches
			// it. When a POSIX shell is installed, run the pipeline there and
			// it simply works.
			//
			// Only stages PowerShell cannot execute trigger the switch, so no
			// command that succeeds under PowerShell today changes path.
			if posix := posixOnlyStagesIn(command); len(posix) > 0 {
				if bashPath := findBashWindows(); bashPath != "" {
					logging.ToolsDebug(
						"run_command: routing to %s instead of PowerShell; POSIX-only stages: %s",
						bashPath, strings.Join(posix, ", "))
					cmd = newCommand(execCtx, bashPath, "-c", command)
				}
			}
			if cmd == nil {
				shellPath := ""
				if p, err := execLookPath("pwsh"); err == nil {
					shellPath = p
				} else if p, err := execLookPath("powershell"); err == nil {
					shellPath = p
				}
				if shellPath == "" {
					return "", fmt.Errorf("interpreter not found: neither pwsh nor powershell is available")
				}
				cmd = newCommand(execCtx, shellPath, "-NoProfile", "-NonInteractive", "-Command", command)
			}
		} else {
			cmd = newCommand(execCtx, "sh", "-c", command)
		}

		cmd.Dir = workingDir
		finalEnv := os.Environ()
		if envMap, ok := args["env"].(map[string]any); ok {
			for k, v := range envMap {
				if vs, ok := v.(string); ok {
					finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, vs))
				}
			}
		}
		cmd.Env = finalEnv

		var stdout, stderr bytes.Buffer
		cmd.Stdout = &stdout
		cmd.Stderr = &stderr

		runErr := cmd.Run()

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
		if runErr != nil {
			if execCtx.Err() == context.DeadlineExceeded {
				return output, fmt.Errorf("command timed out after %d seconds", timeout)
			}
			// A search that matched nothing is a result, not a failure. This
			// branch is the one that matters most: a model writes its searches
			// as pipelines, so a no-match almost always arrives here.
			if searchFoundNothing(command, exitCodeOf(runErr), stderr.String()) {
				logging.Tools("run_command: no matches: %s", command)
				return "(no matches)", nil
			}
			logging.Tools("run_command failed: %s (%v)", command, runErr)
			return output, fmt.Errorf("command failed: %w\nOutput:\n%s", runErr, output)
		}
		logging.Tools("run_command completed: %s (%d bytes output)", command, len(output))
		return output, nil
	}
	// Parse command safely using shellquote to prevent command injection
	parsedArgs, err := shellquote.Split(command)
	if err != nil {
		return "", fmt.Errorf("failed to parse command: %w", err)
	}
	if len(parsedArgs) == 0 {
		return "", fmt.Errorf("empty command after parsing")
	}

	// Cross-platform builtin fallback (F-CMD-2 / contract #4): if the requested
	// binary is not on PATH — common on Windows for unix coreutils like rg/ls/wc
	// that campaign checkpoint reviewers and shards habitually reach for — serve
	// a read-only Go implementation instead of hard-failing with
	// "exec: <cmd>: executable file not found". Only triggers when the real
	// binary is absent, so an installed tool always wins and behavior is
	// unchanged on systems that have the command.
	if _, lookErr := execLookPath(parsedArgs[0]); lookErr != nil {
		if out, handled := runBuiltinFallback(parsedArgs, workingDir); handled {
			logging.Tools("run_command builtin fallback served: %s", parsedArgs[0])
			return out, nil
		}
		// On Windows the model frequently emits PowerShell cmdlets
		// (Get-ChildItem, Select-String, Measure-Object, ...) which are not
		// standalone binaries, so exec fails with "executable file not found".
		// The unix builtins above cover tools PowerShell lacks (rg, wc); for
		// everything else, re-route the whole command through PowerShell so the
		// cmdlet actually runs with its native argument syntax. Only when the
		// requested command was NOT found (installed binaries still win) and
		// PowerShell is present. The command already passed the upstream
		// permission gate, so this changes how it runs, not whether it may.
		if runtime.GOOS == "windows" && isLikelyPowerShell(parsedArgs[0]) {
			shellPath := ""
			if p, err := execLookPath("pwsh"); err == nil {
				shellPath = p
			} else if p, err := execLookPath("powershell"); err == nil {
				shellPath = p
			}
			if shellPath != "" {
				parsedArgs = []string{shellPath, "-NoProfile", "-NonInteractive", "-Command", command}
				logging.Tools("run_command routing via PowerShell: %s", command)
			}
		}
	}

	// Build the timeout context BEFORE constructing the command so the
	// process is actually bound to the deadline (was previously built twice).
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	var cmd *exec.Cmd
	if len(parsedArgs) == 1 {
		cmd = newCommand(execCtx, parsedArgs[0])
	} else {
		cmd = newCommand(execCtx, parsedArgs[0], parsedArgs[1:]...)
	}

	cmd.Dir = workingDir

	// Prepare environment
	finalEnv := os.Environ()
	if envMap, ok := args["env"].(map[string]any); ok {
		for k, v := range envMap {
			if vs, ok := v.(string); ok {
				finalEnv = append(finalEnv, fmt.Sprintf("%s=%s", k, vs))
			}
		}
	}
	cmd.Env = finalEnv

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	runErr := cmd.Run()

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

	if runErr != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			return output, fmt.Errorf("command timed out after %d seconds", timeout)
		}
		// A search that matched nothing is a result, not a failure. Reporting
		// it as an error tells the model its tooling broke, and it then spends
		// turns re-running or routing around a search that worked.
		if searchFoundNothing(command, exitCodeOf(runErr), stderr.String()) {
			logging.Tools("run_command: no matches: %s", command)
			return "(no matches)", nil
		}
		logging.Tools("run_command failed: %s (%v)", command, runErr)
		return output, fmt.Errorf("command failed: %w\nOutput:\n%s", runErr, output)
	}

	logging.Tools("run_command completed: %s (%d bytes output)", command, len(output))
	return output, nil
}

// BashTool returns a tool for executing bash scripts.
func BashTool() *tools.Tool {
	return &tools.Tool{
		Name:          "bash",
		AltCategories: []tools.ToolCategory{tools.CategoryAttack},
		Description:   "Execute a bash script",
		Category:      tools.CategoryCode,
		Priority:      70,
		Execute:       executeBash,
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
					Description: "Timeout in seconds (default: 60; 600 when the command is a build or test toolchain invocation such as go test, go build, cargo test, npm test, make)",
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

	rawWorkingDir, _ := args["working_dir"].(string)
	workingDir, err := resolveWorkingDir(ctx, rawWorkingDir)
	if err != nil {
		return "", err
	}

	// A bash script's first line is the best available signal of what it
	// runs; a script that starts with `go test` deserves the toolchain default.
	timeout := defaultCommandTimeout(strings.SplitN(strings.TrimSpace(script), "\n", 2)[0])
	if t, ok := coerceInt(args["timeout_seconds"]); ok && t > 0 {
		timeout = t
	}

	// Build the timeout context BEFORE constructing the command so the
	// process is actually bound to the deadline.
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
	defer cancel()

	// On Windows, try to use Git Bash or WSL
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		bashPath := findBashWindows()
		if bashPath != "" {
			cmd = newCommand(execCtx, bashPath)
			cmd.Stdin = strings.NewReader(script)
		} else {
			// Fall back to cmd with basic interpretation
			return executeRunCommand(ctx, map[string]any{
				"command":         script,
				"working_dir":     workingDir,
				"timeout_seconds": args["timeout_seconds"],
			})
		}
	} else {
		cmd = newCommand(execCtx, "bash")
		cmd.Stdin = strings.NewReader(script)
	}

	cmd.Dir = workingDir

	logging.ToolsDebug("bash: script_len=%d, dir=%s, timeout=%ds", len(script), workingDir, timeout)

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err = cmd.Run()

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

	logging.Tools("bash completed: (%d bytes output)", len(output))
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
	if path, err := execLookPath("bash"); err == nil {
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
	rawWorkingDir, _ := args["working_dir"].(string)
	workingDir, err := resolveWorkingDir(ctx, rawWorkingDir)
	if err != nil {
		return "", err
	}

	command, _ := args["command"].(string)
	if command == "" {
		// Auto-detect build command
		command, _ = tools.BuildCommandForDir(workingDir)
		if command == "" {
			return "", fmt.Errorf("could not detect build command, please specify one")
		}
	}

	logging.ToolsDebug("run_build: cmd=%s, dir=%s", command, workingDir)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     workingDir,
		"timeout_seconds": args["timeout_seconds"],
	})
}

// RunTestsTool returns a tool for running project tests.
func RunTestsTool() *tools.Tool {
	return &tools.Tool{
		Name:          "run_tests",
		AltCategories: []tools.ToolCategory{tools.CategoryAttack},
		Description:   "Run the project test suite",
		Category:      tools.CategoryTest,
		Priority:      75,
		Execute:       executeRunTests,
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
	rawWorkingDir, _ := args["working_dir"].(string)
	workingDir, err := resolveWorkingDir(ctx, rawWorkingDir)
	if err != nil {
		return "", err
	}

	command, _ := args["command"].(string)
	pattern, _ := args["pattern"].(string)

	if command == "" {
		// Auto-detect test command
		command, _ = tools.TestCommandForDir(workingDir)
		if command == "" {
			return "", fmt.Errorf("could not detect test command, please specify one")
		}
	}

	// Add pattern if specified
	if pattern != "" {
		command = addTestPattern(command, pattern)
	}

	logging.ToolsDebug("run_tests: cmd=%s, dir=%s", command, workingDir)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     workingDir,
		"timeout_seconds": args["timeout_seconds"],
	})
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
		Name:          "git_diff",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral},
		Description:   "Show git diff for files or commits",
		Category:      tools.CategoryCode,
		Priority:      70,
		Execute:       executeGitDiff,
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

	// Add path. The pathspec is contained even though git would reject a path
	// outside the repository anyway: working_dir may legitimately be a nested
	// repo, and "outside the workspace but inside some repo" is exactly the
	// case a pathspec can reach and the working_dir guard cannot see.
	if path, ok := args["path"].(string); ok && path != "" {
		if _, err := tools.ResolveWorkspacePath(ctx, "", path); err != nil {
			return "", err
		}
		cmdArgs = append(cmdArgs, "--", path)
	}

	command := "git " + strings.Join(cmdArgs, " ")

	logging.ToolsDebug("git_diff: cmd=%s", command)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     args["working_dir"],
		"timeout_seconds": 60,
	})
}

// GitLogTool returns a tool for viewing git history.
func GitLogTool() *tools.Tool {
	return &tools.Tool{
		Name:          "git_log",
		AltCategories: []tools.ToolCategory{tools.CategoryReview, tools.CategoryGeneral},
		Description:   "Show git commit history",
		Category:      tools.CategoryCode,
		Priority:      70,
		Execute:       executeGitLog,
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
	if c, ok := coerceInt(args["count"]); ok && c > 0 {
		count = c
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
		if _, err := tools.ResolveWorkspacePath(ctx, "", path); err != nil {
			return "", err
		}
		cmdArgs = append(cmdArgs, "--", path)
	}

	command := "git " + strings.Join(cmdArgs, " ")

	logging.ToolsDebug("git_log: cmd=%s", command)

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
			cmdArgs = append(cmdArgs, "-m", fmt.Sprintf("%q", msg))
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

	logging.ToolsDebug("git_operation: cmd=%s", command)

	return executeRunCommand(ctx, map[string]any{
		"command":         command,
		"working_dir":     args["working_dir"],
		"timeout_seconds": 120,
	})
}
