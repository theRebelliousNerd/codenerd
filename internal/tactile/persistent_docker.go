package tactile

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
)

// =============================================================================
// PERSISTENT DOCKER EXECUTOR
// =============================================================================
// Extends DockerExecutor for long-running containers that preserve state
// across multiple command executions. Essential for SWE-bench where we need
// to: clone repo -> setup venv -> apply patch -> run tests iteratively.
//
// Key difference from DockerExecutor:
// - DockerExecutor: `docker run --rm` (ephemeral, new container per command)
// - PersistentDockerExecutor: `docker create` + `docker exec` (stateful)

// ContainerState represents the lifecycle state of a persistent container.
type ContainerState string

const (
	ContainerStateCreating ContainerState = "creating"
	ContainerStateRunning  ContainerState = "running"
	ContainerStatePaused   ContainerState = "paused"
	ContainerStateStopped  ContainerState = "stopped"
	ContainerStateError    ContainerState = "error"
)

// PersistentContainer represents a long-running container instance.
type PersistentContainer struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Image         string            `json:"image"`
	State         ContainerState    `json:"state"`
	CreatedAt     time.Time         `json:"created_at"`
	LastExecAt    time.Time         `json:"last_exec_at"`
	WorkingDir    string            `json:"working_dir"`
	Environment   []string          `json:"environment"`
	Mounts        []ContainerMount  `json:"mounts"`
	HealthChecks  int               `json:"health_checks"`
	ExecCount     int               `json:"exec_count"`
	Labels        map[string]string `json:"labels"`
	LastError     string            `json:"last_error,omitempty"`
}

// ContainerMount defines a volume mount for the container.
type ContainerMount struct {
	Source   string `json:"source"`
	Target   string `json:"target"`
	ReadOnly bool   `json:"read_only"`
	Type     string `json:"type"` // "bind", "volume", "tmpfs"
}

// ContainerSnapshot represents a point-in-time snapshot of a container.
type ContainerSnapshot struct {
	ID          string    `json:"id"`
	ContainerID string    `json:"container_id"`
	CreatedAt   time.Time `json:"created_at"`
	Description string    `json:"description"`
	ImageTag    string    `json:"image_tag"`
	Size        int64     `json:"size_bytes"`
}

// ContainerPoolConfig configures the container pool behavior.
type ContainerPoolConfig struct {
	MaxContainers       int           `json:"max_containers"`
	IdleTimeout         time.Duration `json:"idle_timeout"`
	HealthCheckInterval time.Duration `json:"health_check_interval"`
	DefaultImage        string        `json:"default_image"`
	DefaultMemoryLimit  int64         `json:"default_memory_limit"`
	DefaultCPULimit     float64       `json:"default_cpu_limit"`
	CleanupOnError      bool          `json:"cleanup_on_error"`
	EnableSnapshots     bool          `json:"enable_snapshots"`
	SnapshotDir         string        `json:"snapshot_dir"`
}

// DefaultContainerPoolConfig returns sensible defaults for the pool.
func DefaultContainerPoolConfig() ContainerPoolConfig {
	return ContainerPoolConfig{
		MaxContainers:       10,
		IdleTimeout:         30 * time.Minute,
		HealthCheckInterval: 1 * time.Minute,
		DefaultImage:        "python:3.11-slim",
		DefaultMemoryLimit:  2 * 1024 * 1024 * 1024, // 2GB
		DefaultCPULimit:     2.0,                     // 2 CPUs
		CleanupOnError:      true,
		EnableSnapshots:     true,
		SnapshotDir:         ".nerd/snapshots",
	}
}

// ContainerCreateOptions specifies options for creating a new container.
type ContainerCreateOptions struct {
	Name         string
	Image        string
	WorkingDir   string
	Environment  []string
	Mounts       []ContainerMount
	MemoryLimit  int64
	CPULimit     float64
	NetworkMode  string
	Labels       map[string]string
	Entrypoint   []string // Override entrypoint to keep container running
	Command      []string // Initial command (default: sleep infinity)
}

