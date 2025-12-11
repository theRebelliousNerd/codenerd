// Package python provides a general-purpose Python project environment.
// This is the base layer for working with any Python codebase in containers.
// SWE-bench, GitHub issues, and local debugging all use this foundation.
package python

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"codenerd/internal/logging"
	"codenerd/internal/tactile"
)

// =============================================================================
// PYTHON PROJECT ENVIRONMENT - General Purpose
// =============================================================================
// Manages containerized Python development environments for ANY Python project:
// - Django, Flask, FastAPI web applications
// - Data science / ML projects
// - CLI tools and libraries
// - Test suites of any complexity
//
// This is NOT benchmark-specific - it's the foundation for real-world Python work.

// EnvironmentState tracks the state of a Python environment.
type EnvironmentState string

const (
	StateInitializing EnvironmentState = "initializing"
	StateCloning      EnvironmentState = "cloning"
	StateCheckout     EnvironmentState = "checkout"
	StateSetup        EnvironmentState = "setup"
	StateReady        EnvironmentState = "ready"
	StatePatchApplied EnvironmentState = "patch_applied"
	StateTesting      EnvironmentState = "testing"
	StateError        EnvironmentState = "error"
	StateComplete     EnvironmentState = "complete"
)

// EnvironmentConfig configures Python environment behavior.
type EnvironmentConfig struct {
	// Container settings
	BaseImage      string        `json:"base_image"`       // Docker image (e.g., python:3.10)
	PythonVersion  string        `json:"python_version"`   // e.g., "3.10"
	MemoryLimit    int64         `json:"memory_limit"`     // Bytes
	CPULimit       float64       `json:"cpu_limit"`        // CPUs
	NetworkEnabled bool          `json:"network_enabled"`  // For pip install
	TestTimeout    time.Duration `json:"test_timeout"`     // Per-test timeout
	SetupTimeout   time.Duration `json:"setup_timeout"`    // Total setup timeout

	// Paths
	WorkspaceDir string `json:"workspace_dir"` // Inside container
	CacheDir     string `json:"cache_dir"`     // pip cache mount

	// Behavior
	EnableSnapshots    bool `json:"enable_snapshots"`
	SnapshotAfterSetup bool `json:"snapshot_after_setup"`
	PreserveOnFailure  bool `json:"preserve_on_failure"`
	Verbose            bool `json:"verbose"`
}

// DefaultConfig returns sensible defaults for most Python projects.
func DefaultConfig() EnvironmentConfig {
	return EnvironmentConfig{
		BaseImage:          "python:3.10-slim",
		PythonVersion:      "3.10",
		MemoryLimit:        4 * 1024 * 1024 * 1024, // 4GB
		CPULimit:           2.0,
		NetworkEnabled:     true, // Need network for pip
		TestTimeout:        5 * time.Minute,
		SetupTimeout:       15 * time.Minute,
		WorkspaceDir:       "/workspace",
		EnableSnapshots:    true,
		SnapshotAfterSetup: true,
		PreserveOnFailure:  true,
		Verbose:            false,
	}
}

// ProjectInfo describes a Python project to set up.
type ProjectInfo struct {
	Name       string `json:"name"`        // Project name
	GitURL     string `json:"git_url"`     // Git clone URL
	Commit     string `json:"commit"`      // Specific commit to checkout (optional)
	Branch     string `json:"branch"`      // Branch to checkout (optional)
	LocalPath  string `json:"local_path"`  // Local path to mount (alternative to git)
	SetupSteps []string `json:"setup_steps"` // Custom setup commands (optional)
}

// RepoName extracts the repository name from the git URL.
func (p *ProjectInfo) RepoName() string {
	if p.Name != "" {
		return p.Name
	}
	// Extract from git URL: https://github.com/owner/repo.git -> repo
	url := p.GitURL
	url = strings.TrimSuffix(url, ".git")
	parts := strings.Split(url, "/")
	if len(parts) > 0 {
		return parts[len(parts)-1]
	}
	return "project"
}

