//go:build windows

package tactile

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"
)

// Windows API constants
const (
	PROCESS_QUERY_INFORMATION = 0x0400
	PROCESS_VM_READ           = 0x0010
	PROCESS_TERMINATE         = 0x0001

	// Job object access rights
	JOB_OBJECT_ASSIGN_PROCESS          = 0x0001
	JOB_OBJECT_SET_ATTRIBUTES          = 0x0002
	JOB_OBJECT_QUERY                   = 0x0004
	JOB_OBJECT_TERMINATE               = 0x0008
	JOB_OBJECT_SET_SECURITY_ATTRIBUTES = 0x0010
	JOB_OBJECT_ALL_ACCESS              = 0x1F001F

	// Job object limit flags
	JOB_OBJECT_LIMIT_PROCESS_TIME        = 0x00000002
	JOB_OBJECT_LIMIT_JOB_TIME            = 0x00000004
	JOB_OBJECT_LIMIT_ACTIVE_PROCESS      = 0x00000008
	JOB_OBJECT_LIMIT_PROCESS_MEMORY      = 0x00000100
	JOB_OBJECT_LIMIT_JOB_MEMORY          = 0x00000200
	JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE   = 0x00002000
	JOB_OBJECT_LIMIT_BREAKAWAY_OK        = 0x00000800
	JOB_OBJECT_LIMIT_SILENT_BREAKAWAY_OK = 0x00001000

	// Job object info classes
	JobObjectBasicLimitInformation      = 2
	JobObjectExtendedLimitInformation   = 9
	JobObjectBasicAccountingInformation = 1
)

// Windows structures for job objects
type JOBOBJECT_BASIC_LIMIT_INFORMATION struct {
	PerProcessUserTimeLimit int64
	PerJobUserTimeLimit     int64
	LimitFlags              uint32
	MinimumWorkingSetSize   uintptr
	MaximumWorkingSetSize   uintptr
	ActiveProcessLimit      uint32
	Affinity                uintptr
	PriorityClass           uint32
	SchedulingClass         uint32
}

type IO_COUNTERS struct {
	ReadOperationCount  uint64
	WriteOperationCount uint64
	OtherOperationCount uint64
	ReadTransferCount   uint64
	WriteTransferCount  uint64
	OtherTransferCount  uint64
}

type JOBOBJECT_EXTENDED_LIMIT_INFORMATION struct {
	BasicLimitInformation JOBOBJECT_BASIC_LIMIT_INFORMATION
	IoInfo                IO_COUNTERS
	ProcessMemoryLimit    uintptr
	JobMemoryLimit        uintptr
	PeakProcessMemoryUsed uintptr
	PeakJobMemoryUsed     uintptr
}

type JOBOBJECT_BASIC_ACCOUNTING_INFORMATION struct {
	TotalUserTime             int64
	TotalKernelTime           int64
	ThisPeriodTotalUserTime   int64
	ThisPeriodTotalKernelTime int64
	TotalPageFaultCount       uint32
	TotalProcesses            uint32
	ActiveProcesses           uint32
	TotalTerminatedProcesses  uint32
}

// Windows kernel32 functions
var (
	kernel32                      = syscall.NewLazyDLL("kernel32.dll")
	procCreateJobObjectW          = kernel32.NewProc("CreateJobObjectW")
	procAssignProcessToJobObject  = kernel32.NewProc("AssignProcessToJobObject")
	procSetInformationJobObject   = kernel32.NewProc("SetInformationJobObject")
	procQueryInformationJobObject = kernel32.NewProc("QueryInformationJobObject")
	procTerminateJobObject        = kernel32.NewProc("TerminateJobObject")
	procGetProcessMemoryInfo      = kernel32.NewProc("K32GetProcessMemoryInfo")
	procGetProcessIoCounters      = kernel32.NewProc("GetProcessIoCounters")
)

