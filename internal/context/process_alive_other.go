//go:build !windows

package context

import (
	"errors"
	"syscall"
)

// processAlive reports whether pid names a running process on this host. A
// process that exists but belongs to another user answers EPERM, and is alive.
// A reused pid reads as alive too: that errs toward keeping an archive a while
// longer, never toward deleting a live one.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || errors.Is(err, syscall.EPERM)
}
