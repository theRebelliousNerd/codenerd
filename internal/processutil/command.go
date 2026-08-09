// Package processutil contains shared subprocess safety helpers.
package processutil

import (
	"os/exec"
	"strings"
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
