package filelock

import (
	"path/filepath"
	"testing"
	"time"
)

func TestAcquire_WhenHeld_ShouldExcludeASecondHolder(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "x.lock")

	l1, err := Acquire(lockPath)
	if err != nil {
		t.Fatalf("first Acquire failed: %v", err)
	}
	if l1 == nil {
		t.Fatalf("first Acquire returned nil lock without error")
	}
	released := false
	defer func() {
		if !released {
			_ = l1.Release()
		}
	}()

	type result struct {
		l   *Lock
		err error
	}
	ch := make(chan result, 1)
	go func() {
		l2, err := Acquire(lockPath)
		ch <- result{l2, err}
	}()

	select {
	case res := <-ch:
		if res.l != nil {
			_ = res.l.Release()
		}
		t.Fatalf("second acquire completed while first held (err=%v); expected exclusion", res.err)
	case <-time.After(250 * time.Millisecond):
		// expected: second holder blocked while first holds the lock
	}

	if err := l1.Release(); err != nil {
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
		if err := res.l.Release(); err != nil {
			t.Fatalf("release of second lock failed: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatalf("second acquire did not complete within 5s after first release")
	}
}

func TestAcquire_ShouldBeReacquirableAfterRelease(t *testing.T) {
	dir := t.TempDir()
	lockPath := filepath.Join(dir, "x.lock")

	l1, err := Acquire(lockPath)
	if err != nil {
		t.Fatalf("first acquire failed: %v", err)
	}
	if l1 == nil {
		t.Fatalf("first acquire returned nil lock without error")
	}
	if err := l1.Release(); err != nil {
		t.Fatalf("first release failed: %v", err)
	}

	l2, err := Acquire(lockPath)
	if err != nil {
		t.Fatalf("second acquire after release failed: %v", err)
	}
	if l2 == nil {
		t.Fatalf("second acquire returned nil lock without error")
	}
	if err := l2.Release(); err != nil {
		t.Fatalf("second release failed: %v", err)
	}
}

func TestAcquire_WhenPathIsUnwritable_ShouldReturnError(t *testing.T) {
	// Directory does not exist, so OpenFile should fail.
	lockPath := filepath.Join(t.TempDir(), "no-such-dir", "x.lock")

	l, err := Acquire(lockPath)
	if err == nil {
		if l != nil {
			_ = l.Release()
		}
		t.Fatalf("expected error for unwritable path %q, got nil", lockPath)
	}
	if l != nil {
		_ = l.Release()
		t.Fatalf("expected nil lock on error, got non-nil lock for path %q", lockPath)
	}
}
