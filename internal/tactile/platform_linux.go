//go:build linux

package tactile

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// Linux-specific rlimit that isn't available on macOS
const (
	RLIMIT_NPROC = 6 // Linux-specific
)

// createRlimits generates rlimit values from ResourceLimits (Linux version).
// Returns a map of resource type to rlimit struct.
func createRlimits(limits *ResourceLimits) map[int]syscall.Rlimit {
	rlimits := createRlimitsCommon(limits)

	if limits == nil {
		return rlimits
	}

	// Process limit (RLIMIT_NPROC) - Linux only
	if limits.MaxProcesses > 0 {
		rlimits[RLIMIT_NPROC] = syscall.Rlimit{
			Cur: uint64(limits.MaxProcesses),
			Max: uint64(limits.MaxProcesses),
		}
	}

	return rlimits
}

// LimitedExecutorLinux provides resource-limited execution on Linux systems.
// It uses setrlimit and cgroups where available.
type LimitedExecutorLinux struct {
	*DirectExecutor
	mu sync.RWMutex

	// useCgroups enables cgroup-based resource limits (requires root or cgroupfs mounted)
	useCgroups bool

	// cgroupPath is the path to the cgroup filesystem
	cgroupPath string

	// cgroupVersion is 1 or 2
	cgroupVersion int
}

// NewLimitedExecutorLinux creates a new resource-limited executor for Linux.
func NewLimitedExecutorLinux(config ExecutorConfig) *LimitedExecutorLinux {
	e := &LimitedExecutorLinux{
		DirectExecutor: NewDirectExecutorWithConfig(config),
		cgroupPath:     "/sys/fs/cgroup",
	}
	e.detectCgroups()
	return e
}

// detectCgroups checks if cgroups are available and writable.
func (e *LimitedExecutorLinux) detectCgroups() {
	// Check for cgroup v2 (unified hierarchy)
	if _, err := os.Stat("/sys/fs/cgroup/cgroup.controllers"); err == nil {
		e.cgroupVersion = 2
		e.cgroupPath = "/sys/fs/cgroup"
	} else if _, err := os.Stat("/sys/fs/cgroup/memory"); err == nil {
		e.cgroupVersion = 1
		e.cgroupPath = "/sys/fs/cgroup"
	} else {
		e.useCgroups = false
		return
	}

	// Check if we can write to cgroup (need root or appropriate permissions)
	testPath := filepath.Join(e.cgroupPath, "tactile_test_"+strconv.Itoa(os.Getpid()))
	if e.cgroupVersion == 2 {
		if err := os.MkdirAll(testPath, 0755); err == nil {
			os.RemoveAll(testPath)
			e.useCgroups = true
		}
	} else {
		// For cgroup v1, we need to check each controller
		memTestPath := filepath.Join(e.cgroupPath, "memory", "tactile_test")
		if err := os.MkdirAll(memTestPath, 0755); err == nil {
			os.RemoveAll(memTestPath)
			e.useCgroups = true
		}
	}
}

// UsesCgroups returns whether this executor is using cgroups.
func (e *LimitedExecutorLinux) UsesCgroups() bool {
	return e.useCgroups
}

// CgroupVersion returns the cgroup version (1 or 2), or 0 if not using cgroups.
func (e *LimitedExecutorLinux) CgroupVersion() int {
	if !e.useCgroups {
		return 0
	}
	return e.cgroupVersion
}

// Capabilities returns what this executor supports.
func (e *LimitedExecutorLinux) Capabilities() ExecutorCapabilities {
	caps := e.DirectExecutor.Capabilities()
	caps.Name = "limited-linux"
	caps.SupportsResourceLimits = true
	caps.SupportsResourceUsage = true
	return caps
}