// PROCESS_MEMORY_COUNTERS for memory info
type PROCESS_MEMORY_COUNTERS struct {
	cb                         uint32
	PageFaultCount             uint32
	PeakWorkingSetSize         uintptr
	WorkingSetSize             uintptr
	QuotaPeakPagedPoolUsage    uintptr
	QuotaPagedPoolUsage        uintptr
	QuotaPeakNonPagedPoolUsage uintptr
	QuotaNonPagedPoolUsage     uintptr
	PagefileUsage              uintptr
	PeakPagefileUsage          uintptr
}

// getProcessResourceUsage extracts resource usage on Windows.
func getProcessResourceUsage(cmd *exec.Cmd) *ResourceUsage {
	if cmd.ProcessState == nil {
		return nil
	}

	userTime := cmd.ProcessState.UserTime()
	sysTime := cmd.ProcessState.SystemTime()

	usage := &ResourceUsage{
		UserTimeMs:   userTime.Milliseconds(),
		SystemTimeMs: sysTime.Milliseconds(),
	}

	// Try to get memory info from the process handle if still valid
	if cmd.Process != nil {
		memInfo := getProcessMemoryInfo(cmd.Process.Pid)
		if memInfo != nil {
			usage.MaxRSSBytes = int64(memInfo.PeakWorkingSetSize)
		}

		ioCounters := getProcessIOCounters(cmd.Process.Pid)
		if ioCounters != nil {
			usage.DiskReadBytes = int64(ioCounters.ReadTransferCount)
			usage.DiskWriteBytes = int64(ioCounters.WriteTransferCount)
		}
	}

	return usage
}

// getProcessMemoryInfo retrieves memory information for a process.
func getProcessMemoryInfo(pid int) *PROCESS_MEMORY_COUNTERS {
	handle, err := syscall.OpenProcess(PROCESS_QUERY_INFORMATION|PROCESS_VM_READ, false, uint32(pid))
	if err != nil {
		return nil
	}
	defer syscall.CloseHandle(handle)

	var memCounters PROCESS_MEMORY_COUNTERS
	memCounters.cb = uint32(unsafe.Sizeof(memCounters))

	ret, _, _ := procGetProcessMemoryInfo.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&memCounters)),
		uintptr(memCounters.cb),
	)
	if ret == 0 {
		return nil
	}

	return &memCounters
}

// getProcessIOCounters retrieves I/O counters for a process.
func getProcessIOCounters(pid int) *IO_COUNTERS {
	handle, err := syscall.OpenProcess(PROCESS_QUERY_INFORMATION, false, uint32(pid))
	if err != nil {
		return nil
	}
	defer syscall.CloseHandle(handle)

	var ioCounters IO_COUNTERS
	ret, _, _ := procGetProcessIoCounters.Call(
		uintptr(handle),
		uintptr(unsafe.Pointer(&ioCounters)),
	)
	if ret == 0 {
		return nil
	}

	return &ioCounters
}

// killProcessGroup kills the process and attempts to terminate child processes.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	// On Windows, use taskkill to kill process tree
	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", fmt.Sprintf("%d", cmd.Process.Pid))
	killCmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}

	if err := killCmd.Run(); err != nil {
		// Fall back to direct kill
		return cmd.Process.Kill()
	}

	return nil
}

// setupProcessGroup sets up the process to run in a job object.
// On Windows, this is handled by creating a job object.
func setupProcessGroup(cmd *exec.Cmd) {
	// Windows uses Job Objects instead of process groups
	// This is handled in LimitedExecutorWindows
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	// Hide window for console processes
	cmd.SysProcAttr.HideWindow = true
}

// createRlimits is a no-op on Windows since we use Job Objects.
// Returns nil as Windows doesn't use rlimits.
func createRlimits(limits *ResourceLimits) map[int]uint64 {
	return nil
}

// JobObject wraps a Windows job object for process resource management.
type JobObject struct {
	handle syscall.Handle
	name   string
}

