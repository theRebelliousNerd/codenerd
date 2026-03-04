package campaign

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeWriteSetPaths_SortsAndDedupes(t *testing.T) {
	workspace := t.TempDir()

	normalized := normalizeWriteSetPaths(workspace, []string{
		"pkg/../pkg/file.go",
		"pkg/file.go",
		"./pkg/other.go",
		"pkg\\other.go",
	})

	if len(normalized) != 2 {
		t.Fatalf("expected 2 normalized paths, got %d: %v", len(normalized), normalized)
	}

	first := normalizeAbsolutePath(workspace, "pkg/file.go")
	second := normalizeAbsolutePath(workspace, "pkg/other.go")
	if first > second {
		first, second = second, first
	}
	expected := []string{first, second}

	for i := range expected {
		if normalized[i] != expected[i] {
			t.Fatalf("path[%d] = %q, want %q", i, normalized[i], expected[i])
		}
	}
}

func TestWriteSetLockManager_AcquireTimeout(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())

	lease, err := manager.acquire(context.Background(), "task-1", []string{"internal/a.go"}, time.Millisecond)
	if err != nil {
		t.Fatalf("task-1 acquire failed: %v", err)
	}
	defer lease.release()

	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Millisecond)
	defer cancel()

	_, err = manager.acquire(ctx, "task-2", []string{"internal/a.go"}, 5*time.Millisecond)
	if err == nil {
		t.Fatalf("expected timeout error for task-2")
	}
	if !errors.Is(err, ErrWriteSetLockTimeout) {
		t.Fatalf("expected ErrWriteSetLockTimeout, got %v", err)
	}
}

func TestWriteSetLockManager_NoDeadlockWithOppositeOrdering(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())

	errCh := make(chan error, 2)
	var wg sync.WaitGroup
	wg.Add(2)

	run := func(taskID string, writeSet []string, hold time.Duration) {
		defer wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), time.Second)
		defer cancel()

		lease, err := manager.acquire(ctx, taskID, writeSet, 2*time.Millisecond)
		if err != nil {
			errCh <- fmt.Errorf("%s acquire failed: %w", taskID, err)
			return
		}
		time.Sleep(hold)
		lease.release()
	}

	go run("task-1", []string{"b.go", "a.go"}, 30*time.Millisecond)
	go run("task-2", []string{"a.go", "b.go"}, 10*time.Millisecond)

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestWriteSetLockManager_ConcurrentMutualExclusion(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())

	var active int32
	var maxActive int32
	errCh := make(chan error, 32)

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()

			ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
			defer cancel()

			lease, err := manager.acquire(ctx, fmt.Sprintf("task-%d", id), []string{"internal/shared.go"}, 2*time.Millisecond)
			if err != nil {
				errCh <- err
				return
			}
			defer lease.release()

			cur := atomic.AddInt32(&active, 1)
			for {
				prev := atomic.LoadInt32(&maxActive)
				if cur <= prev {
					break
				}
				if atomic.CompareAndSwapInt32(&maxActive, prev, cur) {
					break
				}
			}

			time.Sleep(5 * time.Millisecond)
			atomic.AddInt32(&active, -1)
		}(i)
	}

	wg.Wait()
	close(errCh)

	for err := range errCh {
		if err != nil {
			t.Fatalf("concurrent acquire failed: %v", err)
		}
	}
	if maxActive != 1 {
		t.Fatalf("expected maxActive=1 for shared write_set, got %d", maxActive)
	}
}
