//go:build linux

package tactile

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// FirejailExecutor uses Firejail for sandboxing on Linux.
type FirejailExecutor struct {
	*DirectExecutor
	mu sync.RWMutex

	// firejailPath is the path to the firejail binary
	firejailPath string

	// available is true if Firejail is installed
	available bool
}

// NewFirejailExecutor creates a new Firejail-based executor.
func NewFirejailExecutor(config ExecutorConfig) *FirejailExecutor {
	e := &FirejailExecutor{
		DirectExecutor: NewDirectExecutorWithConfig(config),
	}
	e.detectFirejail()
	return e
}

// detectFirejail checks if Firejail is available.
func (e *FirejailExecutor) detectFirejail() {
	path, err := exec.LookPath("firejail")
	if err != nil {
		e.available = false
		return
	}
	e.firejailPath = path

	// Verify firejail works
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, path, "--version")
	if err := cmd.Run(); err != nil {
		e.available = false
		return
	}

	e.available = true
}

// IsAvailable returns whether Firejail is available.
func (e *FirejailExecutor) IsAvailable() bool {
	return e.available
}

// Capabilities returns what this executor supports.
func (e *FirejailExecutor) Capabilities() ExecutorCapabilities {
	modes := []SandboxMode{SandboxNone}
	if e.available {
		modes = append(modes, SandboxFirejail)
	}

	caps := e.DirectExecutor.Capabilities()
	caps.Name = "firejail"
	caps.SupportedSandboxModes = modes
	caps.SupportsNetworkIsolation = e.available
	caps.SupportsResourceLimits = e.available
	return caps
}

// Validate checks if a command can be executed.
func (e *FirejailExecutor) Validate(cmd Command) error {
	if cmd.Binary == "" {
		return fmt.Errorf("binary is required")
	}

	if cmd.Sandbox != nil && cmd.Sandbox.Mode != SandboxNone && cmd.Sandbox.Mode != SandboxFirejail && cmd.Sandbox.Mode != "" {
		return fmt.Errorf("FirejailExecutor only supports SandboxNone or SandboxFirejail, got %s", cmd.Sandbox.Mode)
	}

	if cmd.Sandbox != nil && cmd.Sandbox.Mode == SandboxFirejail && !e.available {
		return fmt.Errorf("Firejail is not available on this system")
	}

	return nil
}

