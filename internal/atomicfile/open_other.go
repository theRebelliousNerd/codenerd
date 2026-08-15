//go:build !windows

package atomicfile

import "os"

// Open opens path for reading. On POSIX a rename can already replace a file
// that is held open, so this is os.Open.
func Open(path string) (*os.File, error) {
	return os.Open(path)
}
