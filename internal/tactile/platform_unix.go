//go:build !windows

package tactile

import (
	"os/exec"
	"strings"
	"syscall"
)

// getProcessResourceUsage extracts resource usage on Unix systems.
func getProcessResourceUsage(cmd *exec.Cmd) *ResourceUsage {
	if cmd.ProcessState == nil {
		return nil
	}

	rusage, ok := cmd.ProcessState.SysUsage().(*syscall.Rusage)
	if !ok || rusage == nil {
		return nil
	}

	return &ResourceUsage{
		UserTimeMs:                 rusage.Utime.Sec*1000 + int64(rusage.Utime.Usec/1000),
		SystemTimeMs:               rusage.Stime.Sec*1000 + int64(rusage.Stime.Usec/1000),
		MaxRSSBytes:                getMaxRSSBytes(rusage),
		VoluntaryContextSwitches:   int64(rusage.Nvcsw),
		InvoluntaryContextSwitches: int64(rusage.Nivcsw),
		DiskReadBytes:              int64(rusage.Inblock) * 512, // Block size is typically 512 bytes
		DiskWriteBytes:             int64(rusage.Oublock) * 512,
	}
}

// setupProcessGroup configures the command to run in its own process group.
// This allows killing all child processes when the parent is terminated.
func setupProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup kills the process and all its children on Unix.
func killProcessGroup(cmd *exec.Cmd) error {
	if cmd.Process == nil {
		return nil
	}

	pid := cmd.Process.Pid

	// First try to get the process group ID
	pgid, err := syscall.Getpgid(pid)
	if err == nil && pgid > 0 {
		// Kill the entire process group
		if err := syscall.Kill(-pgid, syscall.SIGKILL); err != nil {
			// If SIGKILL to group fails, try SIGTERM first
			syscall.Kill(-pgid, syscall.SIGTERM)
		}
	}

	// Also kill the main process directly as a fallback
	if err := cmd.Process.Kill(); err != nil {
		// Process might already be dead
		if !strings.Contains(err.Error(), "process already finished") {
			return err
		}
	}

	return nil
}

// createRlimitsCommon generates rlimit values that work on both Linux and macOS.
// Returns a map of resource type to rlimit struct.
func createRlimitsCommon(limits *ResourceLimits) map[int]syscall.Rlimit {
	rlimits := make(map[int]syscall.Rlimit)

	if limits == nil {
		return rlimits
	}

	// Memory limit (RLIMIT_AS - address space) - works on both Linux and macOS
	if limits.MaxMemoryBytes > 0 {
		rlimits[syscall.RLIMIT_AS] = syscall.Rlimit{
			Cur: uint64(limits.MaxMemoryBytes),
			Max: uint64(limits.MaxMemoryBytes),
		}
	}

	// CPU time limit (RLIMIT_CPU - in seconds) - works on both Linux and macOS
	if limits.MaxCPUTimeMs > 0 {
		cpuSeconds := uint64(limits.MaxCPUTimeMs / 1000)
		if cpuSeconds == 0 {
			cpuSeconds = 1 // Minimum 1 second
		}
		rlimits[syscall.RLIMIT_CPU] = syscall.Rlimit{
			Cur: cpuSeconds,
			Max: cpuSeconds,
		}
	}

	// File size limit (RLIMIT_FSIZE) - works on both Linux and macOS
	if limits.MaxFileSize > 0 {
		rlimits[syscall.RLIMIT_FSIZE] = syscall.Rlimit{
			Cur: uint64(limits.MaxFileSize),
			Max: uint64(limits.MaxFileSize),
		}
	}

	return rlimits
}

// BindMount represents a bind mount configuration.
type BindMount struct {
	Source   string
	Target   string
	ReadOnly bool
}
