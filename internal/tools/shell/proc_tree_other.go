//go:build !windows

package shell

import (
	"os/exec"
	"syscall"
)

// configureTreeKill makes context cancellation kill the whole process tree.
//
// The child is started in its own process group so a timeout can signal the
// group: bash dies together with the pipeline stages it spawned, instead of
// leaving a grep or a test binary orphaned and holding the output pipe.
func configureTreeKill(cmd *exec.Cmd) {
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
}
