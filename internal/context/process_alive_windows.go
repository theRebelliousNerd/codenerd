//go:build windows

package context

import (
	"errors"

	"golang.org/x/sys/windows"
)

// stillActive is GetExitCodeProcess's answer for a process that has not exited.
const stillActive = 259

// processAlive reports whether pid names a running process on this host. A
// process this user may not open (access denied) exists, and is alive. A
// reused pid reads as alive too: that errs toward keeping an archive a while
// longer, never toward deleting a live one.
func processAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, uint32(pid))
	if err != nil {
		return errors.Is(err, windows.ERROR_ACCESS_DENIED)
	}
	defer windows.CloseHandle(h)
	var code uint32
	if err := windows.GetExitCodeProcess(h, &code); err != nil {
		return true
	}
	return code == stillActive
}