// ContainerExecOptions specifies options for executing a command in a container.
type ContainerExecOptions struct {
	ContainerID string
	Binary      string
	Arguments   []string
	WorkingDir  string
	Environment []string
	User        string
	Timeout     time.Duration
}

// PersistentDockerExecutor manages long-running containers for iterative
// command execution. Unlike DockerExecutor which creates a new container
// per command, this maintains container state across executions.
type PersistentDockerExecutor struct {
	mu            sync.RWMutex
	dockerPath    string
	available     bool
	config        ContainerPoolConfig
	containers    map[string]*PersistentContainer
	snapshots     map[string]*ContainerSnapshot
	auditCallback func(AuditEvent)
	healthTicker  *time.Ticker
	stopChan      chan struct{}
	started       bool
}

// NewPersistentDockerExecutor creates a new persistent Docker executor.
func NewPersistentDockerExecutor(config ContainerPoolConfig) *PersistentDockerExecutor {
	logging.TactileDebug("Creating new PersistentDockerExecutor")
	e := &PersistentDockerExecutor{
		config:     config,
		containers: make(map[string]*PersistentContainer),
		snapshots:  make(map[string]*ContainerSnapshot),
		stopChan:   make(chan struct{}),
	}
	e.detectDocker()
	return e
}

// detectDocker checks if Docker is available.
func (e *PersistentDockerExecutor) detectDocker() {
	logging.TactileDebug("Detecting Docker availability for PersistentDockerExecutor")
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		logging.TactileDebug("Docker binary not found in PATH")
		e.available = false
		return
	}
	e.dockerPath = dockerPath

	// Verify docker is responsive
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, dockerPath, "version", "--format", "{{.Server.Version}}")
	if err := cmd.Run(); err != nil {
		logging.TactileWarn("Docker found but not responsive: %v", err)
		e.available = false
		return
	}

	e.available = true
	logging.Tactile("PersistentDockerExecutor available: %s", dockerPath)
}

// IsAvailable returns whether Docker is available on this system.
func (e *PersistentDockerExecutor) IsAvailable() bool {
	return e.available
}

// SetAuditCallback sets the callback for audit events.
func (e *PersistentDockerExecutor) SetAuditCallback(callback func(AuditEvent)) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.auditCallback = callback
}

// emitAudit emits an audit event if a callback is registered.
func (e *PersistentDockerExecutor) emitAudit(event AuditEvent) {
	e.mu.RLock()
	callback := e.auditCallback
	e.mu.RUnlock()

	if callback != nil {
		callback(event)
	}
}

// Start begins background health checking and cleanup.
func (e *PersistentDockerExecutor) Start() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.started {
		return nil
	}

	if !e.available {
		return fmt.Errorf("Docker is not available")
	}

	e.healthTicker = time.NewTicker(e.config.HealthCheckInterval)
	e.started = true

	go e.healthCheckLoop()

	logging.Tactile("PersistentDockerExecutor started with health checks every %s", e.config.HealthCheckInterval)
	return nil
}

// Stop halts background operations and optionally cleans up containers.
func (e *PersistentDockerExecutor) Stop() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.started {
		return nil
	}

	close(e.stopChan)
	if e.healthTicker != nil {
		e.healthTicker.Stop()
	}
	e.started = false

	logging.Tactile("PersistentDockerExecutor stopped")
	return nil
}

// healthCheckLoop runs periodic health checks on all containers.
func (e *PersistentDockerExecutor) healthCheckLoop() {
	for {
		select {
		case <-e.stopChan:
			return
		case <-e.healthTicker.C:
			e.performHealthChecks()
		}
	}
}

// performHealthChecks checks health of all managed containers.
func (e *PersistentDockerExecutor) performHealthChecks() {
	e.mu.RLock()
	containerIDs := make([]string, 0, len(e.containers))
	for id := range e.containers {
		containerIDs = append(containerIDs, id)
	}
	e.mu.RUnlock()

	for _, id := range containerIDs {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		healthy, _ := e.HealthCheck(ctx, id)
		cancel()

		if !healthy {
			logging.TactileWarn("Container %s failed health check", id[:12])
		}
	}
}

