package tactile

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"
)

// DockerExecutor executes commands inside Docker containers.
// This provides strong isolation from the host system.
type DockerExecutor struct {
	mu     sync.RWMutex
	config ExecutorConfig

	// dockerPath is the path to the docker binary
	dockerPath string

	// available is true if Docker is available on this system
	available bool

	// auditCallback is called for execution events
	auditCallback func(AuditEvent)
}

// NewDockerExecutor creates a new Docker executor.
func NewDockerExecutor() *DockerExecutor {
	return NewDockerExecutorWithConfig(DefaultExecutorConfig())
}

// NewDockerExecutorWithConfig creates a new Docker executor with custom config.
func NewDockerExecutorWithConfig(config ExecutorConfig) *DockerExecutor {
	e := &DockerExecutor{
		config: config,
	}
	e.detectDocker()
	return e
}

// detectDocker checks if Docker is available.
func (e *DockerExecutor) detectDocker() {
	// Try to find docker binary
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		e.available = false
		return
	}
	e.dockerPath = dockerPath

	// Verify docker is responsive
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, dockerPath, "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err != nil {
		e.available = false
		return
	}

	e.available = true
}

// IsAvailable returns whether Docker is available on this system.
func (e *DockerExecutor) IsAvailable() bool {
	return e.available
}

// SetAuditCallback sets the callback for audit events.
func (e *DockerExecutor) SetAuditCallback(callback func(AuditEvent)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.auditCallback = callback
}

// emitAudit emits an audit event if a callback is registered.
func (e *DockerExecutor) emitAudit(event AuditEvent) {
	e.mu.RLock()
	callback := e.auditCallback
	e.mu.RUnlock()

	if callback != nil {
		callback(event)
	}
}

// Capabilities returns what this executor supports.
func (e *DockerExecutor) Capabilities() ExecutorCapabilities {
	modes := []SandboxMode{}
	if e.available {
		modes = append(modes, SandboxDocker)
	}

	return ExecutorCapabilities{
		Name:                     "docker",
		Platform:                 runtime.GOOS,
		SupportsResourceLimits:   true, // Docker has full resource limit support
		SupportsResourceUsage:    false, // Would need docker stats parsing
		SupportedSandboxModes:    modes,
		SupportsNetworkIsolation: true,
		SupportsStdin:            true,
		MaxTimeout:               e.config.MaxTimeout,
		DefaultTimeout:           e.config.DefaultTimeout,
	}
}

// Validate checks if a command can be executed.
func (e *DockerExecutor) Validate(cmd Command) error {
	if !e.available {
		return fmt.Errorf("Docker is not available on this system")
	}

	if cmd.Binary == "" {
		return fmt.Errorf("binary is required")
	}

	// Check sandbox mode
	if cmd.Sandbox == nil {
		return fmt.Errorf("DockerExecutor requires sandbox configuration")
	}

	if cmd.Sandbox.Mode != SandboxDocker {
		return fmt.Errorf("DockerExecutor only supports SandboxDocker mode, got %s", cmd.Sandbox.Mode)
	}

	return nil
}

// Execute runs a command inside a Docker container.
func (e *DockerExecutor) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	// Validate first
	if err := e.Validate(cmd); err != nil {
		return nil, err
	}

	// Merge config defaults
	cmd = e.config.Merge(cmd)

	// Ensure sandbox config exists
	if cmd.Sandbox == nil {
		cmd.Sandbox = &SandboxConfig{Mode: SandboxDocker}
	}

	// Prepare the result
	result := &ExecutionResult{
		ExitCode:    -1,
		SandboxUsed: SandboxDocker,
		Command:     &cmd,
	}

	// Emit start event
	e.emitAudit(AuditEvent{
		Type:         AuditEventStart,
		Timestamp:    time.Now(),
		Command:      cmd,
		SessionID:    cmd.SessionID,
		ExecutorName: "docker",
	})

	// Build docker run arguments
	dockerArgs := e.buildDockerArgs(cmd)

	// Determine timeout
	timeout := e.config.DefaultTimeout
	if cmd.Limits != nil && cmd.Limits.TimeoutMs > 0 {
		timeout = time.Duration(cmd.Limits.TimeoutMs) * time.Millisecond
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the docker command
	execCmd := exec.CommandContext(execCtx, e.dockerPath, dockerArgs...)

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

	// Process the error
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Killed = true
			result.KillReason = fmt.Sprintf("timeout after %s", timeout)
			result.Success = true
			e.emitAudit(AuditEvent{
				Type:         AuditEventKilled,
				Timestamp:    time.Now(),
				Command:      cmd,
				Result:       result,
				SessionID:    cmd.SessionID,
				ExecutorName: "docker",
			})
		} else if execCtx.Err() == context.Canceled {
			result.Killed = true
			result.KillReason = "context canceled"
			result.Success = true
			e.emitAudit(AuditEvent{
				Type:         AuditEventKilled,
				Timestamp:    time.Now(),
				Command:      cmd,
				Result:       result,
				SessionID:    cmd.SessionID,
				ExecutorName: "docker",
			})
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.Success = true
			result.ExitCode = exitErr.ExitCode()
		} else {
			result.Success = false
			result.Error = err.Error()
			e.emitAudit(AuditEvent{
				Type:         AuditEventError,
				Timestamp:    time.Now(),
				Command:      cmd,
				Result:       result,
				SessionID:    cmd.SessionID,
				ExecutorName: "docker",
			})
			return result, nil
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
		ExecutorName: "docker",
	})

	return result, nil
}