// Execute runs a command with resource limits enforced via cgroups or rlimits.
func (e *LimitedExecutorLinux) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	// If no limits specified or cgroups not available, use parent implementation
	if cmd.Limits == nil || !e.useCgroups {
		return e.DirectExecutor.Execute(ctx, cmd)
	}

	// Merge config defaults
	cmd = e.config.Merge(cmd)

	// Create a unique cgroup for this execution
	cgroupName := fmt.Sprintf("tactile_%d_%d", os.Getpid(), time.Now().UnixNano())
	cgroup := NewCgroupManager(e.cgroupPath, cgroupName, e.cgroupVersion)

	// Set up the cgroup with limits
	if err := cgroup.Setup(cmd.Limits); err != nil {
		// Fall back to non-cgroup execution
		return e.DirectExecutor.Execute(ctx, cmd)
	}
	defer cgroup.Cleanup()

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
		ExecutorName: "limited-linux",
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
	execCmd.Env = e.buildEnvironment(cmd.Environment)

	// Set up stdin if provided
	if cmd.Stdin != "" {
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

	// Set up process group for clean killing
	setupProcessGroup(execCmd)

	// Record start time
	result.StartedAt = time.Now()

	// Start the command
	if err := execCmd.Start(); err != nil {
		result.Success = false
		result.Error = err.Error()
		result.FinishedAt = time.Now()
		result.Duration = result.FinishedAt.Sub(result.StartedAt)
		return result, nil
	}

	// Add process to cgroup
	if err := cgroup.AddProcess(execCmd.Process.Pid); err != nil {
		// Kill the process if we can't add it to cgroup
		execCmd.Process.Kill()
		result.Success = false
		result.Error = fmt.Sprintf("failed to add process to cgroup: %v", err)
		result.FinishedAt = time.Now()
		result.Duration = result.FinishedAt.Sub(result.StartedAt)
		return result, nil
	}

	// Wait for completion
	err := execCmd.Wait()

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

	// Get resource usage from cgroup
	if cgroupStats, err := cgroup.GetStats(); err == nil {
		result.ResourceUsage = cgroupStats
	} else {
		// Fall back to rusage
		result.ResourceUsage = getProcessResourceUsage(execCmd)
	}

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
		ExecutorName: "limited-linux",
	})

	return result, nil
}

// CgroupManager handles cgroup-based resource limits.
type CgroupManager struct {
	basePath string
	name     string
	version  int
}

// NewCgroupManager creates a new cgroup manager for a specific execution.
func NewCgroupManager(basePath, name string, version int) *CgroupManager {
	return &CgroupManager{
		basePath: basePath,
		name:     name,
		version:  version,
	}
}

// Setup creates the cgroup and configures limits.
func (c *CgroupManager) Setup(limits *ResourceLimits) error {
	if limits == nil {
		return nil
	}

	if c.version == 2 {
		return c.setupV2(limits)
	}
	return c.setupV1(limits)
}

// setupV2 sets up cgroup v2 limits.
func (c *CgroupManager) setupV2(limits *ResourceLimits) error {
	cgroupDir := filepath.Join(c.basePath, c.name)

	// Create the cgroup directory
	if err := os.MkdirAll(cgroupDir, 0755); err != nil {
		return fmt.Errorf("failed to create cgroup: %w", err)
	}

	// Enable controllers
	// First read available controllers
	controllers, err := os.ReadFile(filepath.Join(c.basePath, "cgroup.controllers"))
	if err == nil {
		// Enable memory, cpu, pids controllers if available
		needed := []string{}
		available := string(controllers)
		for _, ctrl := range []string{"memory", "cpu", "pids", "io"} {
			if strings.Contains(available, ctrl) {
				needed = append(needed, "+"+ctrl)
			}
		}
		if len(needed) > 0 {
			subtree := filepath.Join(c.basePath, "cgroup.subtree_control")
			os.WriteFile(subtree, []byte(strings.Join(needed, " ")), 0644)
		}
	}

	// Memory limit
	if limits.MaxMemoryBytes > 0 {
		memFile := filepath.Join(cgroupDir, "memory.max")
		if err := os.WriteFile(memFile, []byte(strconv.FormatInt(limits.MaxMemoryBytes, 10)), 0644); err != nil {
			return fmt.Errorf("failed to set memory limit: %w", err)
		}
		// Also set memory.high to throttle before hitting max
		memHigh := filepath.Join(cgroupDir, "memory.high")
		highLimit := limits.MaxMemoryBytes * 90 / 100 // 90% of max
		os.WriteFile(memHigh, []byte(strconv.FormatInt(highLimit, 10)), 0644)
	}

	// CPU limit (as bandwidth)
	if limits.MaxCPUTimeMs > 0 {
		// CPU bandwidth: quota/period microseconds
		// Set period to 100ms (100000us)
		period := int64(100000) // 100ms in microseconds
		// quota is how much CPU time per period
		// For limits, we interpret MaxCPUTimeMs as total allowed CPU time
		// So we set a low quota relative to period
		quota := int64(50000) // 50ms per 100ms period = 50% CPU

		cpuMax := filepath.Join(cgroupDir, "cpu.max")
		content := fmt.Sprintf("%d %d", quota, period)
		os.WriteFile(cpuMax, []byte(content), 0644)
	}

	// Process limit
	if limits.MaxProcesses > 0 {
		pidsMax := filepath.Join(cgroupDir, "pids.max")
		if err := os.WriteFile(pidsMax, []byte(strconv.Itoa(limits.MaxProcesses)), 0644); err != nil {
			// Non-fatal, continue
		}
	}

	return nil
}