// =============================================================================
// CONTAINER LIFECYCLE
// =============================================================================

// CreateContainer creates a new persistent container.
func (e *PersistentDockerExecutor) CreateContainer(ctx context.Context, opts ContainerCreateOptions) (*PersistentContainer, error) {
	if !e.available {
		return nil, fmt.Errorf("Docker is not available")
	}

	logging.Tactile("Creating persistent container: image=%s, name=%s", opts.Image, opts.Name)

	// Use default image if not specified
	image := opts.Image
	if image == "" {
		image = e.config.DefaultImage
	}

	// Build docker create arguments
	args := []string{"create"}

	// Container name
	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}

	// Working directory
	if opts.WorkingDir != "" {
		args = append(args, "-w", opts.WorkingDir)
	}

	// Environment variables
	for _, env := range opts.Environment {
		args = append(args, "-e", env)
	}

	// Mounts
	for _, mount := range opts.Mounts {
		mountArg := fmt.Sprintf("%s:%s", mount.Source, mount.Target)
		if mount.ReadOnly {
			mountArg += ":ro"
		}
		args = append(args, "-v", mountArg)
	}

	// Resource limits
	if opts.MemoryLimit > 0 {
		args = append(args, "--memory", fmt.Sprintf("%d", opts.MemoryLimit))
	} else if e.config.DefaultMemoryLimit > 0 {
		args = append(args, "--memory", fmt.Sprintf("%d", e.config.DefaultMemoryLimit))
	}

	if opts.CPULimit > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", opts.CPULimit))
	} else if e.config.DefaultCPULimit > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.2f", e.config.DefaultCPULimit))
	}

	// Network mode
	if opts.NetworkMode != "" {
		args = append(args, "--network", opts.NetworkMode)
	}

	// Labels for tracking
	args = append(args, "--label", "codenerd.managed=true")
	args = append(args, "--label", fmt.Sprintf("codenerd.created=%d", time.Now().Unix()))
	for k, v := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", k, v))
	}

	// Image
	args = append(args, image)

	// Command to keep container running
	if len(opts.Command) > 0 {
		args = append(args, opts.Command...)
	} else {
		// Default: sleep infinity to keep container alive
		args = append(args, "sleep", "infinity")
	}

	logging.TactileDebug("Docker create args: %v", args)

	// Execute docker create
	cmd := exec.CommandContext(ctx, e.dockerPath, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logging.TactileError("Failed to create container: %v, stderr: %s", err, stderr.String())
		return nil, fmt.Errorf("failed to create container: %w: %s", err, stderr.String())
	}

	containerID := strings.TrimSpace(stdout.String())
	logging.TactileDebug("Container created with ID: %s", containerID)

	// Create container record
	container := &PersistentContainer{
		ID:          containerID,
		Name:        opts.Name,
		Image:       image,
		State:       ContainerStateStopped,
		CreatedAt:   time.Now(),
		WorkingDir:  opts.WorkingDir,
		Environment: opts.Environment,
		Mounts:      opts.Mounts,
		Labels:      opts.Labels,
	}

	// Store container
	e.mu.Lock()
	e.containers[containerID] = container
	e.mu.Unlock()

	logging.Tactile("Container created: %s (%s)", containerID[:12], image)
	return container, nil
}

// StartContainer starts a stopped container.
func (e *PersistentDockerExecutor) StartContainer(ctx context.Context, containerID string) error {
	if !e.available {
		return fmt.Errorf("Docker is not available")
	}

	logging.Tactile("Starting container: %s", containerID[:12])

	cmd := exec.CommandContext(ctx, e.dockerPath, "start", containerID)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logging.TactileError("Failed to start container: %v, stderr: %s", err, stderr.String())
		return fmt.Errorf("failed to start container: %w: %s", err, stderr.String())
	}

	// Update state
	e.mu.Lock()
	if container, ok := e.containers[containerID]; ok {
		container.State = ContainerStateRunning
	}
	e.mu.Unlock()

	logging.Tactile("Container started: %s", containerID[:12])
	return nil
}

