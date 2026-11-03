package filelock

import (
	"errors"
	"github.com/google/go-cmp/cmp"
	"os"
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

func TestAcquire_PreservesLockFileContents(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name     string
		exists   bool
		contents []byte
	}{
		{name: "creates missing file"},
		{name: "existing empty file", exists: true, contents: []byte{}},
		{name: "existing text file", exists: true, contents: []byte("owner metadata\n")},
		{name: "existing binary file", exists: true, contents: []byte{0, 255, 1, 128, 0}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			path := filepath.Join(t.TempDir(), "contents.lock")
			if tt.exists {
				if err := os.WriteFile(path, tt.contents, 0o600); err != nil {
					t.Fatalf("seed lock file: %v", err)
				}
			}
			lock, err := Acquire(path)
			if err != nil {
				t.Fatalf("Acquire: %v", err)
			}
			if lock == nil {
				t.Fatal("Acquire returned nil lock without error")
			}
			released := false
			t.Cleanup(func() {
				if !released {
					if err := lock.Release(); err != nil {
						t.Errorf("cleanup Release: %v", err)
					}
				}
			})
			info, err := os.Stat(path)
			if err != nil {
				t.Fatalf("stat acquired lock file: %v", err)
			}
			if !info.Mode().IsRegular() || info.Size() != int64(len(tt.contents)) {
				t.Errorf("acquired file: mode=%v size=%d; want regular file of size %d", info.Mode(), info.Size(), len(tt.contents))
			}
			err = lock.Release()
			released = true
			if err != nil {
				t.Fatalf("Release: %v", err)
			}
			// Read after unlocking: Windows locks exclude reads of the locked byte.
			got, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read released lock file: %v", err)
			}
			if diff := cmp.Diff(string(tt.contents), string(got)); diff != "" {
				t.Errorf("lock file contents mismatch (-want +got):\n%s", diff)
			}
		})
	}
}

func TestAcquire_InvalidPathsReturnPathErrors(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	tests := []struct {
		name    string
		path    string
		wantErr error
	}{
		{name: "empty path", path: "", wantErr: os.ErrNotExist},
		{name: "missing parent", path: filepath.Join(dir, "missing", "x.lock"), wantErr: os.ErrNotExist},
		{name: "directory", path: dir},
		{name: "embedded NUL", path: filepath.Join(dir, "bad\x00.lock")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			lock, err := Acquire(tt.path)
			if lock != nil {
				if releaseErr := lock.Release(); releaseErr != nil {
					t.Errorf("release unexpected lock: %v", releaseErr)
				}
				t.Error("Acquire returned a lock for an invalid path")
			}
			if err == nil {
				t.Fatal("Acquire returned no error for an invalid path")
			}
			var pathErr *os.PathError
			if !errors.As(err, &pathErr) {
				t.Fatalf("Acquire error = %T; want *os.PathError: %v", err, err)
			}
			if pathErr.Path != tt.path || pathErr.Op != "open" {
				t.Errorf("path error: path=%q op=%q; want path=%q op=open", pathErr.Path, pathErr.Op, tt.path)
			}
			if tt.wantErr != nil && !errors.Is(err, tt.wantErr) {
				t.Errorf("Acquire error = %v; want errors.Is(_, %v)", err, tt.wantErr)
			}
		})
	}
}
