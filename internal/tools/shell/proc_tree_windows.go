//go:build windows

package shell

import (
	"os/exec"
	"strconv"
)

// configureTreeKill makes context cancellation kill the whole process tree.
//
// exec.Cmd's default cancel kills only the direct child. When run_command
// times out on a pipeline such as `bash -c "grep -r x . | head"`, bash dies
// and grep is orphaned, holding the pipe (bounded by WaitDelay) and burning
// CPU for as long as the machine stays up. Seventeen such greps, some three
// days old, were found on 2026-09-04. taskkill /T walks the tree.
func configureTreeKill(cmd *exec.Cmd) {
	cmd.Cancel = func() error {
		if cmd.Process == nil {
			return nil
		}
		pid := strconv.Itoa(cmd.Process.Pid)
		// Best effort: taskkill may be missing or the tree already gone. The
		// direct kill below still applies either way.
		_ = exec.Command("taskkill", "/T", "/F", "/PID", pid).Run()
		return cmd.Process.Kill()
	}
}