// StopContainer stops a running container.
func (e *PersistentDockerExecutor) StopContainer(ctx context.Context, containerID string, timeout time.Duration) error {
	if !e.available {
		return fmt.Errorf("Docker is not available")
	}

	logging.Tactile("Stopping container: %s (timeout=%s)", containerID[:12], timeout)

	args := []string{"stop"}
	if timeout > 0 {
		args = append(args, "-t", fmt.Sprintf("%d", int(timeout.Seconds())))
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, e.dockerPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logging.TactileError("Failed to stop container: %v, stderr: %s", err, stderr.String())
		return fmt.Errorf("failed to stop container: %w: %s", err, stderr.String())
	}

	// Update state
	e.mu.Lock()
	if container, ok := e.containers[containerID]; ok {
		container.State = ContainerStateStopped
	}
	e.mu.Unlock()

	logging.Tactile("Container stopped: %s", containerID[:12])
	return nil
}

// RemoveContainer removes a container.
func (e *PersistentDockerExecutor) RemoveContainer(ctx context.Context, containerID string, force bool) error {
	if !e.available {
		return fmt.Errorf("Docker is not available")
	}

	logging.Tactile("Removing container: %s (force=%v)", containerID[:12], force)

	args := []string{"rm"}
	if force {
		args = append(args, "-f")
	}
	args = append(args, containerID)

	cmd := exec.CommandContext(ctx, e.dockerPath, args...)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logging.TactileError("Failed to remove container: %v, stderr: %s", err, stderr.String())
		return fmt.Errorf("failed to remove container: %w: %s", err, stderr.String())
	}

	// Remove from tracking
	e.mu.Lock()
	delete(e.containers, containerID)
	e.mu.Unlock()

	logging.Tactile("Container removed: %s", containerID[:12])
	return nil
}

// =============================================================================
// COMMAND EXECUTION
// =============================================================================

// ExecInContainer executes a command in a running container using docker exec.
// This is the key differentiator from DockerExecutor - state is preserved.
func (e *PersistentDockerExecutor) ExecInContainer(ctx context.Context, opts ContainerExecOptions) (*ExecutionResult, error) {
	timer := logging.StartTimer(logging.CategoryTactile, "Docker exec")
	defer timer.Stop()

	if !e.available {
		return nil, fmt.Errorf("Docker is not available")
	}

	logging.Tactile("Executing in container %s: %s %v", opts.ContainerID[:12], opts.Binary, opts.Arguments)

	// Build docker exec arguments
	args := []string{"exec"}

	// Working directory
	if opts.WorkingDir != "" {
		args = append(args, "-w", opts.WorkingDir)
	}

	// Environment variables
	for _, env := range opts.Environment {
		args = append(args, "-e", env)
	}

	// User
	if opts.User != "" {
		args = append(args, "-u", opts.User)
	}

	// Container ID
	args = append(args, opts.ContainerID)

	// Command
	args = append(args, opts.Binary)
	args = append(args, opts.Arguments...)

	logging.TactileDebug("Docker exec args: %v", args)

	// Determine timeout
	timeout := opts.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute // Default 5 min for exec
	}

	// Create timeout context
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// Build the command
	execCmd := exec.CommandContext(execCtx, e.dockerPath, args...)

	// Capture output
	var stdoutBuf, stderrBuf bytes.Buffer
	execCmd.Stdout = &stdoutBuf
	execCmd.Stderr = &stderrBuf

	// Prepare result
	result := &ExecutionResult{
		ExitCode:    -1,
		SandboxUsed: SandboxDocker,
		Command: &Command{
			Binary:    opts.Binary,
			Arguments: opts.Arguments,
		},
	}

	// Record start time
	result.StartedAt = time.Now()

	// Run the command
	err := execCmd.Run()

	// Record completion
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

	// Process error
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Killed = true
			result.KillReason = fmt.Sprintf("timeout after %s", timeout)
			result.Success = true
			logging.TactileWarn("Docker exec killed (timeout): %s after %s", opts.Binary, timeout)
		} else if execCtx.Err() == context.Canceled {
			result.Killed = true
			result.KillReason = "context canceled"
			result.Success = true
		} else if exitErr, ok := err.(*exec.ExitError); ok {
			result.Success = true
			result.ExitCode = exitErr.ExitCode()
			logging.TactileDebug("Docker exec exited non-zero: %s -> %d", opts.Binary, result.ExitCode)
		} else {
			result.Success = false
			result.Error = err.Error()
			logging.TactileError("Docker exec failed: %s - %v", opts.Binary, err)
			return result, nil
		}
	} else {
		result.Success = true
		result.ExitCode = 0
	}

	// Update container stats
	e.mu.Lock()
	if container, ok := e.containers[opts.ContainerID]; ok {
		container.LastExecAt = time.Now()
		container.ExecCount++
	}
	e.mu.Unlock()

	logging.Tactile("Docker exec completed: %s -> exit=%d, duration=%s",
		opts.Binary, result.ExitCode, result.Duration)

	return result, nil
}

