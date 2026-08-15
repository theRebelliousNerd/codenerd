//go:build windows

package atomicfile

import (
	"os"
	"syscall"
)

// Open opens path for reading in a way that does not block a concurrent
// atomic replace.
//
// Windows refuses to replace a file that another handle holds open unless
// that handle allowed FILE_SHARE_DELETE. os.Open does not pass it, so a
// reader using os.Open makes every WriteFile to that path fail for as long
// as the reader lives. Readers of atomically-written files should use this.
func Open(path string) (*os.File, error) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(
		p,
		syscall.GENERIC_READ,
		syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE|syscall.FILE_SHARE_DELETE,
		nil,
		syscall.OPEN_EXISTING,
		syscall.FILE_ATTRIBUTE_NORMAL,
		0,
	)
	if err != nil {
		return nil, &os.PathError{Op: "open", Path: path, Err: err}
	}
	return os.NewFile(uintptr(h), path), nil
}
