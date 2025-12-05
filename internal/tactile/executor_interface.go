package tactile

import (
	"context"
)

// Executor is the interface for command execution.
// All executor implementations must satisfy this interface.
type Executor interface {
	// Execute runs a command and returns a comprehensive result.
	// The context can be used for cancellation.
	Execute(ctx context.Context, cmd Command) (*ExecutionResult, error)

	// Capabilities returns what this executor supports.
	Capabilities() ExecutorCapabilities

	// Validate checks if a command can be executed by this executor.
	// Returns nil if valid, or an error explaining why not.
	Validate(cmd Command) error
}

// AuditedExecutorInterface wraps an executor to provide audit event generation.
type AuditedExecutorInterface interface {
	Executor

	// SetAuditCallback sets the callback for audit events.
	SetAuditCallback(callback func(AuditEvent))
}

// LimitedExecutorInterface is an executor that enforces resource limits.
type LimitedExecutorInterface interface {
	Executor

	// SetDefaultLimits sets the default resource limits.
	SetDefaultLimits(limits ResourceLimits)
}

// SandboxedExecutorInterface is an executor that supports isolation modes.
type SandboxedExecutorInterface interface {
	Executor

	// SetDefaultSandbox sets the default sandbox configuration.
	SetDefaultSandbox(sandbox SandboxConfig)

	// AvailableSandboxModes returns which modes are actually available.
	AvailableSandboxModes() []SandboxMode
}

// CompositeExecutorInterface combines multiple executors and routes based on sandbox mode.
type CompositeExecutorInterface interface {
	Executor

	// RegisterExecutor registers an executor for specific sandbox modes.
	RegisterExecutor(modes []SandboxMode, executor Executor)
}