// =============================================================================
// HEALTH CHECKS
// =============================================================================

// HealthCheck checks if a container is healthy and responsive.
func (e *PersistentDockerExecutor) HealthCheck(ctx context.Context, containerID string) (bool, error) {
	if !e.available {
		return false, fmt.Errorf("Docker is not available")
	}

	// Check container status
	cmd := exec.CommandContext(ctx, e.dockerPath, "inspect", "-f", "{{.State.Running}}", containerID)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout

	if err := cmd.Run(); err != nil {
		return false, err
	}

	running := strings.TrimSpace(stdout.String()) == "true"

	// Update state
	e.mu.Lock()
	if container, ok := e.containers[containerID]; ok {
		container.HealthChecks++
		if running {
			container.State = ContainerStateRunning
		} else {
			container.State = ContainerStateStopped
		}
	}
	e.mu.Unlock()

	return running, nil
}

// =============================================================================
// SNAPSHOT/RESTORE
// =============================================================================

// CreateSnapshot creates a snapshot of a container using docker commit.
func (e *PersistentDockerExecutor) CreateSnapshot(ctx context.Context, containerID, description string) (*ContainerSnapshot, error) {
	if !e.available {
		return nil, fmt.Errorf("Docker is not available")
	}

	if !e.config.EnableSnapshots {
		return nil, fmt.Errorf("snapshots are disabled")
	}

	logging.Tactile("Creating snapshot of container: %s", containerID[:12])

	// Generate snapshot tag
	snapshotTag := fmt.Sprintf("codenerd-snapshot-%s-%d", containerID[:12], time.Now().Unix())

	// Docker commit
	cmd := exec.CommandContext(ctx, e.dockerPath, "commit", "-m", description, containerID, snapshotTag)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		logging.TactileError("Failed to create snapshot: %v, stderr: %s", err, stderr.String())
		return nil, fmt.Errorf("failed to create snapshot: %w: %s", err, stderr.String())
	}

	imageID := strings.TrimSpace(stdout.String())

	// Create snapshot record
	snapshot := &ContainerSnapshot{
		ID:          imageID,
		ContainerID: containerID,
		CreatedAt:   time.Now(),
		Description: description,
		ImageTag:    snapshotTag,
	}

	// Store snapshot
	e.mu.Lock()
	e.snapshots[imageID] = snapshot
	e.mu.Unlock()

	logging.Tactile("Snapshot created: %s -> %s", containerID[:12], snapshotTag)
	return snapshot, nil
}