// Execute runs a command inside a Firejail sandbox.
func (e *FirejailExecutor) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	if err := e.Validate(cmd); err != nil {
		return nil, err
	}

	// If not using firejail sandbox, delegate to parent
	if cmd.Sandbox == nil || cmd.Sandbox.Mode != SandboxFirejail {
		return e.DirectExecutor.Execute(ctx, cmd)
	}

	if !e.available {
		return nil, fmt.Errorf("Firejail is not available on this system")
	}

	// Merge config defaults
	cmd = e.config.Merge(cmd)

	// Prepare the result
	result := &ExecutionResult{
		ExitCode:    -1,
		SandboxUsed: SandboxFirejail,
		Command:     &cmd,
	}

	// Emit start event
	e.emitAudit(AuditEvent{
		Type:         AuditEventStart,
		Timestamp:    time.Now(),
		Command:      cmd,
		SessionID:    cmd.SessionID,
		ExecutorName: "firejail",
	})

	// Build firejail arguments
	firejailArgs := e.buildFirejailArgs(cmd)

	// Determine timeout
	timeout := e.config.DefaultTimeout
	if cmd.Limits != nil && cmd.Limits.TimeoutMs > 0 {
		timeout = time.Duration(cmd.Limits.TimeoutMs) * time.Millisecond
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the firejail command
	execCmd := exec.CommandContext(execCtx, e.firejailPath, firejailArgs...)
	execCmd.Dir = cmd.WorkingDirectory
	execCmd.Env = e.buildEnvironment(cmd.Environment)

	// Set up stdin if provided
	if cmd.Stdin != "" {
		execCmd.Stdin = strings.NewReader(cmd.Stdin)
	}

	// Set up output capture
	maxOutput := e.config.MaxOutputBytes
	if cmd.Limits != nil && cmd.Limits.MaxOutputBytes > 0 {
		maxOutput = cmd.Limits.MaxOutputBytes
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutLimited := &limitedWriter{w: &stdoutBuf, max: maxOutput}
	stderrLimited := &limitedWriter{w: &stderrBuf, max: maxOutput}

	execCmd.Stdout = stdoutLimited
	execCmd.Stderr = stderrLimited

	// Set up process group
	setupProcessGroup(execCmd)

	// Record start time
	result.StartedAt = time.Now()

	// Run the command
	err := execCmd.Run()

	// Record completion time
	result.FinishedAt = time.Now()
	result.Duration = result.FinishedAt.Sub(result.StartedAt)

	// Capture output
	result.Stdout = stdoutBuf.String()
	result.Stderr = stderrBuf.String()
	result.Combined = result.Stdout
	if result.Stderr != "" {
		if result.Combined != "" {
			result.Combined += "\n"
		}
		result.Combined += result.Stderr
	}

	// Check for truncation
	if stdoutLimited.truncated || stderrLimited.truncated {
		result.Truncated = true
		result.TruncatedBytes = stdoutLimited.discarded + stderrLimited.discarded
	}

	// Get resource usage
	result.ResourceUsage = getProcessResourceUsage(execCmd)

	// Process the error
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Killed = true
			result.KillReason = fmt.Sprintf("timeout after %s", timeout)
			result.Success = true
		} else if execCtx.Err() == context.Canceled {
			result.Killed = true
			result.KillReason = "context canceled"
			result.Success = true
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.Success = true
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Success = false
			result.Error = err.Error()
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	// Emit completion event
	e.emitAudit(AuditEvent{
		Type:         AuditEventComplete,
		Timestamp:    time.Now(),
		Command:      cmd,
		Result:       result,
		SessionID:    cmd.SessionID,
		ExecutorName: "firejail",
	})

	return result, nil
}

// buildFirejailArgs constructs Firejail arguments from sandbox config.
func (e *FirejailExecutor) buildFirejailArgs(cmd Command) []string {
	args := []string{}

	sandbox := cmd.Sandbox
	if sandbox == nil {
		sandbox = &SandboxConfig{}
	}

	// Quiet mode (less verbose)
	args = append(args, "--quiet")

	// Private /tmp
	args = append(args, "--private-tmp")

	// No new privileges (always enabled for security)
	args = append(args, "--nonewprivs")

	// Seccomp filtering
	args = append(args, "--seccomp")

	// Read-only filesystem
	if sandbox.ReadOnlyRoot {
		args = append(args, "--read-only=/")
	}

	// Network isolation
	if cmd.Limits != nil && cmd.Limits.NetworkAllowed != nil && !*cmd.Limits.NetworkAllowed {
		args = append(args, "--net=none")
	}

	// Drop capabilities
	if len(sandbox.DropCapabilities) > 0 {
		args = append(args, "--caps.drop="+strings.Join(sandbox.DropCapabilities, ","))
	} else {
		// Default: drop all capabilities
		args = append(args, "--caps.drop=all")
	}

	// Allowed paths (whitelist)
	for _, path := range sandbox.AllowedPaths {
		args = append(args, "--whitelist="+path)
	}

	// Read-only paths
	for _, path := range sandbox.ReadOnlyPaths {
		args = append(args, "--read-only="+path)
	}

	// Tmpfs for /tmp
	if sandbox.TmpfsSize != "" {
		// Firejail doesn't support tmpfs size directly, but private-tmp gives a tmpfs
	}

	// Resource limits via rlimit
	if cmd.Limits != nil {
		if cmd.Limits.MaxMemoryBytes > 0 {
			// Firejail uses KB for rlimit-as
			kb := cmd.Limits.MaxMemoryBytes / 1024
			args = append(args, fmt.Sprintf("--rlimit-as=%d", kb))
		}
		if cmd.Limits.MaxCPUTimeMs > 0 {
			seconds := cmd.Limits.MaxCPUTimeMs / 1000
			if seconds == 0 {
				seconds = 1
			}
			args = append(args, fmt.Sprintf("--rlimit-cpu=%d", seconds))
		}
		if cmd.Limits.MaxFileSize > 0 {
			args = append(args, fmt.Sprintf("--rlimit-fsize=%d", cmd.Limits.MaxFileSize))
		}
		if cmd.Limits.MaxProcesses > 0 {
			args = append(args, fmt.Sprintf("--rlimit-nproc=%d", cmd.Limits.MaxProcesses))
		}
	}

	// Timeout (firejail has its own timeout)
	if cmd.Limits != nil && cmd.Limits.TimeoutMs > 0 {
		seconds := cmd.Limits.TimeoutMs / 1000
		if seconds > 0 {
			args = append(args, fmt.Sprintf("--timeout=%02d:%02d:%02d", seconds/3600, (seconds%3600)/60, seconds%60))
		}
	}

	// Separator
	args = append(args, "--")

	// Actual command
	args = append(args, cmd.Binary)
	args = append(args, cmd.Arguments...)

	return args
}
