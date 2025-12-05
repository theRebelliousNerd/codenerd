// Package tactile is the motor cortex of the neuro-symbolic architecture.
// It provides the lowest-level execution layer that physically interacts with
// the outside world - shell commands, process management, and sandboxed execution.
//
// The name follows the neuroscience metaphor:
//   - Perception: NL → Mangle atoms (sensory input)
//   - Kernel: Mangle reasoning (cognition)
//   - Articulation: Mangle atoms → NL (speech output)
//   - Tactile: Physical world interaction (motor output)
//
// Design Principles:
//   - Minimal logic: Constitutional checks happen in VirtualStore, not here
//   - Sandboxing: Support for Docker, namespaces, and direct execution
//   - Resource limits: CPU, memory, disk I/O constraints
//   - Structured output: Comprehensive execution results for kernel feedback
//   - Cross-platform: Windows and Unix support
//   - Audit trail: Execution events as facts for the kernel
package tactile

import (
	"time"
)

// SandboxMode defines the isolation level for command execution.
type SandboxMode string

const (
	// SandboxNone runs commands directly on the host (default).
	SandboxNone SandboxMode = "none"

	// SandboxDocker runs commands in a Docker container.
	SandboxDocker SandboxMode = "docker"

	// SandboxNamespace uses Linux namespaces for isolation (Linux only).
	SandboxNamespace SandboxMode = "namespace"

	// SandboxFirejail uses Firejail for sandboxing (Linux only).
	SandboxFirejail SandboxMode = "firejail"
)

// Command represents a command to be executed.
// This is the input specification for all executor types.
type Command struct {
	// Binary is the executable to run (e.g., "go", "git", "bash").
	Binary string `json:"binary"`

	// Arguments are the command-line arguments.
	Arguments []string `json:"arguments"`

	// WorkingDirectory is the directory to execute in.
	// If empty, uses the executor's default working directory.
	WorkingDirectory string `json:"working_directory,omitempty"`

	// Environment variables to set (in KEY=VALUE format).
	// These are merged with the executor's allowed environment.
	Environment []string `json:"environment,omitempty"`

	// Stdin provides input to the command's standard input.
	Stdin string `json:"stdin,omitempty"`

	// Limits specifies resource constraints for execution.
	Limits *ResourceLimits `json:"limits,omitempty"`

	// Sandbox specifies isolation settings.
	Sandbox *SandboxConfig `json:"sandbox,omitempty"`

	// SessionID links this execution to a logical session (for audit).
	SessionID string `json:"session_id,omitempty"`

	// RequestID uniquely identifies this execution request.
	RequestID string `json:"request_id,omitempty"`

	// Tags are arbitrary key-value pairs for categorization and audit.
	Tags map[string]string `json:"tags,omitempty"`
}

// CommandString returns the full command as a string (for display/logging).
func (c Command) CommandString() string {
	if len(c.Arguments) == 0 {
		return c.Binary
	}
	result := c.Binary
	for _, arg := range c.Arguments {
		result += " " + arg
	}
	return result
}

// ResourceLimits defines constraints on command execution.
type ResourceLimits struct {
	// TimeoutMs is the maximum execution time in milliseconds.
	// Zero means use the executor's default timeout.
	TimeoutMs int64 `json:"timeout_ms,omitempty"`

	// MaxCPUTimeMs limits CPU time consumption (not wall time).
	// Zero means unlimited. Not all platforms support this.
	MaxCPUTimeMs int64 `json:"max_cpu_time_ms,omitempty"`

	// MaxMemoryBytes limits memory usage.
	// Zero means unlimited. Not all platforms support this.
	MaxMemoryBytes int64 `json:"max_memory_bytes,omitempty"`

	// MaxOutputBytes limits captured stdout+stderr size.
	// Zero means use the executor's default (typically 10MB).
	MaxOutputBytes int64 `json:"max_output_bytes,omitempty"`

	// MaxFileSize limits the size of files the process can create.
	// Zero means unlimited.
	MaxFileSize int64 `json:"max_file_size,omitempty"`

	// MaxProcesses limits the number of child processes.
	// Zero means use OS default.
	MaxProcesses int `json:"max_processes,omitempty"`

	// NetworkAllowed controls whether network access is permitted.
	// Only enforced in sandbox modes that support network isolation.
	NetworkAllowed *bool `json:"network_allowed,omitempty"`
}

// SandboxConfig specifies isolation settings for command execution.
type SandboxConfig struct {
	// Mode is the sandboxing strategy.
	Mode SandboxMode `json:"mode"`

	// Image is the Docker image to use (for Docker mode).
	Image string `json:"image,omitempty"`

	// ReadOnlyRoot makes the root filesystem read-only.
	ReadOnlyRoot bool `json:"read_only_root,omitempty"`

	// AllowedPaths are paths to mount read-write into the sandbox.
	AllowedPaths []string `json:"allowed_paths,omitempty"`

	// ReadOnlyPaths are paths to mount read-only into the sandbox.
	ReadOnlyPaths []string `json:"read_only_paths,omitempty"`

	// DropCapabilities lists Linux capabilities to drop.
	DropCapabilities []string `json:"drop_capabilities,omitempty"`

	// NoNewPrivileges prevents privilege escalation.
	NoNewPrivileges bool `json:"no_new_privileges,omitempty"`

	// User runs the command as this user (user:group format).
	User string `json:"user,omitempty"`

	// NetworkMode for Docker: "none", "host", "bridge".
	NetworkMode string `json:"network_mode,omitempty"`

	// TmpfsSize is the size of /tmp tmpfs mount (e.g., "100m").
	TmpfsSize string `json:"tmpfs_size,omitempty"`
}

