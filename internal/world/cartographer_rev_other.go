//go:build !windows && !linux && !darwin

package world

import "os"

// fileContentGen has no portable generation on this OS. genOK false makes
// loadSymbols hash every sibling instead of trusting size and mtime.
func fileContentGen(string, os.FileInfo) (int64, bool, bool) {
	return 0, false, true
}
