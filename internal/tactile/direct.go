package tactile

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// DirectExecutor executes commands directly on the host using os/exec.
// This is the simplest executor with no sandboxing.
type DirectExecutor struct {
	mu     sync.RWMutex
	config ExecutorConfig

	// auditCallback is called for execution events
	auditCallback func(AuditEvent)
}

// NewDirectExecutor creates a new direct executor with default config.
func NewDirectExecutor() *DirectExecutor {
	logging.TactileDebug("Creating new DirectExecutor with default config")
	return NewDirectExecutorWithConfig(DefaultExecutorConfig())
}

// NewDirectExecutorWithConfig creates a new direct executor with custom config.
func NewDirectExecutorWithConfig(config ExecutorConfig) *DirectExecutor {
	logging.TactileDebug("Creating DirectExecutor with config: timeout=%s, maxOutput=%d bytes",
		config.DefaultTimeout, config.MaxOutputBytes)
	return &DirectExecutor{
		config: config,
	}
}

// SetAuditCallback sets the callback for audit events.
func (e *DirectExecutor) SetAuditCallback(callback func(AuditEvent)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.auditCallback = callback
}

// emitAudit emits an audit event if a callback is registered.
func (e *DirectExecutor) emitAudit(event AuditEvent) {
	e.mu.RLock()
	callback := e.auditCallback
	e.mu.RUnlock()

	if callback != nil {
		callback(event)
	}
}

// Capabilities returns what this executor supports.
func (e *DirectExecutor) Capabilities() ExecutorCapabilities {
	return ExecutorCapabilities{
		Name:                     "direct",
		Platform:                 runtime.GOOS,
		SupportsResourceLimits:   runtime.GOOS != "windows", // Unix has ulimit
		SupportsResourceUsage:    runtime.GOOS != "windows", // Unix has rusage
		SupportedSandboxModes:    []SandboxMode{SandboxNone},
		SupportsNetworkIsolation: false,
		SupportsStdin:            true,
		MaxTimeout:               e.config.MaxTimeout,
		DefaultTimeout:           e.config.DefaultTimeout,
	}
}

// Validate checks if a command can be executed.
func (e *DirectExecutor) Validate(cmd Command) error {
	if cmd.Binary == "" {
		return fmt.Errorf("binary is required")
	}

	// Check if sandbox mode is supported
	if cmd.Sandbox != nil && cmd.Sandbox.Mode != SandboxNone && cmd.Sandbox.Mode != "" {
		return fmt.Errorf("DirectExecutor only supports SandboxNone, got %s", cmd.Sandbox.Mode)
	}

	return nil
}

