//go:build windows

package selfrestart

import "os"

// platformExec cannot replace a running process on Windows; the session
// reports the rebuild and asks the user to restart instead.
func platformExec(string, []string, []string) error {
	return ErrExecUnsupported
}

func inodeOf(os.FileInfo) uint64 { return 0 }

// ExecSupported reports whether this platform restarts in place.
const ExecSupported = false