// TestResult holds the result of running a single test.
type TestResult struct {
	TestName     string        `json:"test_name"`
	Passed       bool          `json:"passed"`
	Duration     time.Duration `json:"duration"`
	Output       string        `json:"output"`
	ErrorMessage string        `json:"error_message,omitempty"`
	ExitCode     int           `json:"exit_code"`
}

// Environment manages a containerized Python development environment.
type Environment struct {
	mu sync.RWMutex

	project   *ProjectInfo
	config    EnvironmentConfig
	executor  *tactile.PersistentDockerExecutor
	container *tactile.PersistentContainer

	state         EnvironmentState
	setupSnapshot *tactile.ContainerSnapshot
	lastError     error
	repoPath      string
	venvPath      string

	// Timing
	setupStarted   time.Time
	setupCompleted time.Time
}

// NewEnvironment creates a new Python project environment.
func NewEnvironment(
	project *ProjectInfo,
	config EnvironmentConfig,
	executor *tactile.PersistentDockerExecutor,
) *Environment {
	repoPath := config.WorkspaceDir + "/" + project.RepoName()
	return &Environment{
		project:  project,
		config:   config,
		executor: executor,
		state:    StateInitializing,
		repoPath: repoPath,
		venvPath: config.WorkspaceDir + "/venv",
	}
}

// =============================================================================
// STATE ACCESSORS
// =============================================================================

// State returns the current environment state.
func (e *Environment) State() EnvironmentState {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.state
}

// Project returns the project info.
func (e *Environment) Project() *ProjectInfo {
	return e.project
}

// ContainerID returns the container ID (if created).
func (e *Environment) ContainerID() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if e.container == nil {
		return ""
	}
	return e.container.ID
}

// RepoPath returns the path to the cloned repo inside the container.
func (e *Environment) RepoPath() string {
	return e.repoPath
}

// VenvPath returns the path to the virtual environment.
func (e *Environment) VenvPath() string {
	return e.venvPath
}

// GetError returns the last error.
func (e *Environment) GetError() error {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.lastError
}

func (e *Environment) setState(state EnvironmentState) {
	e.mu.Lock()
	e.state = state
	e.mu.Unlock()
}

func (e *Environment) setError(err error) {
	e.mu.Lock()
	e.lastError = err
	e.state = StateError
	e.mu.Unlock()
}

// =============================================================================
// LIFECYCLE METHODS
// =============================================================================

// Initialize creates the container and prepares the environment.
func (e *Environment) Initialize(ctx context.Context) error {
	logging.Tactile("Initializing Python environment for %s", e.project.RepoName())
	e.setupStarted = time.Now()

	// Determine Docker image
	image := e.config.BaseImage
	if image == "" {
		image = fmt.Sprintf("python:%s-slim", e.config.PythonVersion)
	}

	// Create container
	networkMode := "none"
	if e.config.NetworkEnabled {
		networkMode = "bridge"
	}

	opts := tactile.ContainerCreateOptions{
		Name:        fmt.Sprintf("nerd-python-%s-%d", e.project.RepoName(), time.Now().Unix()),
		Image:       image,
		WorkingDir:  e.config.WorkspaceDir,
		MemoryLimit: e.config.MemoryLimit,
		CPULimit:    e.config.CPULimit,
		NetworkMode: networkMode,
		Labels: map[string]string{
			"nerd.project": e.project.RepoName(),
			"nerd.type":    "python",
		},
	}

	container, err := e.executor.CreateContainer(ctx, opts)
	if err != nil {
		e.setError(fmt.Errorf("failed to create container: %w", err))
		return e.lastError
	}

	e.mu.Lock()
	e.container = container
	e.mu.Unlock()

	// Start container
	if err := e.executor.StartContainer(ctx, container.ID); err != nil {
		e.setError(fmt.Errorf("failed to start container: %w", err))
		return e.lastError
	}

	logging.Tactile("Container created and started: %s", container.ID[:12])
	return nil
}