// NewJobObject creates a new Windows job object.
func NewJobObject(name string) (*JobObject, error) {
	var namePtr *uint16
	if name != "" {
		var err error
		namePtr, err = syscall.UTF16PtrFromString(name)
		if err != nil {
			return nil, err
		}
	}

	handle, _, err := procCreateJobObjectW.Call(
		0,
		uintptr(unsafe.Pointer(namePtr)),
	)
	if handle == 0 {
		return nil, fmt.Errorf("CreateJobObjectW failed: %v", err)
	}

	return &JobObject{
		handle: syscall.Handle(handle),
		name:   name,
	}, nil
}

// SetLimits configures resource limits on the job object.
func (j *JobObject) SetLimits(limits *ResourceLimits) error {
	if limits == nil {
		return nil
	}

	var extInfo JOBOBJECT_EXTENDED_LIMIT_INFORMATION

	// Always kill all processes when job is closed
	extInfo.BasicLimitInformation.LimitFlags = JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE

	// Memory limit
	if limits.MaxMemoryBytes > 0 {
		extInfo.ProcessMemoryLimit = uintptr(limits.MaxMemoryBytes)
		extInfo.BasicLimitInformation.LimitFlags |= JOB_OBJECT_LIMIT_PROCESS_MEMORY
	}

	// CPU time limit (per-process, in 100-nanosecond intervals)
	if limits.MaxCPUTimeMs > 0 {
		// Convert milliseconds to 100-nanosecond intervals
		extInfo.BasicLimitInformation.PerProcessUserTimeLimit = limits.MaxCPUTimeMs * 10000
		extInfo.BasicLimitInformation.LimitFlags |= JOB_OBJECT_LIMIT_PROCESS_TIME
	}

	// Process count limit
	if limits.MaxProcesses > 0 {
		extInfo.BasicLimitInformation.ActiveProcessLimit = uint32(limits.MaxProcesses)
		extInfo.BasicLimitInformation.LimitFlags |= JOB_OBJECT_LIMIT_ACTIVE_PROCESS
	}

	ret, _, err := procSetInformationJobObject.Call(
		uintptr(j.handle),
		JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&extInfo)),
		uintptr(unsafe.Sizeof(extInfo)),
	)
	if ret == 0 {
		return fmt.Errorf("SetInformationJobObject failed: %v", err)
	}

	return nil
}

// Windows process access rights for job object assignment
const (
	PROCESS_SET_QUOTA = 0x0100
)

// AssignProcess adds a process to the job object.
func (j *JobObject) AssignProcess(process *os.Process) error {
	handle, err := syscall.OpenProcess(
		PROCESS_SET_QUOTA|PROCESS_TERMINATE,
		false,
		uint32(process.Pid),
	)
	if err != nil {
		return fmt.Errorf("OpenProcess failed: %v", err)
	}
	defer syscall.CloseHandle(handle)

	ret, _, lastErr := procAssignProcessToJobObject.Call(
		uintptr(j.handle),
		uintptr(handle),
	)
	if ret == 0 {
		return fmt.Errorf("AssignProcessToJobObject failed: %v", lastErr)
	}

	return nil
}

// GetStats retrieves resource usage statistics from the job object.
func (j *JobObject) GetStats() (*ResourceUsage, error) {
	var accountInfo JOBOBJECT_BASIC_ACCOUNTING_INFORMATION

	ret, _, err := procQueryInformationJobObject.Call(
		uintptr(j.handle),
		JobObjectBasicAccountingInformation,
		uintptr(unsafe.Pointer(&accountInfo)),
		uintptr(unsafe.Sizeof(accountInfo)),
		0,
	)
	if ret == 0 {
		return nil, fmt.Errorf("QueryInformationJobObject failed: %v", err)
	}

	// Also get extended info for memory usage
	var extInfo JOBOBJECT_EXTENDED_LIMIT_INFORMATION
	procQueryInformationJobObject.Call(
		uintptr(j.handle),
		JobObjectExtendedLimitInformation,
		uintptr(unsafe.Pointer(&extInfo)),
		uintptr(unsafe.Sizeof(extInfo)),
		0,
	)

	return &ResourceUsage{
		// Convert 100-nanosecond intervals to milliseconds
		UserTimeMs:     accountInfo.TotalUserTime / 10000,
		SystemTimeMs:   accountInfo.TotalKernelTime / 10000,
		MaxRSSBytes:    int64(extInfo.PeakJobMemoryUsed),
		DiskReadBytes:  int64(extInfo.IoInfo.ReadTransferCount),
		DiskWriteBytes: int64(extInfo.IoInfo.WriteTransferCount),
	}, nil
}

