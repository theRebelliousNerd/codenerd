//go:build windows

package usage

import (
	"os"
	"syscall"
	"unsafe"
)

var (
	modkernel32      = syscall.NewLazyDLL("kernel32.dll")
	procLockFileEx   = modkernel32.NewProc("LockFileEx")
	procUnlockFileEx = modkernel32.NewProc("UnlockFileEx")
)

const lockfileExclusiveLock = 0x2

// fileLock is an advisory lock held ACROSS processes for one workspace's
// usage.json. sync.Mutex only serialises goroutines inside one process while
// two nerd processes on the same workspace need this.
type fileLock struct {
	f *os.File
}

// acquireFileLock blocks until the lock is held.
func acquireFileLock(path string) (*fileLock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	var ol syscall.Overlapped
	r1, _, err := procLockFileEx.Call(f.Fd(), uintptr(lockfileExclusiveLock), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	if r1 == 0 {
		f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
}

func (l *fileLock) release() error {
	var ol syscall.Overlapped
	procUnlockFileEx.Call(l.f.Fd(), 0, 1, 0, uintptr(unsafe.Pointer(&ol)))
	return l.f.Close()
}