// setupV1 sets up cgroup v1 limits.
func (c *CgroupManager) setupV1(limits *ResourceLimits) error {
	// For cgroup v1, we need to create directories in each controller
	controllers := []string{"memory", "cpu", "pids"}

	for _, ctrl := range controllers {
		cgroupDir := filepath.Join(c.basePath, ctrl, c.name)
		if err := os.MkdirAll(cgroupDir, 0755); err != nil {
			continue // Controller might not exist
		}
	}

	// Memory limit
	if limits.MaxMemoryBytes > 0 {
		memDir := filepath.Join(c.basePath, "memory", c.name)
		memLimit := filepath.Join(memDir, "memory.limit_in_bytes")
		os.WriteFile(memLimit, []byte(strconv.FormatInt(limits.MaxMemoryBytes, 10)), 0644)
	}

	// CPU limit (as CFS bandwidth)
	if limits.MaxCPUTimeMs > 0 {
		cpuDir := filepath.Join(c.basePath, "cpu", c.name)
		os.WriteFile(filepath.Join(cpuDir, "cpu.cfs_period_us"), []byte("100000"), 0644)
		os.WriteFile(filepath.Join(cpuDir, "cpu.cfs_quota_us"), []byte("50000"), 0644)
	}

	// Process limit
	if limits.MaxProcesses > 0 {
		pidsDir := filepath.Join(c.basePath, "pids", c.name)
		pidsMax := filepath.Join(pidsDir, "pids.max")
		os.WriteFile(pidsMax, []byte(strconv.Itoa(limits.MaxProcesses)), 0644)
	}

	return nil
}

// AddProcess adds a process to this cgroup.
func (c *CgroupManager) AddProcess(pid int) error {
	pidStr := strconv.Itoa(pid)

	if c.version == 2 {
		cgroupDir := filepath.Join(c.basePath, c.name)
		procsFile := filepath.Join(cgroupDir, "cgroup.procs")
		return os.WriteFile(procsFile, []byte(pidStr), 0644)
	}

	// For cgroup v1, add to each controller
	controllers := []string{"memory", "cpu", "pids"}
	var lastErr error
	for _, ctrl := range controllers {
		cgroupDir := filepath.Join(c.basePath, ctrl, c.name)
		tasksFile := filepath.Join(cgroupDir, "tasks")
		if err := os.WriteFile(tasksFile, []byte(pidStr), 0644); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// GetStats retrieves resource usage statistics from the cgroup.
func (c *CgroupManager) GetStats() (*ResourceUsage, error) {
	if c.version == 2 {
		return c.getStatsV2()
	}
	return c.getStatsV1()
}

// getStatsV2 retrieves stats from cgroup v2.
func (c *CgroupManager) getStatsV2() (*ResourceUsage, error) {
	cgroupDir := filepath.Join(c.basePath, c.name)
	usage := &ResourceUsage{}

	// Read memory usage
	memCurrent := filepath.Join(cgroupDir, "memory.current")
	if data, err := os.ReadFile(memCurrent); err == nil {
		if val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			usage.MaxRSSBytes = val
		}
	}

	// Read peak memory
	memPeak := filepath.Join(cgroupDir, "memory.peak")
	if data, err := os.ReadFile(memPeak); err == nil {
		if val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			if val > usage.MaxRSSBytes {
				usage.MaxRSSBytes = val
			}
		}
	}

	// Read CPU usage
	cpuStat := filepath.Join(cgroupDir, "cpu.stat")
	if data, err := os.ReadFile(cpuStat); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			parts := strings.Fields(line)
			if len(parts) != 2 {
				continue
			}
			val, _ := strconv.ParseInt(parts[1], 10, 64)
			switch parts[0] {
			case "usage_usec":
				// Total CPU time
				usage.UserTimeMs = val / 1000
			case "user_usec":
				usage.UserTimeMs = val / 1000
			case "system_usec":
				usage.SystemTimeMs = val / 1000
			}
		}
	}

	// Read I/O stats
	ioStat := filepath.Join(cgroupDir, "io.stat")
	if data, err := os.ReadFile(ioStat); err == nil {
		lines := strings.Split(string(data), "\n")
		for _, line := range lines {
			for _, field := range strings.Fields(line) {
				if strings.HasPrefix(field, "rbytes=") {
					val, _ := strconv.ParseInt(strings.TrimPrefix(field, "rbytes="), 10, 64)
					usage.DiskReadBytes += val
				} else if strings.HasPrefix(field, "wbytes=") {
					val, _ := strconv.ParseInt(strings.TrimPrefix(field, "wbytes="), 10, 64)
					usage.DiskWriteBytes += val
				}
			}
		}
	}

	return usage, nil
}