// Setup performs full environment setup: clone, checkout, venv, deps.
func (e *Environment) Setup(ctx context.Context) error {
	logging.Tactile("Setting up Python environment for %s", e.project.RepoName())

	// Clone repository (if git URL provided)
	if e.project.GitURL != "" {
		if err := e.CloneRepo(ctx); err != nil {
			return err
		}

		// Checkout specific commit/branch if specified
		if e.project.Commit != "" {
			if err := e.CheckoutCommit(ctx, e.project.Commit); err != nil {
				return err
			}
		} else if e.project.Branch != "" {
			if err := e.CheckoutBranch(ctx, e.project.Branch); err != nil {
				return err
			}
		}
	}

	// Set up virtual environment
	if err := e.SetupVirtualEnv(ctx); err != nil {
		return err
	}

	// Install dependencies
	if err := e.InstallDependencies(ctx); err != nil {
		return err
	}

	// Run custom setup steps if any
	for _, step := range e.project.SetupSteps {
		if _, err := e.ExecInVenv(ctx, step); err != nil {
			logging.TactileWarn("Custom setup step failed: %s - %v", step, err)
		}
	}

	e.setupCompleted = time.Now()
	e.setState(StateReady)

	// Create post-setup snapshot if enabled
	if e.config.EnableSnapshots && e.config.SnapshotAfterSetup {
		snapshot, err := e.executor.CreateSnapshot(ctx, e.container.ID, "post-setup")
		if err != nil {
			logging.TactileWarn("Failed to create setup snapshot: %v", err)
		} else {
			e.mu.Lock()
			e.setupSnapshot = snapshot
			e.mu.Unlock()
			logging.Tactile("Setup snapshot created: %s", snapshot.ImageTag)
		}
	}

	logging.Tactile("Python environment setup complete for %s (took %s)",
		e.project.RepoName(), e.setupCompleted.Sub(e.setupStarted).Round(time.Second))
	return nil
}

// Teardown removes the container and cleans up resources.
func (e *Environment) Teardown(ctx context.Context) error {
	logging.Tactile("Tearing down Python environment for %s", e.project.RepoName())

	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil
	}

	// Remove container (force)
	if err := e.executor.RemoveContainer(ctx, container.ID, true); err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	e.mu.Lock()
	e.container = nil
	e.state = StateComplete
	e.mu.Unlock()

	return nil
}

// Reset restores the environment to post-setup state.
func (e *Environment) Reset(ctx context.Context) error {
	e.mu.RLock()
	snapshot := e.setupSnapshot
	e.mu.RUnlock()

	if snapshot == nil {
		return fmt.Errorf("no setup snapshot available")
	}

	logging.Tactile("Resetting environment to post-setup state")

	// Stop current container
	if err := e.executor.StopContainer(ctx, e.container.ID, 10*time.Second); err != nil {
		logging.TactileWarn("Failed to stop container during reset: %v", err)
	}

	// Remove current container
	if err := e.executor.RemoveContainer(ctx, e.container.ID, true); err != nil {
		return fmt.Errorf("failed to remove container during reset: %w", err)
	}

	// Restore from snapshot
	newContainer, err := e.executor.RestoreSnapshot(ctx, snapshot.ID)
	if err != nil {
		e.setError(fmt.Errorf("failed to restore from snapshot: %w", err))
		return e.lastError
	}

	// Start restored container
	if err := e.executor.StartContainer(ctx, newContainer.ID); err != nil {
		e.setError(fmt.Errorf("failed to start restored container: %w", err))
		return e.lastError
	}

	e.mu.Lock()
	e.container = newContainer
	e.state = StateReady
	e.mu.Unlock()

	logging.Tactile("Environment reset complete")
	return nil
}

// =============================================================================
// REPOSITORY OPERATIONS
// =============================================================================