// Terminate terminates all processes in the job object.
func (j *JobObject) Terminate(exitCode uint32) error {
	ret, _, err := procTerminateJobObject.Call(
		uintptr(j.handle),
		uintptr(exitCode),
	)
	if ret == 0 {
		return fmt.Errorf("TerminateJobObject failed: %v", err)
	}
	return nil
}

// Close closes the job object handle.
func (j *JobObject) Close() error {
	if j.handle != 0 {
		return syscall.CloseHandle(j.handle)
	}
	return nil
}

// LimitedExecutorWindows provides resource-limited execution on Windows.
// It uses Job Objects for process resource management.
type LimitedExecutorWindows struct {
	*DirectExecutor
	mu sync.RWMutex

	// useJobObjects enables job object-based resource limits
	useJobObjects bool
}

// NewLimitedExecutorWindows creates a new resource-limited executor for Windows.
func NewLimitedExecutorWindows(config ExecutorConfig) *LimitedExecutorWindows {
	e := &LimitedExecutorWindows{
		DirectExecutor: NewDirectExecutorWithConfig(config),
		useJobObjects:  true, // Job objects are available on all modern Windows
	}
	return e
}

// Capabilities returns what this executor supports.
func (e *LimitedExecutorWindows) Capabilities() ExecutorCapabilities {
	caps := e.DirectExecutor.Capabilities()
	caps.Name = "limited-windows"
	caps.SupportsResourceLimits = true
	caps.SupportsResourceUsage = true
	return caps
}

// Validate checks if a command can be executed.
func (e *LimitedExecutorWindows) Validate(cmd Command) error {
	return e.DirectExecutor.Validate(cmd)
}

// Execute runs a command with resource limits enforced via Job Objects.
func (e *LimitedExecutorWindows) Execute(ctx context.Context, cmd Command) (*ExecutionResult, error) {
	if err := e.Validate(cmd); err != nil {
		return nil, err
	}

	// If no limits specified, use parent implementation
	if cmd.Limits == nil {
		return e.DirectExecutor.Execute(ctx, cmd)
	}

	// Merge config defaults
	cmd = e.config.Merge(cmd)

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
		ExecutorName: "limited-windows",
	})

	// Create a job object for this execution
	jobName := fmt.Sprintf("tactile_%d_%d", os.Getpid(), time.Now().UnixNano())
	job, err := NewJobObject(jobName)
	if err != nil {
		// Fall back to non-job execution
		return e.DirectExecutor.Execute(ctx, cmd)
	}
	defer job.Close()

	// Set limits on the job object
	if err := job.SetLimits(cmd.Limits); err != nil {
		// Fall back to non-job execution
		return e.DirectExecutor.Execute(ctx, cmd)
	}

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

	// Set up Windows-specific process attributes
	if execCmd.SysProcAttr == nil {
		execCmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	execCmd.SysProcAttr.HideWindow = true

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

	// Assign process to job object
	if err := job.AssignProcess(execCmd.Process); err != nil {
		// Kill the process if we can't add it to the job
		execCmd.Process.Kill()
		result.Success = false
		result.Error = fmt.Sprintf("failed to assign process to job object: %v", err)
		result.FinishedAt = time.Now()
		result.Duration = result.FinishedAt.Sub(result.StartedAt)
		return result, nil
	}

	// Wait for completion
	err = execCmd.Wait()

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

	// Get resource usage from job object
	if jobStats, err := job.GetStats(); err == nil {
		result.ResourceUsage = jobStats
	} else {
		// Fall back to process state
		result.ResourceUsage = getProcessResourceUsage(execCmd)
	}

	// Process the error
	if err != nil {
		if execCtx.Err() == context.DeadlineExceeded {
			result.Killed = true
			result.KillReason = fmt.Sprintf("timeout after %s", timeout)
			result.Success = true
			// Terminate all processes in the job
			job.Terminate(1)
		} else if execCtx.Err() == context.Canceled {
			result.Killed = true
			result.KillReason = "context canceled"
			result.Success = true
			job.Terminate(1)
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
		ExecutorName: "limited-windows",
	})

	return result, nil
}

