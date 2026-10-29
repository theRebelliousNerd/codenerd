//go:build !windows

package filelock

import (
	"os"
	"syscall"
)

// Lock is an advisory lock held ACROSS processes for one file's
// shared file (a store's JSON). sync.Mutex only serialises goroutines inside one process while
// two nerd processes on the same workspace need this.
type Lock struct {
	f *os.File
}

// Acquire blocks until the lock is held.
func Acquire(path string) (*Lock, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o644)
	if err != nil {
		return nil, err
	}
	if err := syscall.Flock(int(f.Fd()), syscall.LOCK_EX); err != nil {
		f.Close()
		return nil, err
	}
	return &Lock{f: f}, nil
}

func (l *Lock) Release() error {
	syscall.Flock(int(l.f.Fd()), syscall.LOCK_UN)
	return l.f.Close()
}
