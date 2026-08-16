package usage

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAcquireFileLock_WhenHeld_ShouldExcludeASecondHolder(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "x.lock")

	l1, err := acquireFileLock(lockPath)
	if err != nil {
		t.Fatalf("first acquireFileLock failed: %v", err)
	}
	if l1 == nil {
		t.Fatalf("first acquireFileLock returned nil lock without error")
	}
	released := false
	defer func() {
		if !released {
			_ = l1.release()
		}
	}()

	type result struct {
		l   *fileLock
		err error
	}
	ch := make(chan result, 1)
	go func() {
		l2, err := acquireFileLock(lockPath)
		ch <- result{l2, err}
	}()

	select {
	case res := <-ch:
		if res.l != nil {
			_ = res.l.release()
		}
		t.Fatalf("second acquire completed while first held (err=%v); expected exclusion", res.err)
	case <-time.After(250 * time.Millisecond):
		// expected: second holder blocked while first holds the lock
	}

	if err := l1.release(); err != nil {
		t.Fatalf("release of first lock failed: %v", err)
	}
	released = true

	select {
	case res := <-ch:
		if res.err != nil {
			t.Fatalf("second acquire after release failed: %v", res.err)
		}
		if res.l == nil {
			t.Fatalf("second acquire after release returned nil lock without error")
		}
		if err := res.l.release(); err != nil {
			t.Fatalf("release of second lock failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("second acquire did not complete within 5s after first release")
	}
}

func TestAcquireFileLock_ShouldBeReacquirableAfterRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "x.lock")

	l1, err := acquireFileLock(lockPath)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if l1 == nil {
		t.Fatalf("first acquire returned nil lock without error")
	}
	if err := l1.release(); err != nil {
		t.Fatalf("first release failed: %v", err)
	}

	l2, err := acquireFileLock(lockPath)
	if err != nil {
		t.Fatalf("second acquire after release failed: %v", err)
	}
	if l2 == nil {
		t.Fatalf("second acquire returned nil lock without error")
	}
	if err := l2.release(); err != nil {
		t.Fatalf("second release failed: %v", err)
	}
}

func TestAcquireFileLock_WhenPathIsUnwritable_ShouldReturnError(t *testing.T) {
	// Directory does not exist, so OpenFile should fail.
	lockPath := filepath.Join(t.TempDir(), "no-such-dir", "x.lock")

	l, err := acquireFileLock(lockPath)
	if err == nil {
		if l != nil {
			_ = l.release()
		}
		t.Fatalf("expected error for unwritable path %q, got nil", lockPath)
	}
	if l != nil {
		_ = l.release()
		t.Fatalf("expected nil lock on error, got non-nil lock for path %q", lockPath)
	}
}
