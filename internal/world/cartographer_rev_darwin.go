//go:build darwin

package world

import (
	"os"
	"syscall"
)

// fileContentGen is the inode change time. Darwin names the field
// Ctimespec; linux names it Ctim. The trust rule is the same.
func fileContentGen(_ string, info os.FileInfo) (int64, bool, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, false, true
	}
	return st.Ctimespec.Nano(), true, true
}
