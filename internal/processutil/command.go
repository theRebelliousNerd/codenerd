// Package processutil contains shared subprocess safety helpers.
package processutil

import (
	"bytes"
	"errors"
	"os/exec"
	"strings"
	"time"
)

// NonInteractive gives cmd a finite empty stdin unless the caller already
// supplied one. Besides making the subprocess contract explicit, this avoids
// os/exec opening the platform null device for a nil stdin. On Windows that
// open can block in GetConsoleMode under a heavily concurrent Go test run.
func NonInteractive(cmd *exec.Cmd) *exec.Cmd {
	if cmd != nil && cmd.Stdin == nil {
		cmd.Stdin = strings.NewReader("")
	}
	return cmd
}

// PipeWaitDelay bounds how long Run waits for output pipes after the
// process exits or is killed; a grandchild holding a pipe cannot keep a
// cancelled command's Wait blocked past it.
const PipeWaitDelay = 5 * time.Second

// Run runs cmd noninteractively with a bounded wait and whole-tree
// cancellation: when cmd's context ends, every process the command
// spawned is killed, not just the direct child, and Wait returns at most
// WaitDelay after that even if something still holds a pipe. Use it
// instead of cmd.Run for any CommandContext command.
func Run(cmd *exec.Cmd) error {
	NonInteractive(cmd)
	cmd.WaitDelay = PipeWaitDelay
	release, err := startKillScope(cmd)
	if err != nil {
		return err
	}
	defer release()
	return cmd.Wait()
}

// CombinedOutput is Run with stdout and stderr captured together, like
// exec.Cmd.CombinedOutput (it returns an error if Stdout or Stderr is already set).
func CombinedOutput(cmd *exec.Cmd) ([]byte, error) {
	if cmd.Stdout != nil {
		return nil, errors.New("exec: Stdout already set")
	}
	if cmd.Stderr != nil {
		return nil, errors.New("exec: Stderr already set")
	}
	var b bytes.Buffer
	cmd.Stdout = &b
	cmd.Stderr = &b
	err := Run(cmd)
	return b.Bytes(), err
}
