//go:build windows

package atomicfile

import (
	"errors"
	"os"
	"syscall"
	"time"
	"unsafe"
)

var (
	modkernel32          = syscall.NewLazyDLL("kernel32.dll")
	procReplaceFileW     = modkernel32.NewProc("ReplaceFileW")
	procGetFileAttrsW    = modkernel32.NewProc("GetFileAttributesW")
	procSetFileAttrsW    = modkernel32.NewProc("SetFileAttributesW")
	invalidFileAttrs     = uint32(0xFFFFFFFF)
	fileAttributeRdOnly  = uint32(0x00000001)
	errorAccessDenied    = syscall.Errno(5)
	errorSharingViolate  = syscall.Errno(32)
	errorLockViolation   = syscall.Errno(33)
	errorUserMappedFile  = syscall.Errno(1224)
	replaceRetryAttempts = 10
)

// replaceExisting atomically moves src onto dst.
//
// os.Rename maps to MoveFileEx, which refuses to supersede a destination that
// any process holds open, so every atomic write failed while a reader was
// active. ReplaceFileW is the API built for that case. It requires the
// destination to exist, so a first-time write still goes through os.Rename.
//
// Two things ReplaceFileW does NOT do on its own, both of which showed up as
// whole packages failing on the Windows runner while Linux stayed green:
//
//  1. It refuses a destination carrying the read-only ATTRIBUTE, with
//     ERROR_ACCESS_DENIED. That attribute is what os.Chmod sets for a mode
//     without the write bit, so a file written 0400 could never be replaced —
//     even though replacing by rename is supposed to need permission on the
//     directory rather than on the file, which is the property this whole
//     package is documented as having. Clearing the attribute and retrying
//     makes Windows match that promise; ReplaceFileW then takes its
//     attributes from the (now writable) destination, so the replacement does
//     not inherit read-only either, and the mode the caller asked for stands.
//
//  2. It is not immune to contention. Concurrent replaces of one path, or a
//     reader that opened without FILE_SHARE_DELETE closing a moment later,
//     produce ERROR_SHARING_VIOLATION and ERROR_LOCK_VIOLATION. Those are
//     transient by nature, so they are retried on a short backoff rather than
//     surfaced as a failed write. The retry is bounded: a handle that is held
//     open indefinitely is a real error and must be reported as one, not
//     waited on forever.
func replaceExisting(src, dst string) error {
	if _, err := os.Stat(dst); err != nil {
		return os.Rename(src, dst)
	}

	clearedReadOnly := false
	replaced := false
	backoff := time.Millisecond
	var lastErr error

	for attempt := 0; attempt < replaceRetryAttempts; attempt++ {
		err := replaceFileOnce(src, dst)
		if err == nil {
			replaced = true
			return nil
		}
		lastErr = err

		switch {
		case errors.Is(err, errorSharingViolate),
			errors.Is(err, errorLockViolation),
			errors.Is(err, errorUserMappedFile):
			time.Sleep(backoff)
			if backoff < 50*time.Millisecond {
				backoff *= 2
			}
		case errors.Is(err, errorAccessDenied) && !clearedReadOnly:
			// Clear it at most once. Retrying a genuine permission failure in a
			// loop would turn a clear error into a slow one.
			clearedReadOnly = true
			if !clearReadOnlyAttribute(dst) {
				return err
			}
			// Restored if we still cannot replace: a failed write must not
			// leave the caller's file more writable than it found it.
			defer func() {
				if !replaced {
					setReadOnlyAttribute(dst)
				}
			}()
		default:
			return err
		}
	}
	return lastErr
}

func replaceFileOnce(src, dst string) error {
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

// clearReadOnlyAttribute removes FILE_ATTRIBUTE_READONLY from path, reporting
// whether anything changed. A path that was not read-only reports false, so the
// caller does not retry a replace that failed for some other reason.
func clearReadOnlyAttribute(path string) bool {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return false
	}
	attrs, _, _ := procGetFileAttrsW.Call(uintptr(unsafe.Pointer(p)))
	if uint32(attrs) == invalidFileAttrs || uint32(attrs)&fileAttributeRdOnly == 0 {
		return false
	}
	r, _, _ := procSetFileAttrsW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(uint32(attrs)&^fileAttributeRdOnly),
	)
	return r != 0
}

// setReadOnlyAttribute puts FILE_ATTRIBUTE_READONLY back on path. Best effort:
// it runs only on a path that has already failed, and reporting a second error
// there would bury the first.
func setReadOnlyAttribute(path string) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return
	}
	attrs, _, _ := procGetFileAttrsW.Call(uintptr(unsafe.Pointer(p)))
	if uint32(attrs) == invalidFileAttrs {
		return
	}
	_, _, _ = procSetFileAttrsW.Call(
		uintptr(unsafe.Pointer(p)),
		uintptr(uint32(attrs)|fileAttributeRdOnly),
	)
}
