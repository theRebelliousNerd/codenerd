package campaign

import (
	"context"
	"errors"
	"testing"
	"time"
)

// The direction the report was about (ladder run R1-11, audit L3): a task
// that declares a file under a directory another task holds waits for the
// directory, and gets its lease -- and its write -- once the directory is
// released. Until then the file's lease was granted at once, both tasks ran,
// and the second one's first write was refused as held by the first.
func TestWriteSetLockManager_AFileUnderAHeldDirectoryWaitsForIt(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())
	dir, err := manager.acquire(context.Background(), "/task_dir", []string{"internal/foo"}, time.Millisecond)
	if err != nil || dir == nil {
		t.Fatalf("acquire(directory) = %v, %v", dir, err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	if lease, err := manager.acquire(ctx, "/task_file", []string{"internal/foo/x.go"}, 5*time.Millisecond); lease != nil || !errors.Is(err, ErrWriteSetLockTimeout) {
		if lease != nil {
			lease.release()
		}
		t.Fatalf("acquire(file under the held directory) granted=%v err=%v; want it to wait for the directory", lease != nil, err)
	}

	granted := make(chan *writeSetLockLease, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		lease, _ := manager.acquire(ctx, "/task_file", []string{"internal/foo/x.go"}, 5*time.Millisecond)
		granted <- lease
	}()
	dir.release()
	lease := <-granted
	if lease == nil {
		t.Fatal("the file's lease was not granted once the directory was released")
	}
	defer lease.release()
	write, heldBy, err := manager.tryAcquire("/task_file", []string{"internal/foo/x.go"})
	if err != nil || write == nil {
		t.Fatalf("the task's write under its own lease was refused: heldBy=%q err=%v", heldBy, err)
	}
	write.release()
}

// A path that only shares the directory's name as a prefix is not under it:
// both tasks run side by side.
func TestWriteSetLockManager_APrefixSiblingOfAHeldDirectoryDoesNotWait(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())
	dir, err := manager.acquire(context.Background(), "/task_dir", []string{"internal/foo"}, time.Millisecond)
	if err != nil || dir == nil {
		t.Fatalf("acquire(directory) = %v, %v", dir, err)
	}
	defer dir.release()

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	lease, err := manager.acquire(ctx, "/task_sibling", []string{"internal/foobar/x.go"}, 5*time.Millisecond)
	if err != nil || lease == nil {
		t.Fatalf("acquire(internal/foobar/x.go) beside a held internal/foo = %v, %v; want a lease at once", lease, err)
	}
	lease.release()
}
