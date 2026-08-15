//go:build windows

package atomicfile

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32     = syscall.NewLazyDLL("kernel32.dll")
	procReplaceFileW = modkernel32.NewProc("ReplaceFileW")
)

// replaceExisting atomically moves src onto dst.
//
// os.Rename maps to MoveFileEx, which refuses to supersede a destination that
// any process holds open, so every atomic write failed while a reader was
// active. ReplaceFileW is the API built for that case. It requires the
// destination to exist, so a first-time write still goes through os.Rename.
func replaceExisting(src, dst string) error {
	if _, err := os.Stat(dst); err != nil {
		return os.Rename(src, dst)
	}
	d, err := syscall.UTF16PtrFromString(dst)
	if err != nil {
		return err
	}
	s, err := syscall.UTF16PtrFromString(src)
	if err != nil {
		return err
	}
	r, _, callErr := procReplaceFileW.Call(
		uintptr(unsafe.Pointer(d)),
		uintptr(unsafe.Pointer(s)),
		0, 0, 0, 0,
	)
	if r == 0 {
		return callErr
	}
	return nil
}
