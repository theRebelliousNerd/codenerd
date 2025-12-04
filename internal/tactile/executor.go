package tactile

import (
	"context"
	"fmt"
	"os/exec"
	"time"
)

// ShellCommand represents a command to be executed.
type ShellCommand struct {
	Binary           string
	Arguments        []string
	WorkingDirectory string
	TimeoutSeconds   int
	EnvironmentVars  []string
}

// Executor defines the interface for executing commands.
type Executor interface {
	Execute(ctx context.Context, cmd ShellCommand) (string, error)
}

// SafeExecutor implements Executor with safety checks.
type SafeExecutor struct {
	AllowedBinaries map[string]bool
}

func NewSafeExecutor() *SafeExecutor {
	return &SafeExecutor{
		AllowedBinaries: map[string]bool{
			"go":    true,
			"grep":  true,
			"git":   true,
			"ls":    true,
			"mkdir": true,
			"rm":    false, // Explicitly denied. Cortex 1.5.0 "Constitutional Logic"
			"bash":  true,  // Added for VirtualStore routing
		},
	}
}

func (e *SafeExecutor) Execute(ctx context.Context, cmd ShellCommand) (string, error) {
	// Cortex 1.5.0: The Kernel should have already filtered this via 'permitted(Action)'.
	// This is a secondary "Defense in Depth" check.
	if !e.AllowedBinaries[cmd.Binary] {
		return "", fmt.Errorf("binary not allowed by Constitutional Logic: %s", cmd.Binary)
	}

	timeout := time.Duration(cmd.TimeoutSeconds) * time.Second
	if timeout == 0 {
		timeout = 30 * time.Second // Default timeout
	}

	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	c := exec.CommandContext(ctx, cmd.Binary, cmd.Arguments...)
	c.Dir = cmd.WorkingDirectory
	c.Env = cmd.EnvironmentVars

	output, err := c.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("command failed: %w, output: %s", err, string(output))
	}

	return string(output), nil
}
