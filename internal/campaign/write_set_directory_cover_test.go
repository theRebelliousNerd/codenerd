package campaign

import (
	"context"
	"errors"
	"testing"
	"time"
)

// Directory-covers-file direction: a held file blocks a directory above it.
// Exercises claim's descendant check (prefix match + holder mismatch).
func TestWriteSetLockManager_DirectoryBlockedByHeldFile(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())

	fileLease, err := manager.acquire(context.Background(), "holder-a", []string{"internal/foo/x.go"}, time.Millisecond)
	if err != nil {
		t.Fatalf("acquire(file) failed: %v", err)
	}
	if fileLease == nil {
		t.Fatal("acquire(file) returned nil lease")
	}
	defer fileLease.release()

	lease, heldBy, err := manager.tryAcquire("holder-b", []string{"internal/foo"})
	if err != nil {
		t.Fatalf("tryAcquire(dir) failed: %v", err)
	}
	if lease != nil {
		lease.release()
		t.Fatal("tryAcquire(dir) granted a lease while a file under it is held")
	}
	if heldBy != "holder-a" {
		t.Fatalf("tryAcquire(dir) heldBy = %q, want %q", heldBy, "holder-a")
	}
}

// Error branch of the same descendant conflict: a blocking acquire waits and
// then reports ErrWriteSetLockTimeout instead of granting.
func TestWriteSetLockManager_DirectoryAcquireTimesOutWhileFileHeld(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())

	fileLease, err := manager.acquire(context.Background(), "holder-a", []string{"internal/foo/x.go"}, time.Millisecond)
	if err != nil {
		t.Fatalf("acquire(file) failed: %v", err)
	}
	if fileLease == nil {
		t.Fatal("acquire(file) returned nil lease")
	}
	defer fileLease.release()

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	lease, err := manager.acquire(ctx, "holder-b", []string{"internal/foo"}, 5*time.Millisecond)
	if !errors.Is(err, ErrWriteSetLockTimeout) {
		t.Fatalf("acquire(dir) error = %v, want %v", err, ErrWriteSetLockTimeout)
	}
	if lease != nil {
		lease.release()
		t.Fatal("acquire(dir) returned a lease despite timeout")
	}
}

// Waiting branch: the directory requester blocks while the file is held and
// is granted once the file is released.
func TestWriteSetLockManager_DirectoryWaitsThenProceedsAfterFileRelease(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())

	fileLease, err := manager.acquire(context.Background(), "holder-a", []string{"internal/foo/x.go"}, time.Millisecond)
	if err != nil {
		t.Fatalf("acquire(file) failed: %v", err)
	}
	if fileLease == nil {
		t.Fatal("acquire(file) returned nil lease")
	}

	acquired := make(chan *writeSetLockLease, 1)
	errCh := make(chan error, 1)
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		lease, err := manager.acquire(ctx, "holder-b", []string{"internal/foo"}, 5*time.Millisecond)
		if err != nil {
			errCh <- err
			return
		}
		acquired <- lease
	}()

	select {
	case lease := <-acquired:
		lease.release()
		t.Fatal("directory lease granted while file under it is still held")
	case err := <-errCh:
		t.Fatalf("waiting acquire(dir) failed early: %v", err)
	case <-time.After(50 * time.Millisecond):
		// Still blocked: the expected state before release.
	}

	fileLease.release()

	select {
	case lease := <-acquired:
		if lease == nil {
			t.Fatal("waiting acquire(dir) returned nil lease after release")
		}
		lease.release()
	case err := <-errCh:
		t.Fatalf("waiting acquire(dir) failed after release: %v", err)
	case <-time.After(2 * time.Second):
		t.Fatal("waiting directory was not granted after file release")
	}
}

// Edge branch: the same holder may take a directory above a file it already
// holds. The prefix matches but the holder check lets it through.
func TestWriteSetLockManager_DirectoryAllowedForSameHolderAsFile(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())

	fileLease, err := manager.acquire(context.Background(), "holder-a", []string{"internal/foo/x.go"}, time.Millisecond)
	if err != nil {
		t.Fatalf("acquire(file) failed: %v", err)
	}
	if fileLease == nil {
		t.Fatal("acquire(file) returned nil lease")
	}
	defer fileLease.release()

	lease, heldBy, err := manager.tryAcquire("holder-a", []string{"internal/foo"})
	if err != nil {
		t.Fatalf("tryAcquire(dir, same holder) failed: %v", err)
	}
	if heldBy != "" {
		t.Fatalf("tryAcquire(dir, same holder) heldBy = %q, want empty", heldBy)
	}
	if lease == nil {
		t.Fatal("tryAcquire(dir, same holder) returned nil lease, want a lease")
	}
	defer lease.release()
}

// Edge branch: a sibling prefix must not count as overlap. internal/foo does
// not cover internal/foobar/x.go, so both leases coexist.
func TestWriteSetLockManager_DirectorySiblingPrefixNotBlocked(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())

	fileLease, err := manager.acquire(context.Background(), "holder-a", []string{"internal/foobar/x.go"}, time.Millisecond)
	if err != nil {
		t.Fatalf("acquire(sibling file) failed: %v", err)
	}
	if fileLease == nil {
		t.Fatal("acquire(sibling file) returned nil lease")
	}
	defer fileLease.release()

	lease, heldBy, err := manager.tryAcquire("holder-b", []string{"internal/foo"})
	if err != nil {
		t.Fatalf("tryAcquire(dir) failed: %v", err)
	}
	if heldBy != "" {
		t.Fatalf("tryAcquire(dir) heldBy = %q, want empty (sibling is not under dir)", heldBy)
	}
	if lease == nil {
		t.Fatal("tryAcquire(dir) returned nil lease, want a lease alongside the sibling file")
	}
	defer lease.release()
}
