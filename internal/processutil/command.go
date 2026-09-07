// Package processutil contains shared subprocess safety helpers.
package processutil

import (
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

// Cancellable configures a CommandContext for bounded, noninteractive waits
// and process-tree cancellation. Call it before Start or Run.
func Cancellable(cmd *exec.Cmd) *exec.Cmd {
	NonInteractive(cmd)
	if cmd != nil {
		cmd.WaitDelay = 5 * time.Second
		configureTreeKill(cmd)
	}
	return cmd
}
