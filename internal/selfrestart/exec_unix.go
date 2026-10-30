//go:build !windows

package selfrestart

import (
	"os"
	"syscall"
)

// platformExec replaces the process image in place: same PID, same terminal.
func platformExec(argv0 string, argv []string, envv []string) error {
	return syscall.Exec(argv0, argv, envv)
}

// inodeOf is the file's inode: a rename over the executable changes it even
// when size and time do not.
func inodeOf(st os.FileInfo) uint64 {
	if sys, ok := st.Sys().(*syscall.Stat_t); ok {
		return uint64(sys.Ino)
	}
	return 0
}

// ExecSupported reports whether this platform restarts in place.
const ExecSupported = true