// getStatsV1 retrieves stats from cgroup v1.
func (c *CgroupManager) getStatsV1() (*ResourceUsage, error) {
	usage := &ResourceUsage{}

	// Read memory usage
	memUsage := filepath.Join(c.basePath, "memory", c.name, "memory.usage_in_bytes")
	if data, err := os.ReadFile(memUsage); err == nil {
		if val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			usage.MaxRSSBytes = val
		}
	}

	// Read max memory
	memMax := filepath.Join(c.basePath, "memory", c.name, "memory.max_usage_in_bytes")
	if data, err := os.ReadFile(memMax); err == nil {
		if val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			if val > usage.MaxRSSBytes {
				usage.MaxRSSBytes = val
			}
		}
	}

	// Read CPU usage
	cpuUsage := filepath.Join(c.basePath, "cpuacct", c.name, "cpuacct.usage")
	if data, err := os.ReadFile(cpuUsage); err == nil {
		if val, err := strconv.ParseInt(strings.TrimSpace(string(data)), 10, 64); err == nil {
			// cpuacct.usage is in nanoseconds
			usage.UserTimeMs = val / 1000000
		}
	}

	return usage, nil
}

// Cleanup removes the cgroup.
func (c *CgroupManager) Cleanup() error {
	if c.version == 2 {
		cgroupDir := filepath.Join(c.basePath, c.name)
		// Kill any remaining processes
		c.killProcesses(cgroupDir)
		return os.RemoveAll(cgroupDir)
	}

	// For cgroup v1, remove from each controller
	controllers := []string{"memory", "cpu", "pids", "cpuacct"}
	for _, ctrl := range controllers {
		cgroupDir := filepath.Join(c.basePath, ctrl, c.name)
		c.killProcesses(cgroupDir)
		os.RemoveAll(cgroupDir)
	}
	return nil
}

// killProcesses kills all processes in a cgroup directory.
func (c *CgroupManager) killProcesses(cgroupDir string) {
	procsFile := filepath.Join(cgroupDir, "cgroup.procs")
	if c.version == 1 {
		procsFile = filepath.Join(cgroupDir, "tasks")
	}

	if data, err := os.ReadFile(procsFile); err == nil {
		for _, pidStr := range strings.Fields(string(data)) {
			if pid, err := strconv.Atoi(pidStr); err == nil && pid > 0 {
				syscall.Kill(pid, syscall.SIGKILL)
			}
		}
	}
}

// NamespaceConfig configures Linux namespace isolation.
type NamespaceConfig struct {
	// NewPID creates a new PID namespace
	NewPID bool

	// NewNet creates a new network namespace
	NewNet bool

	// NewMount creates a new mount namespace
	NewMount bool

	// NewUTS creates a new UTS namespace (hostname)
	NewUTS bool

	// NewIPC creates a new IPC namespace
	NewIPC bool

	// NewUser creates a new user namespace
	NewUser bool

	// Hostname sets the hostname in the new UTS namespace
	Hostname string

	// BindMounts is a list of paths to bind mount
	BindMounts []BindMount
}

// NamespaceExecutor uses Linux namespaces for isolation.
// This requires either root privileges or user namespaces.
type NamespaceExecutor struct {
	*DirectExecutor
	mu sync.RWMutex

	// defaultConfig is the default namespace configuration
	defaultConfig NamespaceConfig
}

// NewNamespaceExecutor creates a new namespace-based executor.
func NewNamespaceExecutor(config ExecutorConfig) *NamespaceExecutor {
	return &NamespaceExecutor{
		DirectExecutor: NewDirectExecutorWithConfig(config),
		defaultConfig: NamespaceConfig{
			NewPID:   true,
			NewNet:   true,
			NewMount: true,
			NewUTS:   true,
			NewIPC:   true,
			Hostname: "sandbox",
		},
	}
}