// CloneRepo clones the repository into the container.
func (e *Environment) CloneRepo(ctx context.Context) error {
	e.setState(StateCloning)
	logging.Tactile("Cloning repository: %s", e.project.GitURL)

	// Create workspace directory
	_, err := e.Exec(ctx, "mkdir", []string{"-p", e.config.WorkspaceDir})
	if err != nil {
		e.setError(fmt.Errorf("failed to create workspace: %w", err))
		return e.lastError
	}

	// Clone repository
	result, err := e.execWithTimeout(ctx, e.config.SetupTimeout,
		"git", "clone", "--depth", "1000", e.project.GitURL, e.repoPath)
	if err != nil {
		e.setError(fmt.Errorf("failed to clone repository: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("git clone failed: %s", result.Stderr))
		return e.lastError
	}

	logging.Tactile("Repository cloned to %s", e.repoPath)
	return nil
}

// CheckoutCommit checks out a specific commit.
func (e *Environment) CheckoutCommit(ctx context.Context, commit string) error {
	e.setState(StateCheckout)
	logging.Tactile("Checking out commit: %s", commit)

	// Fetch the specific commit if needed
	_, _ = e.execInRepo(ctx, 2*time.Minute, "git", "fetch", "origin", commit)

	// Checkout the commit
	result, err := e.execInRepo(ctx, 1*time.Minute, "git", "checkout", commit)
	if err != nil {
		e.setError(fmt.Errorf("failed to checkout: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("git checkout failed: %s", result.Stderr))
		return e.lastError
	}

	logging.Tactile("Checked out commit: %s", commit[:min(12, len(commit))])
	return nil
}

// CheckoutBranch checks out a specific branch.
func (e *Environment) CheckoutBranch(ctx context.Context, branch string) error {
	e.setState(StateCheckout)
	logging.Tactile("Checking out branch: %s", branch)

	result, err := e.execInRepo(ctx, 1*time.Minute, "git", "checkout", branch)
	if err != nil {
		e.setError(fmt.Errorf("failed to checkout branch: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("git checkout failed: %s", result.Stderr))
		return e.lastError
	}

	logging.Tactile("Checked out branch: %s", branch)
	return nil
}

// =============================================================================
// ENVIRONMENT SETUP
// =============================================================================

// SetupVirtualEnv creates a Python virtual environment.
func (e *Environment) SetupVirtualEnv(ctx context.Context) error {
	e.setState(StateSetup)
	logging.Tactile("Setting up virtual environment")

	// Create venv
	result, err := e.Exec(ctx, "python", []string{"-m", "venv", e.venvPath})
	if err != nil {
		e.setError(fmt.Errorf("failed to create venv: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("venv creation failed: %s", result.Stderr))
		return e.lastError
	}

	// Upgrade pip
	_, _ = e.ExecInVenv(ctx, "pip install --upgrade pip setuptools wheel")

	logging.Tactile("Virtual environment created at %s", e.venvPath)
	return nil
}

// InstallDependencies installs project dependencies.
func (e *Environment) InstallDependencies(ctx context.Context) error {
	logging.Tactile("Installing dependencies for %s", e.project.RepoName())

	// Try different dependency files in order
	depFiles := []struct {
		file    string
		command string
	}{
		{"pyproject.toml", "pip install -e .[dev,test]"},
		{"setup.py", "pip install -e .[dev,test]"},
		{"requirements.txt", "pip install -r requirements.txt"},
		{"requirements-dev.txt", "pip install -r requirements-dev.txt"},
		{"requirements-test.txt", "pip install -r requirements-test.txt"},
	}

	for _, dep := range depFiles {
		// Check if file exists
		checkResult, _ := e.execInRepo(ctx, 10*time.Second, "test", "-f", dep.file)
		if checkResult != nil && checkResult.ExitCode == 0 {
			logging.Tactile("Installing from %s", dep.file)

			// Install with timeout
			result, err := e.execInRepoVenv(ctx, e.config.SetupTimeout, dep.command)
			if err != nil {
				logging.TactileWarn("Install command failed: %v", err)
				continue
			}
			if result.ExitCode != 0 {
				logging.TactileWarn("Install exited non-zero: %s", result.Stderr)
				continue
			}

			logging.Tactile("Dependencies installed successfully")
			return nil
		}
	}

	// Fallback: try installing the package directly
	logging.Tactile("Attempting fallback: pip install -e .")
	result, err := e.execInRepoVenv(ctx, e.config.SetupTimeout, "pip install -e .")
	if err != nil || result.ExitCode != 0 {
		logging.TactileWarn("Fallback install failed, continuing anyway")
	}

	return nil
}

// =============================================================================
// PATCH OPERATIONS
// =============================================================================

// ApplyPatch applies a unified diff patch to the repository.
func (e *Environment) ApplyPatch(ctx context.Context, patch string) error {
	logging.Tactile("Applying patch (%d bytes)", len(patch))

	// Write patch to temp file
	patchPath := "/tmp/model.patch"
	writeResult, err := e.Exec(ctx, "sh", []string{"-c", fmt.Sprintf("cat > %s << 'PATCH_EOF'\n%s\nPATCH_EOF", patchPath, patch)})
	if err != nil {
		e.setError(fmt.Errorf("failed to write patch file: %w", err))
		return e.lastError
	}
	if writeResult.ExitCode != 0 {
		e.setError(fmt.Errorf("failed to write patch: %s", writeResult.Stderr))
		return e.lastError
	}

	// Apply patch
	result, err := e.execInRepo(ctx, 1*time.Minute, "git", "apply", "--verbose", patchPath)
	if err != nil {
		e.setError(fmt.Errorf("failed to apply patch: %w", err))
		return e.lastError
	}
	if result.ExitCode != 0 {
		e.setError(fmt.Errorf("patch apply failed: %s", result.Combined))
		return e.lastError
	}

	e.setState(StatePatchApplied)
	logging.Tactile("Patch applied successfully")
	return nil
}

// RevertChanges reverts any uncommitted changes.
func (e *Environment) RevertChanges(ctx context.Context) error {
	logging.Tactile("Reverting changes")

	result, err := e.execInRepo(ctx, 1*time.Minute, "git", "checkout", "--", ".")
	if err != nil {
		return fmt.Errorf("failed to revert: %w", err)
	}
	if result.ExitCode != 0 {
		return fmt.Errorf("git checkout failed: %s", result.Stderr)
	}

	e.setState(StateReady)
	return nil
}

// GetDiff returns the current diff in the repository.
func (e *Environment) GetDiff(ctx context.Context) (string, error) {
	result, err := e.execInRepo(ctx, 30*time.Second, "git", "diff")
	if err != nil {
		return "", err
	}
	return result.Stdout, nil
}

// =============================================================================
// TEST EXECUTION
// =============================================================================

// RunPytest runs pytest with the given arguments.
func (e *Environment) RunPytest(ctx context.Context, args ...string) (*TestResult, error) {
	logging.Tactile("Running pytest: %v", args)
	e.setState(StateTesting)

	startTime := time.Now()

	// Build pytest command
	command := "pytest -xvs"
	if len(args) > 0 {
		command = fmt.Sprintf("pytest -xvs %s", strings.Join(args, " "))
	}

	result, err := e.execInRepoVenv(ctx, e.config.TestTimeout, command)

	duration := time.Since(startTime)

	testResult := &TestResult{
		TestName: strings.Join(args, " "),
		Duration: duration,
	}

	if err != nil {
		testResult.Passed = false
		testResult.ErrorMessage = err.Error()
		testResult.ExitCode = -1
		return testResult, nil
	}

	testResult.Output = result.Combined
	testResult.ExitCode = result.ExitCode
	testResult.Passed = result.ExitCode == 0

	if !testResult.Passed {
		testResult.ErrorMessage = extractPytestError(result.Combined)
	}

	logging.Tactile("Test completed: passed=%v, duration=%s", testResult.Passed, duration.Round(time.Millisecond))
	return testResult, nil
}

// RunTest runs a single test and returns the result.
func (e *Environment) RunTest(ctx context.Context, testName string) (*TestResult, error) {
	return e.RunPytest(ctx, testName)
}

// RunTests runs multiple tests and returns all results.
func (e *Environment) RunTests(ctx context.Context, testNames []string) (map[string]*TestResult, error) {
	results := make(map[string]*TestResult)

	for _, testName := range testNames {
		result, err := e.RunTest(ctx, testName)
		if err != nil {
			return results, err
		}
		results[testName] = result
	}

	return results, nil
}

// RunAllTests runs all tests in the project.
func (e *Environment) RunAllTests(ctx context.Context) (*TestResult, error) {
	return e.RunPytest(ctx)
}

// =============================================================================
// COMMAND EXECUTION
// =============================================================================

// Exec executes a command in the container.
func (e *Environment) Exec(ctx context.Context, binary string, args []string) (*tactile.ExecutionResult, error) {
	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container not initialized")
	}

	return e.executor.ExecInContainer(ctx, tactile.ContainerExecOptions{
		ContainerID: container.ID,
		Binary:      binary,
		Arguments:   args,
		WorkingDir:  e.config.WorkspaceDir,
		Timeout:     2 * time.Minute,
	})
}

// ExecInVenv executes a command using the venv's Python.
func (e *Environment) ExecInVenv(ctx context.Context, command string) (*tactile.ExecutionResult, error) {
	venvBin := e.venvPath + "/bin"
	fullCommand := fmt.Sprintf("PATH=%s:$PATH %s", venvBin, command)

	return e.Exec(ctx, "sh", []string{"-c", fullCommand})
}

// ExecInRepo executes a command in the repository directory.
func (e *Environment) ExecInRepo(ctx context.Context, command string) (*tactile.ExecutionResult, error) {
	return e.execInRepoVenv(ctx, 2*time.Minute, command)
}

func (e *Environment) execWithTimeout(ctx context.Context, timeout time.Duration, binary string, args ...string) (*tactile.ExecutionResult, error) {
	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container not initialized")
	}

	return e.executor.ExecInContainer(ctx, tactile.ContainerExecOptions{
		ContainerID: container.ID,
		Binary:      binary,
		Arguments:   args,
		WorkingDir:  e.config.WorkspaceDir,
		Timeout:     timeout,
	})
}

func (e *Environment) execInRepo(ctx context.Context, timeout time.Duration, binary string, args ...string) (*tactile.ExecutionResult, error) {
	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container not initialized")
	}

	return e.executor.ExecInContainer(ctx, tactile.ContainerExecOptions{
		ContainerID: container.ID,
		Binary:      binary,
		Arguments:   args,
		WorkingDir:  e.repoPath,
		Timeout:     timeout,
	})
}

func (e *Environment) execInRepoVenv(ctx context.Context, timeout time.Duration, command string) (*tactile.ExecutionResult, error) {
	e.mu.RLock()
	container := e.container
	e.mu.RUnlock()

	if container == nil {
		return nil, fmt.Errorf("container not initialized")
	}

	venvBin := e.venvPath + "/bin"
	fullCommand := fmt.Sprintf("PATH=%s:$PATH %s", venvBin, command)

	return e.executor.ExecInContainer(ctx, tactile.ContainerExecOptions{
		ContainerID: container.ID,
		Binary:      "sh",
		Arguments:   []string{"-c", fullCommand},
		WorkingDir:  e.repoPath,
		Timeout:     timeout,
	})
}

// extractPytestError extracts a concise error message from pytest output.
func extractPytestError(output string) string {
	lines := strings.Split(output, "\n")
	for _, line := range lines {
		if strings.Contains(line, "AssertionError") ||
			strings.Contains(line, "Error:") ||
			strings.Contains(line, "FAILED") {
			return strings.TrimSpace(line)
		}
	}
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return strings.TrimSpace(lines[i])
		}
	}
	return "unknown error"
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
