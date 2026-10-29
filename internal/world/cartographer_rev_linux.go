//go:build linux

package world

import (
	"os"
	"syscall"
)

// fileContentGen is the inode change time. A content write and os.Chtimes
// both move it; a same-tick rewrite that does not is rejected by
// symbolStampQuantum.
func fileContentGen(_ string, info os.FileInfo) (int64, bool, bool) {
	st, ok := info.Sys().(*syscall.Stat_t)
	if !ok || st == nil {
		return 0, false, true
	}
	return st.Ctim.Nano(), true, true
}