// Capabilities returns what this executor supports.
func (e *NamespaceExecutor) Capabilities() ExecutorCapabilities {
	caps := e.DirectExecutor.Capabilities()
	caps.Name = "namespace"
	caps.SupportedSandboxModes = []SandboxMode{SandboxNone, SandboxNamespace}
	caps.SupportsNetworkIsolation = true
	return caps
}

// Execute runs a command with namespace isolation.
func (e *NamespaceExecutor) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	// Check if namespace isolation is requested
	if cmd.Sandbox == nil || cmd.Sandbox.Mode != SandboxNamespace {
		return e.DirectExecutor.Execute(ctx, cmd)
	}

	// Merge config defaults
	cmd = e.config.Merge(cmd)

	// Prepare the result
	result := &ExecutionResult{
		ExitCode:    -1,
		SandboxUsed: SandboxNamespace,
		Command:     &cmd,
	}

	// Emit start event
	e.emitAudit(AuditEvent{
		Type:         AuditEventStart,
		Timestamp:    time.Now(),
		Command:      cmd,
		SessionID:    cmd.SessionID,
		ExecutorName: "namespace",
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
	execCmd.Env = e.buildEnvironment(cmd.Environment)

	// Set up namespace isolation
	nsConfig := e.defaultConfig

	// Check if network should be allowed
	if cmd.Limits != nil && cmd.Limits.NetworkAllowed != nil && *cmd.Limits.NetworkAllowed {
		nsConfig.NewNet = false
	}

	// Set up SysProcAttr with clone flags
	execCmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: buildCloneFlags(nsConfig),
		Setpgid:    true,
	}

	// If using user namespace, set up UID/GID mapping
	if nsConfig.NewUser {
		execCmd.SysProcAttr.UidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		}
		execCmd.SysProcAttr.GidMappings = []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		}
	}

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
		ExecutorName: "namespace",
	})

	return result, nil
}

// buildCloneFlags creates the clone flags for namespace isolation.
func buildCloneFlags(nsConfig NamespaceConfig) uintptr {
	var flags uintptr = 0

	if nsConfig.NewPID {
		flags |= syscall.CLONE_NEWPID
	}
	if nsConfig.NewNet {
		flags |= syscall.CLONE_NEWNET
	}
	if nsConfig.NewMount {
		flags |= syscall.CLONE_NEWNS
	}
	if nsConfig.NewUTS {
		flags |= syscall.CLONE_NEWUTS
	}
	if nsConfig.NewIPC {
		flags |= syscall.CLONE_NEWIPC
	}
	if nsConfig.NewUser {
		flags |= syscall.CLONE_NEWUSER
	}

	return flags
}

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

// Execute runs a command inside a Firejail sandbox.
func (e *FirejailExecutor) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
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

// GetPlatformExecutor returns the best executor for this Linux system.
func GetPlatformExecutor(config ExecutorConfig) Executor {
	// Try in order of preference: Firejail, Namespace, Limited, Direct

	// Check for Firejail
	fj := NewFirejailExecutor(config)
	if fj.IsAvailable() {
		return fj
	}

	// Check if we can use namespaces (need CAP_SYS_ADMIN or user namespaces)
	if os.Getuid() == 0 {
		return NewNamespaceExecutor(config)
	}

	// Check if user namespaces are enabled
	if canUseUserNamespaces() {
		return NewNamespaceExecutor(config)
	}

	// Fall back to cgroup-limited executor
	limited := NewLimitedExecutorLinux(config)
	if limited.UsesCgroups() {
		return limited
	}

	// Ultimate fallback: direct execution
	return NewDirectExecutorWithConfig(config)
}

// canUseUserNamespaces checks if unprivileged user namespaces are enabled.
func canUseUserNamespaces() bool {
	data, err := os.ReadFile("/proc/sys/kernel/unprivileged_userns_clone")
	if err != nil {
		// File doesn't exist on some kernels; try to create a namespace
		return testUserNamespace()
	}

	val := strings.TrimSpace(string(data))
	return val == "1"
}

// testUserNamespace attempts to create a user namespace to test availability.
func testUserNamespace() bool {
	// Fork a process with CLONE_NEWUSER to test
	cmd := exec.Command("/bin/true")
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Cloneflags: syscall.CLONE_NEWUSER,
		UidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getuid(), Size: 1},
		},
		GidMappings: []syscall.SysProcIDMap{
			{ContainerID: 0, HostID: os.Getgid(), Size: 1},
		},
	}

	return cmd.Run() == nil
}
