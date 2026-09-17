//go:build windows

package processutil

import (
	"context"
	"os/exec"
	"strconv"
	"sync/atomic"
	"time"

	"golang.org/x/sys/windows"
)

// startKillScope starts cmd in a Windows job object so context cancellation
// kills the whole process tree.
//
// taskkill /T /F /PID walks the parent chain, which misses processes Git-bash
// (MSYS) spawns: measured 2026-09-17, exec.CommandContext(ctx, "bash", "-c",
// `sh -c "sleep 7"`) with a 2 s deadline and Cancel = taskkill /F /T /PID
// reported success for ONE process, Run returned only after 7.2 s (the
// orphaned sleep.exe held the output pipe), and sleep.exe kept running. The
// same command with the bash process assigned to a job object right after
// Start and Cancel = TerminateJobObject returned in 2.01 s with the sleep
// killed. A job contains every process started after the parent joins it,
// whatever their parent chain looks like.
//
// The job has to be assigned after Start, which a Cancel hook set before Run
// cannot do, so this helper owns Start and returns a release that closes the
// job handle. Do NOT set JOB_OBJECT_LIMIT_KILL_ON_JOB_CLOSE: a command that
// finished normally must not have its background children killed by release.
//
// A process the child spawns in the microseconds between Start and the
// assignment is not in the job — the taskkill fallback below does not cover
// that either, so it is documented, not handled. If CreateJobObject,
// OpenProcess or AssignProcessToJobObject fails, Cancel falls back to the old
// taskkill /T /F tree kill (5 s bounded, NonInteractive) followed by
// Process.Kill.
func startKillScope(cmd *exec.Cmd) (release func(), err error) {
	job, jobErr := windows.CreateJobObject(nil, nil)
	var fallback atomic.Bool
	fallback.Store(jobErr != nil)
	cmd.Cancel = func() error {
		if !fallback.Load() {
			_ = windows.TerminateJobObject(job, 1)
		} else if cmd.Process != nil {
			pid := strconv.Itoa(cmd.Process.Pid)
			killCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = NonInteractive(exec.CommandContext(killCtx, "taskkill", "/T", "/F", "/PID", pid)).Run()
		}
		if cmd.Process == nil {
			return nil
		}
		return cmd.Process.Kill()
	}
	if err := cmd.Start(); err != nil {
		if jobErr == nil {
			_ = windows.CloseHandle(job)
		}
		return func() {}, err
	}
	if jobErr == nil {
		h, err := windows.OpenProcess(windows.PROCESS_SET_QUOTA|windows.PROCESS_TERMINATE, false, uint32(cmd.Process.Pid))
		if err != nil {
			fallback.Store(true)
		} else {
			if err := windows.AssignProcessToJobObject(job, h); err != nil {
				fallback.Store(true)
			}
			_ = windows.CloseHandle(h)
		}
	}
	release = func() {
		if jobErr == nil {
			_ = windows.CloseHandle(job)
		}
	}
	return release, nil
}