// ExecutionResult is the comprehensive output of command execution.
// It provides all information needed for kernel fact injection and debugging.
type ExecutionResult struct {
	// Success indicates whether the command completed without error.
	// Note: A command that runs but returns non-zero exit code has Success=true.
	// Success=false means the execution infrastructure failed.
	Success bool `json:"success"`

	// ExitCode is the command's exit code (-1 if not available).
	ExitCode int `json:"exit_code"`

	// Stdout is the captured standard output.
	Stdout string `json:"stdout"`

	// Stderr is the captured standard error.
	Stderr string `json:"stderr"`

	// Combined is stdout+stderr interleaved in order (if available).
	Combined string `json:"combined"`

	// Duration is how long the command ran.
	Duration time.Duration `json:"duration"`

	// StartedAt is when execution began.
	StartedAt time.Time `json:"started_at"`

	// FinishedAt is when execution completed.
	FinishedAt time.Time `json:"finished_at"`

	// Killed indicates the command was forcibly terminated.
	Killed bool `json:"killed"`

	// KillReason explains why the command was killed.
	KillReason string `json:"kill_reason,omitempty"`

	// Truncated indicates output was truncated due to size limits.
	Truncated bool `json:"truncated"`

	// TruncatedBytes is how many bytes were discarded.
	TruncatedBytes int64 `json:"truncated_bytes,omitempty"`

	// ResourceUsage contains resource consumption metrics (if available).
	ResourceUsage *ResourceUsage `json:"resource_usage,omitempty"`

	// Error contains any infrastructure-level error message.
	Error string `json:"error,omitempty"`

	// SandboxUsed indicates which sandbox mode was actually used.
	SandboxUsed SandboxMode `json:"sandbox_used"`

	// Command is a copy of the command that was executed (for audit).
	Command *Command `json:"command,omitempty"`
}

// IsError returns true if the execution failed (infrastructure error).
func (r *ExecutionResult) IsError() bool {
	return !r.Success || r.Error != ""
}

// IsNonZeroExit returns true if the command ran but returned non-zero.
func (r *ExecutionResult) IsNonZeroExit() bool {
	return r.Success && r.ExitCode != 0
}

// Output returns Combined if available, otherwise Stdout+Stderr.
func (r *ExecutionResult) Output() string {
	if r.Combined != "" {
		return r.Combined
	}
	if r.Stderr == "" {
		return r.Stdout
	}
	if r.Stdout == "" {
		return r.Stderr
	}
	return r.Stdout + "\n" + r.Stderr
}

// ResourceUsage contains metrics about resource consumption.
type ResourceUsage struct {
	// UserTimeMs is user-mode CPU time in milliseconds.
	UserTimeMs int64 `json:"user_time_ms"`

	// SystemTimeMs is kernel-mode CPU time in milliseconds.
	SystemTimeMs int64 `json:"system_time_ms"`

	// MaxRSSBytes is peak resident set size in bytes.
	MaxRSSBytes int64 `json:"max_rss_bytes"`

	// DiskReadBytes is bytes read from disk.
	DiskReadBytes int64 `json:"disk_read_bytes"`

	// DiskWriteBytes is bytes written to disk.
	DiskWriteBytes int64 `json:"disk_write_bytes"`

	// VoluntaryContextSwitches is voluntary context switches.
	VoluntaryContextSwitches int64 `json:"voluntary_context_switches"`

	// InvoluntaryContextSwitches is involuntary context switches.
	InvoluntaryContextSwitches int64 `json:"involuntary_context_switches"`
}

// TotalCPUTimeMs returns total CPU time (user + system).
func (r *ResourceUsage) TotalCPUTimeMs() int64 {
	return r.UserTimeMs + r.SystemTimeMs
}

// ExecutorCapabilities describes what an executor can do.
type ExecutorCapabilities struct {
	// Name is the executor implementation name.
	Name string `json:"name"`

	// Platform is the operating system (e.g., "windows", "linux", "darwin").
	Platform string `json:"platform"`

	// SupportsResourceLimits indicates resource limit enforcement is available.
	SupportsResourceLimits bool `json:"supports_resource_limits"`

	// SupportsResourceUsage indicates resource usage metrics are available.
	SupportsResourceUsage bool `json:"supports_resource_usage"`

	// SupportedSandboxModes lists available sandbox modes.
	SupportedSandboxModes []SandboxMode `json:"supported_sandbox_modes"`

	// SupportsNetworkIsolation indicates network isolation is available.
	SupportsNetworkIsolation bool `json:"supports_network_isolation"`

	// SupportsStdin indicates stdin input is supported.
	SupportsStdin bool `json:"supports_stdin"`

	// MaxTimeout is the maximum allowed timeout (0 = unlimited).
	MaxTimeout time.Duration `json:"max_timeout"`

	// DefaultTimeout is used when no timeout is specified.
	DefaultTimeout time.Duration `json:"default_timeout"`
}

