package campaign

import (
	"context"
	"testing"
	"time"
)

// External audit F4: a write taken at the moment it happens takes the path's
// lease without waiting, and is refused -- naming the holder -- when another
// task holds the path or a directory above it.
func TestWriteSetLockManager_TryAcquireNeverWaitsAndSeesDirectories(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())
	dirLease, err := manager.acquire(context.Background(), "task_dir", []string{"internal/foo"}, time.Millisecond)
	if err != nil || dirLease == nil {
		t.Fatalf("acquire(internal/foo) = %v, %v", dirLease, err)
	}
	fileLease, err := manager.acquire(context.Background(), "task_file", []string{"cmd/app/main.go"}, time.Millisecond)
	if err != nil || fileLease == nil {
		t.Fatalf("acquire(cmd/app/main.go) = %v, %v", fileLease, err)
	}

	for _, tc := range []struct {
		name, taskID, path, wantHolder string
	}{
		{"a file another task holds", "task_writer", "cmd/app/main.go", "task_file"},
		{"a file under a directory another task holds", "task_writer", "internal/foo/bar/baz.go", "task_dir"},
		{"a file under the task's own directory", "task_dir", "internal/foo/bar/baz.go", ""},
		{"a file nobody holds", "task_writer", "internal/other/x.go", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			done := make(chan struct{})
			var (
				lease  *writeSetLockLease
				holder string
				terr   error
			)
			go func() {
				defer close(done)
				lease, holder, terr = manager.tryAcquire(tc.taskID, []string{tc.path})
			}()
			select {
			case <-done:
			case <-time.After(5 * time.Second):
				t.Fatal("tryAcquire waited")
			}
			if terr != nil {
				t.Fatalf("tryAcquire: %v", terr)
			}
			if holder != tc.wantHolder {
				t.Fatalf("held by %q, want %q", holder, tc.wantHolder)
			}
			if (lease != nil) != (tc.wantHolder == "") {
				t.Fatalf("lease = %v with holder %q", lease, holder)
			}
			if lease != nil {
				lease.release()
			}
		})
	}

	// Released, the directory no longer covers the file.
	dirLease.release()
	lease, holder, err := manager.tryAcquire("task_writer", []string{"internal/foo/bar/baz.go"})
	if err != nil || holder != "" || lease == nil {
		t.Fatalf("after release: lease=%v holder=%q err=%v", lease, holder, err)
	}
	lease.release()
	fileLease.release()
}
