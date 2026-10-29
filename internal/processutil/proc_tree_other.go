//go:build !windows

package processutil

import (
	"os/exec"
	"syscall"
)

// startKillScope starts cmd in its own process group, so a cancel can
// signal the group.
//
// The child is started in its own process group so a timeout can signal the
// group: bash dies together with the pipeline stages it spawned, instead of
// leaving a grep or a test binary orphaned and holding the output pipe.
func startKillScope(cmd *exec.Cmd) (release func(), err error) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		// Negative pid addresses the process group created by Setpgid.
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		return cmd.Process.Kill()
	}
	if err := cmd.Start(); err != nil {
		return func() {}, err
	}
	return func() {}, nil
}