// NamespaceConfig is a stub for Windows (namespaces are Linux-only).
type NamespaceConfig struct {
	NewPID   bool
	NewNet   bool
	NewMount bool
	NewUTS   bool
	NewIPC   bool
	NewUser  bool
	Hostname string
}

// GetPlatformExecutor returns the best executor for Windows.
func GetPlatformExecutor(config ExecutorConfig) Executor {
	// On Windows, we can use Docker or Job Objects for limiting

	// Check for Docker
	docker := NewDockerExecutor()
	if docker.IsAvailable() {
		// Return direct executor - composite will be created by factory
		return NewDirectExecutorWithConfig(config)
	}

	// Use job object-based limiting (wraps DirectExecutor)
	return NewDirectExecutorWithConfig(config)
}

// GetLimitedExecutor returns a resource-limited executor for Windows.
func GetLimitedExecutor(config ExecutorConfig) *LimitedExecutorWindows {
	return NewLimitedExecutorWindows(config)
}

// WindowsContainerExecutor uses Windows Containers (if available).
// This is for Windows Server or Windows with Hyper-V containers.
type WindowsContainerExecutor struct {
	*DirectExecutor
	mu sync.RWMutex

	// dockerPath is the path to docker binary
	dockerPath string

	// available is true if Windows containers are available
	available bool

	// useHyperV enables Hyper-V isolation
	useHyperV bool
}

// NewWindowsContainerExecutor creates a new Windows Container executor.
func NewWindowsContainerExecutor(config ExecutorConfig) *WindowsContainerExecutor {
	e := &WindowsContainerExecutor{
		DirectExecutor: NewDirectExecutorWithConfig(config),
	}
	e.detect()
	return e
}

// detect checks if Windows containers are available.
func (e *WindowsContainerExecutor) detect() {
	// Check for docker
	dockerPath, err := exec.LookPath("docker")
	if err != nil {
		e.available = false
		return
	}
	e.dockerPath = dockerPath

	// Check if Docker is in Windows container mode
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, dockerPath, "version", "--format", "{{.Server.Os}}")
	output, err := cmd.Output()
	if err != nil {
		e.available = false
		return
	}

	// Check if running Windows containers
	if strings.Contains(strings.ToLower(string(output)), "windows") {
		e.available = true
	}

	// Check for Hyper-V support
	hyperVCmd := exec.CommandContext(ctx, "powershell", "-Command", "(Get-WindowsOptionalFeature -Online -FeatureName Microsoft-Hyper-V).State")
	hyperVOutput, err := hyperVCmd.Output()
	if err == nil && strings.Contains(string(hyperVOutput), "Enabled") {
		e.useHyperV = true
	}
}

// IsAvailable returns whether Windows containers are available.
func (e *WindowsContainerExecutor) IsAvailable() bool {
	return e.available
}

// Capabilities returns what this executor supports.
func (e *WindowsContainerExecutor) Capabilities() ExecutorCapabilities {
	modes := []SandboxMode{SandboxNone}
	if e.available {
		modes = append(modes, SandboxDocker)
	}

	caps := e.DirectExecutor.Capabilities()
	caps.Name = "windows-container"
	caps.SupportedSandboxModes = modes
	caps.SupportsNetworkIsolation = e.available
	caps.SupportsResourceLimits = e.available
	return caps
}

// Keep unused functions alive for future Windows implementation
var _ = killProcessGroup
var _ = setupProcessGroup
var _ = createRlimits