// buildDockerArgs constructs the docker run command arguments.
func (e *DockerExecutor) buildDockerArgs(cmd Command) []string {
	args := []string{"run", "--rm"}

	sandbox := cmd.Sandbox
	if sandbox == nil {
		sandbox = &SandboxConfig{}
	}

	// Determine image
	image := sandbox.Image
	if image == "" {
		image = e.config.DockerDefaultImage
	}
	if image == "" {
		image = "alpine:latest"
	}

	// Network mode
	networkMode := sandbox.NetworkMode
	if networkMode == "" {
		// Default to no network for security
		if cmd.Limits != nil && cmd.Limits.NetworkAllowed != nil && *cmd.Limits.NetworkAllowed {
			networkMode = "bridge"
		} else {
			networkMode = "none"
		}
	}
	args = append(args, "--network", networkMode)

	// Read-only root filesystem
	if sandbox.ReadOnlyRoot {
		args = append(args, "--read-only")
	}

	// Tmpfs for /tmp if read-only
	if sandbox.ReadOnlyRoot || sandbox.TmpfsSize != "" {
		tmpfsSize := sandbox.TmpfsSize
		if tmpfsSize == "" {
			tmpfsSize = "100m"
		}
		args = append(args, "--tmpfs", fmt.Sprintf("/tmp:size=%s", tmpfsSize))
	}

	// No new privileges
	if sandbox.NoNewPrivileges {
		args = append(args, "--security-opt", "no-new-privileges")
	}

	// Drop capabilities
	for _, cap := range sandbox.DropCapabilities {
		args = append(args, "--cap-drop", cap)
	}

	// User mapping
	if sandbox.User != "" {
		args = append(args, "--user", sandbox.User)
	}

	// Mount allowed paths read-write
	for _, path := range sandbox.AllowedPaths {
		args = append(args, "-v", fmt.Sprintf("%s:%s:rw", path, path))
	}

	// Mount read-only paths
	for _, path := range sandbox.ReadOnlyPaths {
		args = append(args, "-v", fmt.Sprintf("%s:%s:ro", path, path))
	}

	// Working directory
	if cmd.WorkingDirectory != "" {
		args = append(args, "-w", cmd.WorkingDirectory)
	}

	// Environment variables
	for _, env := range cmd.Environment {
		args = append(args, "-e", env)
	}

	// Resource limits
	if cmd.Limits != nil {
		// Memory limit
		if cmd.Limits.MaxMemoryBytes > 0 {
			args = append(args, "--memory", fmt.Sprintf("%d", cmd.Limits.MaxMemoryBytes))
		}

		// CPU limit (as CPU period/quota)
		if cmd.Limits.MaxCPUTimeMs > 0 {
			// Convert to CPU quota (100000 = 1 CPU second per 100ms period)
			// This is approximate - Docker uses CPU shares differently
			args = append(args, "--cpu-period", "100000")
			args = append(args, "--cpu-quota", fmt.Sprintf("%d", cmd.Limits.MaxCPUTimeMs*100))
		}

		// Process limit
		if cmd.Limits.MaxProcesses > 0 {
			args = append(args, "--pids-limit", fmt.Sprintf("%d", cmd.Limits.MaxProcesses))
		}
	}

	// Interactive mode for stdin
	if cmd.Stdin != "" {
		args = append(args, "-i")
	}

	// Add the image
	args = append(args, image)

	// Add the command and arguments
	args = append(args, cmd.Binary)
	args = append(args, cmd.Arguments...)

	return args
}

// PullImage pulls a Docker image if not already present.
func (e *DockerExecutor) PullImage(ctx context.Context, image string) error {
	if !e.available {
		return fmt.Errorf("Docker is not available")
	}

	cmd := exec.CommandContext(ctx, e.dockerPath, "pull", image)
	return cmd.Run()
}

// ImageExists checks if a Docker image exists locally.
func (e *DockerExecutor) ImageExists(ctx context.Context, image string) bool {
	if !e.available {
		return false
	}

	cmd := exec.CommandContext(ctx, e.dockerPath, "image", "inspect", image)
	return cmd.Run() == nil
}