// AuditEventType categorizes audit events.
type AuditEventType string

const (
	AuditEventStart     AuditEventType = "start"
	AuditEventComplete  AuditEventType = "complete"
	AuditEventKilled    AuditEventType = "killed"
	AuditEventError     AuditEventType = "error"
	AuditEventBlocked   AuditEventType = "blocked"
	AuditEventSandboxed AuditEventType = "sandboxed"
)

// AuditEvent represents an execution event for kernel fact injection.
type AuditEvent struct {
	// Type is the event category.
	Type AuditEventType `json:"type"`

	// Timestamp is when the event occurred.
	Timestamp time.Time `json:"timestamp"`

	// Command is the command being executed.
	Command Command `json:"command"`

	// Result is the execution result (for complete/killed/error events).
	Result *ExecutionResult `json:"result,omitempty"`

	// SessionID links to the logical session.
	SessionID string `json:"session_id,omitempty"`

	// ExecutorName is which executor handled this.
	ExecutorName string `json:"executor_name"`

	// BlockReason explains why execution was blocked (for blocked events).
	BlockReason string `json:"block_reason,omitempty"`
}

// ExecutorConfig is the configuration for creating executors.
type ExecutorConfig struct {
	// DefaultWorkingDir is used when Command.WorkingDirectory is empty.
	DefaultWorkingDir string `json:"default_working_dir"`

	// DefaultTimeout is used when no timeout is specified.
	DefaultTimeout time.Duration `json:"default_timeout"`

	// MaxTimeout caps all timeout values.
	MaxTimeout time.Duration `json:"max_timeout"`

	// AllowedEnvironment lists environment variables to pass through.
	AllowedEnvironment []string `json:"allowed_environment"`

	// DefaultSandbox is applied when Command.Sandbox is nil.
	DefaultSandbox *SandboxConfig `json:"default_sandbox,omitempty"`

	// DefaultLimits is applied when Command.Limits is nil.
	DefaultLimits *ResourceLimits `json:"default_limits,omitempty"`

	// MaxOutputBytes caps output capture (default 10MB).
	MaxOutputBytes int64 `json:"max_output_bytes"`

	// AuditCallback is called for each execution event (optional).
	AuditCallback func(AuditEvent) `json:"-"`

	// DockerDefaultImage is used for Docker sandbox when no image specified.
	DockerDefaultImage string `json:"docker_default_image,omitempty"`

	// EnableResourceUsage enables collection of resource metrics.
	EnableResourceUsage bool `json:"enable_resource_usage"`
}

// DefaultExecutorConfig returns sensible defaults.
func DefaultExecutorConfig() ExecutorConfig {
	return ExecutorConfig{
		DefaultWorkingDir:  ".",
		DefaultTimeout:     30 * time.Second,
		MaxTimeout:         10 * time.Minute,
		MaxOutputBytes:     10 * 1024 * 1024, // 10MB
		AllowedEnvironment: []string{"PATH", "HOME", "GOPATH", "GOROOT", "GOBIN", "USER", "LANG", "LC_ALL"},
		DefaultLimits: &ResourceLimits{
			TimeoutMs:      30000,
			MaxOutputBytes: 10 * 1024 * 1024,
		},
		DockerDefaultImage:  "alpine:latest",
		EnableResourceUsage: true,
	}
}

// Merge combines this config with command-specific settings.
// Command settings override config defaults.
func (c ExecutorConfig) Merge(cmd Command) Command {
	result := cmd

	// Apply default working directory
	if result.WorkingDirectory == "" {
		result.WorkingDirectory = c.DefaultWorkingDir
	}

	// Apply default limits
	if result.Limits == nil && c.DefaultLimits != nil {
		limitsCopy := *c.DefaultLimits
		result.Limits = &limitsCopy
	} else if result.Limits != nil && c.DefaultLimits != nil {
		// Merge specific limit fields
		if result.Limits.TimeoutMs == 0 {
			result.Limits.TimeoutMs = c.DefaultLimits.TimeoutMs
		}
		if result.Limits.MaxOutputBytes == 0 {
			result.Limits.MaxOutputBytes = c.DefaultLimits.MaxOutputBytes
		}
	}

	// Cap timeout at max
	if result.Limits != nil && c.MaxTimeout > 0 {
		maxMs := int64(c.MaxTimeout / time.Millisecond)
		if result.Limits.TimeoutMs > maxMs {
			result.Limits.TimeoutMs = maxMs
		}
	}

	// Apply default sandbox
	if result.Sandbox == nil && c.DefaultSandbox != nil {
		sandboxCopy := *c.DefaultSandbox
		result.Sandbox = &sandboxCopy
	}

	return result
}
