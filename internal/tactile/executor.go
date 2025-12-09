// Package tactile provides the legacy SafeExecutor for backwards compatibility.
// New code should use the Executor interface from executor_interface.go.
package tactile

import (
	"context"
	"fmt"
	"os/exec"
	"time"

	"codenerd/internal/logging"
)

// ShellCommand represents a command to be executed (legacy type).
// Deprecated: Use Command instead for new code.
type ShellCommand struct {
	Binary           string
	Arguments        []string
	WorkingDirectory string
	TimeoutSeconds   int
	EnvironmentVars  []string
}

// SafeExecutor implements the legacy executor with safety checks.
// This is maintained for backwards compatibility with VirtualStore.
// New code should use DirectExecutor or other Executor implementations.
type SafeExecutor struct {
	AllowedBinaries map[string]bool
}

// NewSafeExecutor creates a new legacy SafeExecutor.
// Deprecated: Use NewDirectExecutor() for new code.
func NewSafeExecutor() *SafeExecutor {
	return &SafeExecutor{
		AllowedBinaries: map[string]bool{
			"go":      true,
			"grep":    true,
			"git":     true,
			"ls":      true,
			"dir":     true, // Windows
			"mkdir":   true,
			"rm":      false, // Explicitly denied. Constitutional Logic
			"bash":    true,
			"sh":      true,
			"cmd":     true, // Windows
			"powershell": true, // Windows
		},
	}
}

// Execute runs a command using the legacy ShellCommand interface.
// This method is for backwards compatibility with VirtualStore.
func (e *SafeExecutor) Execute(ctx context.Context, cmd ShellCommand) (string, error) {
	timer := logging.StartTimer(logging.CategoryTactile, "Legacy command execution")
	defer timer.Stop()

	logging.Tactile("Executing (legacy): %s %v", cmd.Binary, cmd.Arguments)
	logging.TactileDebug("Working directory: %s, timeout: %ds", cmd.WorkingDirectory, cmd.TimeoutSeconds)

	// Defense in depth check - kernel should have filtered via 'permitted(Action)'
	if allowed, exists := e.AllowedBinaries[cmd.Binary]; exists && !allowed {
		logging.TactileError("Binary blocked by Constitutional Logic: %s", cmd.Binary)
		return "", fmt.Errorf("binary not allowed by Constitutional Logic: %s", cmd.Binary)
	}

	timeout := time.Duration(cmd.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second
	}
	logging.TactileDebug("Using timeout: %s", timeout)

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(ctx, cmd.Binary, cmd.Arguments...)
	c.Dir = cmd.WorkingDirectory
	c.Env = cmd.EnvironmentVars

	output, err := c.CombinedOutput()
	if err != nil {
		if ctx.Err() == context.DeadlineExceeded {
			logging.TactileWarn("Command timed out after %s: %s", timeout, cmd.Binary)
		} else {
			logging.TactileError("Command failed: %s - %v", cmd.Binary, err)
		}
		return "", fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	logging.TactileDebug("Command completed successfully: %s (output=%d bytes)", cmd.Binary, len(output))
	return string(output), nil
}

// ExecuteNew runs a command using the new Command interface and returns ExecutionResult.
// This bridges the legacy SafeExecutor to the new interface.
func (e *SafeExecutor) ExecuteNew(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	logging.TactileDebug("ExecuteNew: bridging to legacy executor for: %s", cmd.Binary)

	// Convert to legacy format and execute
	legacyCmd := ShellCommand{
		Binary:           cmd.Binary,
		Arguments:        cmd.Arguments,
		WorkingDirectory: cmd.WorkingDirectory,
		EnvironmentVars:  cmd.Environment,
	}

	if cmd.Limits != nil && cmd.Limits.TimeoutMs > 0 {
		legacyCmd.TimeoutSeconds = int(cmd.Limits.TimeoutMs / 1000)
	}

	startTime := time.Now()
	output, err := e.Execute(ctx, legacyCmd)
	endTime := time.Now()

	result := &ExecutionResult{
		StartedAt:   startTime,
		FinishedAt:  endTime,
		Duration:    endTime.Sub(startTime),
		SandboxUsed: SandboxNone,
		Command:     &cmd,
	}

	if err != nil {
		result.Success = false
		result.Error = err.Error()
		result.ExitCode = -1
		logging.TactileDebug("ExecuteNew result: failed - %v", err)
	} else {
		result.Success = true
		result.ExitCode = 0
		result.Stdout = output
		result.Combined = output
		logging.TactileDebug("ExecuteNew result: success (output=%d bytes)", len(output))
	}

	return result, nil
}

// ToDirectExecutor converts this SafeExecutor to a DirectExecutor.
// Use this when migrating to the new interface.
func (e *SafeExecutor) ToDirectExecutor() *DirectExecutor {
	config := DefaultExecutorConfig()
	return NewDirectExecutorWithConfig(config)
}