// Execute runs a command directly on the host.
func (e *DirectExecutor) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	timer := logging.StartTimer(logging.CategoryTactile, "Direct command execution")
	defer timer.Stop()

	logging.Tactile("Executing command: %s", cmd.CommandString())

	// Validate first
	if err := e.Validate(cmd); err != nil {
		logging.TactileWarn("Command validation failed: %s %v - %v", cmd.Binary, cmd.Arguments, err)
		return nil, err
	}

	// Merge config defaults
	cmd = e.config.Merge(cmd)

	logging.TactileDebug("Executing: %s %v (dir=%s, timeout=%dms)",
		cmd.Binary, cmd.Arguments, cmd.WorkingDirectory,
		func() int64 {
			if cmd.Limits != nil {
				return cmd.Limits.TimeoutMs
			}
			return 0
		}())

	// Prepare the result
	result := &ExecutionResult{
		ExitCode:    -1,
		SandboxUsed: SandboxNone,
		Command:     &cmd,
	}

	// Emit start event
	e.emitAudit(AuditEvent{
		Type:         AuditEventStart,
		Timestamp:    time.Now(),
		Command:      cmd,
		SessionID:    cmd.SessionID,
		ExecutorName: "direct",
	})

	// Determine timeout
	timeout := e.config.DefaultTimeout
	if cmd.Limits != nil && cmd.Limits.TimeoutMs > 0 {
		timeout = time.Duration(cmd.Limits.TimeoutMs) * time.Millisecond
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the command
	execCmd := exec.CommandContext(execCtx, cmd.Binary, cmd.Arguments...)
	execCmd.Dir = cmd.WorkingDirectory

	// Set up environment
	execCmd.Env = e.buildEnvironment(cmd.Environment)

	// Set up stdin if provided
	if cmd.Stdin != "" {
		logging.TactileDebug("Providing stdin input (%d bytes)", len(cmd.Stdin))
		execCmd.Stdin = strings.NewReader(cmd.Stdin)
	}

	// Set up output capture with size limits
	maxOutput := e.config.MaxOutputBytes
	if cmd.Limits != nil && cmd.Limits.MaxOutputBytes > 0 {
		maxOutput = cmd.Limits.MaxOutputBytes
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	stdoutLimited := &limitedWriter{w: &stdoutBuf, max: maxOutput}
	stderrLimited := &limitedWriter{w: &stderrBuf, max: maxOutput}

	execCmd.Stdout = stdoutLimited
	execCmd.Stderr = stderrLimited

	// Record start time
	result.StartedAt = time.Now()
	logging.TactileDebug("Starting process: %s", cmd.Binary)

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
		logging.TactileWarn("Command output truncated: %d bytes discarded", result.TruncatedBytes)
	}

	// Process the error
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Killed = true
			result.KillReason = fmt.Sprintf("timeout after %s", timeout)
			result.Success = true // Infrastructure worked, command was killed
			logging.TactileWarn("Command killed (timeout): %s after %s", cmd.Binary, timeout)
			e.emitAudit(AuditEvent{
				Type:         AuditEventKilled,
				Timestamp:    time.Now(),
				Command:      cmd,
				Result:       result,
				SessionID:    cmd.SessionID,
				ExecutorName: "direct",
			})
		} else if execCtx.Err() == context.Canceled {
			result.Killed = true
			result.KillReason = "context canceled"
			result.Success = true
			logging.TactileDebug("Command canceled: %s", cmd.Binary)
			e.emitAudit(AuditEvent{
				Type:         AuditEventKilled,
				Timestamp:    time.Now(),
				Command:      cmd,
				Result:       result,
				SessionID:    cmd.SessionID,
				ExecutorName: "direct",
			})
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.Success = true // Command ran, just returned non-zero
			result.ExitCode = exitErr.ExitCode()
			logging.TactileDebug("Command exited non-zero: %s -> %d", cmd.Binary, result.ExitCode)
		} else {
			result.Success = false
			result.Error = err.Error()
			logging.TactileError("Command failed: %s - %v", cmd.Binary, err)
			e.emitAudit(AuditEvent{
				Type:         AuditEventError,
				Timestamp:    time.Now(),
				Command:      cmd,
				Result:       result,
				SessionID:    cmd.SessionID,
				ExecutorName: "direct",
			})
			return result, nil
		}
	} else {
		result.Success = true
		result.ExitCode = 0
		logging.TactileDebug("Command succeeded with exit code 0")
	}

	// Try to get resource usage (platform-specific)
	result.ResourceUsage = e.getResourceUsage(execCmd)

	// Emit completion event
	e.emitAudit(AuditEvent{
		Type:         AuditEventComplete,
		Timestamp:    time.Now(),
		Command:      cmd,
		Result:       result,
		SessionID:    cmd.SessionID,
		ExecutorName: "direct",
	})

	logging.Tactile("Command completed: %s -> exit=%d, duration=%s, stdout=%d bytes",
		cmd.Binary, result.ExitCode, result.Duration, len(result.Stdout))

	return result, nil
}

// buildEnvironment creates the environment variable list.
func (e *DirectExecutor) buildEnvironment(cmdEnv []string) []string {
	env := make([]string, 0)

	// Get allowed variables from current environment
	for _, key := range e.config.AllowedEnvironment {
		if val := os.Getenv(key); val != "" {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	// Add command-specific environment variables
	env = append(env, cmdEnv...)

	return env
}

// getResourceUsage extracts resource usage from the command (platform-specific).
// This is a stub - platform-specific implementations are in platform_*.go
func (e *DirectExecutor) getResourceUsage(cmd *exec.Cmd) *ResourceUsage {
	if !e.config.EnableResourceUsage {
		return nil
	}
	return getProcessResourceUsage(cmd)
}

// limitedWriter is an io.Writer that limits total bytes written.
type limitedWriter struct {
	w         io.Writer
	max       int64
	written   int64
	truncated bool
	discarded int64
}

func (lw *limitedWriter) Write(p []byte) (int, error) {
	n := len(p)

	if lw.written >= lw.max {
		lw.truncated = true
		lw.discarded += int64(n)
		return n, nil // Pretend we wrote it
	}

	remaining := lw.max - lw.written
	if int64(n) > remaining {
		// Partial write
		lw.truncated = true
		toWrite := p[:remaining]
		lw.discarded += int64(n) - remaining
		written, err := lw.w.Write(toWrite)
		lw.written += int64(written)
		return n, err // Return original length to avoid "short write" errors
	}

	written, err := lw.w.Write(p)
	lw.written += int64(written)
	return written, err
}

// ToCommand converts a legacy ShellCommand to the new Command type.
func (sc ShellCommand) ToCommand() Command {
	var limits *ResourceLimits
	if sc.TimeoutSeconds > 0 {
		limits = &ResourceLimits{
			TimeoutMs: int64(sc.TimeoutSeconds) * 1000,
		}
	}
	return Command{
		Binary:           sc.Binary,
		Arguments:        sc.Arguments,
		WorkingDirectory: sc.WorkingDirectory,
		Environment:      sc.EnvironmentVars,
		Limits:           limits,
	}
}
