//go:build !windows

package atomicfile

import "os"

// replaceExisting atomically moves src onto dst. On POSIX a rename already
// replaces an open destination, so this is os.Rename.
func replaceExisting(src, dst string) error {
	return os.Rename(src, dst)
}