// RestoreSnapshot creates a new container from a snapshot.
func (e *PersistentDockerExecutor) RestoreSnapshot(ctx context.Context, snapshotID string) (*PersistentContainer, error) {
	if !e.available {
		return nil, fmt.Errorf("Docker is not available")
	}

	e.mu.RLock()
	snapshot, ok := e.snapshots[snapshotID]
	e.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	logging.Tactile("Restoring from snapshot: %s", snapshot.ImageTag)

	// Get original container config
	e.mu.RLock()
	originalContainer, ok := e.containers[snapshot.ContainerID]
	e.mu.RUnlock()

	var opts ContainerCreateOptions
	if ok {
		opts = ContainerCreateOptions{
			Name:        fmt.Sprintf("%s-restored-%d", originalContainer.Name, time.Now().Unix()),
			Image:       snapshot.ImageTag,
			WorkingDir:  originalContainer.WorkingDir,
			Environment: originalContainer.Environment,
			Mounts:      originalContainer.Mounts,
			Labels:      originalContainer.Labels,
		}
	} else {
		opts = ContainerCreateOptions{
			Image: snapshot.ImageTag,
		}
	}

	return e.CreateContainer(ctx, opts)
}

// ListSnapshots returns all snapshots for a container.
func (e *PersistentDockerExecutor) ListSnapshots(containerID string) []*ContainerSnapshot {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var result []*ContainerSnapshot
	for _, snapshot := range e.snapshots {
		if snapshot.ContainerID == containerID {
			result = append(result, snapshot)
		}
	}
	return result
}

// DeleteSnapshot removes a snapshot image.
func (e *PersistentDockerExecutor) DeleteSnapshot(ctx context.Context, snapshotID string) error {
	if !e.available {
		return fmt.Errorf("Docker is not available")
	}

	e.mu.RLock()
	snapshot, ok := e.snapshots[snapshotID]
	e.mu.RUnlock()

	if !ok {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}

	logging.Tactile("Deleting snapshot: %s", snapshot.ImageTag)

	cmd := exec.CommandContext(ctx, e.dockerPath, "rmi", snapshot.ImageTag)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to delete snapshot: %w", err)
	}

	e.mu.Lock()
	delete(e.snapshots, snapshotID)
	e.mu.Unlock()

	logging.Tactile("Snapshot deleted: %s", snapshot.ImageTag)
	return nil
}

// =============================================================================
// CONTAINER MANAGEMENT
// =============================================================================

// GetContainer returns a container by ID.
func (e *PersistentDockerExecutor) GetContainer(containerID string) (*PersistentContainer, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	container, ok := e.containers[containerID]
	return container, ok
}

// ListContainers returns all managed containers.
func (e *PersistentDockerExecutor) ListContainers() []*PersistentContainer {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*PersistentContainer, 0, len(e.containers))
	for _, container := range e.containers {
		result = append(result, container)
	}
	return result
}

// Cleanup removes all managed containers.
func (e *PersistentDockerExecutor) Cleanup(ctx context.Context) error {
	e.mu.RLock()
	containerIDs := make([]string, 0, len(e.containers))
	for id := range e.containers {
		containerIDs = append(containerIDs, id)
	}
	e.mu.RUnlock()

	var lastErr error
	for _, id := range containerIDs {
		if err := e.RemoveContainer(ctx, id, true); err != nil {
			lastErr = err
			logging.TactileWarn("Failed to remove container %s during cleanup: %v", id[:12], err)
		}
	}

	return lastErr
}

// =============================================================================
// FILE OPERATIONS
// =============================================================================

// CopyToContainer copies a file from host to container.
func (e *PersistentDockerExecutor) CopyToContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	if !e.available {
		return fmt.Errorf("Docker is not available")
	}

	logging.TactileDebug("Copying %s to container %s:%s", srcPath, containerID[:12], dstPath)

	cmd := exec.CommandContext(ctx, e.dockerPath, "cp", srcPath, fmt.Sprintf("%s:%s", containerID, dstPath))
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy to container: %w: %s", err, stderr.String())
	}

	return nil
}

// CopyFromContainer copies a file from container to host.
func (e *PersistentDockerExecutor) CopyFromContainer(ctx context.Context, containerID, srcPath, dstPath string) error {
	if !e.available {
		return fmt.Errorf("Docker is not available")
	}

	logging.TactileDebug("Copying container %s:%s to %s", containerID[:12], srcPath, dstPath)

	cmd := exec.CommandContext(ctx, e.dockerPath, "cp", fmt.Sprintf("%s:%s", containerID, srcPath), dstPath)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to copy from container: %w: %s", err, stderr.String())
	}

	return nil
}
