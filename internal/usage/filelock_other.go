//go:build !windows

package usage

import (
	"os"
	"syscall"
)

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
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &fileLock{f: f}, nil
}

func (l *fileLock) release() error {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	return l.f.Close()
}
