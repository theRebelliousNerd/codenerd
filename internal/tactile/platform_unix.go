//go:build !windows

package tactile

import (
	"os/exec"
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

// applyPlatformAttrs is a no-op on Unix; Command.CommandLine only affects Windows.
func applyPlatformAttrs(execCmd *exec.Cmd, cmd Command) {}

// BindMount represents a bind mount configuration.
type BindMount struct {
	Source   string
	Target   string
	ReadOnly bool
}
