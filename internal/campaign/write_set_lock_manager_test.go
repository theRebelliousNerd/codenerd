package campaign

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNormalizeWriteSetPaths_SortsAndDedupes(t *testing.T) {
	workspace := t.TempDir()

	pathsToTest := []string{
		"pkg/../pkg/file.go",
		"pkg/file.go",
		"./pkg/other.go",
		"pkg\\other.go",
	}

	normalized := normalizeWriteSetPaths(workspace, pathsToTest)

	// On Windows, the backslash path resolves to the same element as the forward slash path,
	// deduplicating to 2 elements. On Linux, \ is a valid filename character, so "pkg\other.go"
	// is a distinct file, resulting in 3 elements.
	if len(normalized) < 2 || len(normalized) > 3 {
		t.Fatalf("expected 2 or 3 normalized paths depending on OS, got %d: %v", len(normalized), normalized)
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

func TestNormalizeWriteSetPaths_RejectsOutsideWorkspace(t *testing.T) {
	workspace := t.TempDir()
	insideAbs := filepath.Join(workspace, "pkg", "inside.go")

	normalized := normalizeWriteSetPaths(workspace, []string{
		"../escape.go",
		insideAbs,
		filepath.Join(workspace, "..", "outside.go"),
	})

	if len(normalized) != 1 {
		t.Fatalf("expected exactly 1 in-workspace path, got %d: %v", len(normalized), normalized)
	}

	expected := normalizeAbsolutePath(workspace, insideAbs)
	if normalized[0] != expected {
		t.Fatalf("normalized[0] = %q, want %q", normalized[0], expected)
	}
}

// -----------------------------------------------------------------------------
// Marathon 20: Write Set Lock Manager Gaps
// -----------------------------------------------------------------------------

func TestWriteSetLockManager_NullEmptyInputs(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())
	
	// empty/nil write set
	lease, err := manager.acquire(context.Background(), "t1", nil, time.Millisecond)
	if err != nil || lease != nil {
		t.Errorf("Expected nil lease/error for nil writeSet")
	}
	lease, err = manager.acquire(context.Background(), "t1", []string{}, time.Millisecond)
	if err != nil || lease != nil {
		t.Errorf("Expected nil lease/error for empty writeSet")
	}

	// nil context
	lease, err = manager.acquire(nil, "t2", []string{"a"}, time.Millisecond)
	if err != nil || lease == nil {
		t.Errorf("Expected success for nil context (should fallback to Background)")
	}
	if lease != nil { lease.release() }

	// empty/whitespace taskID
	lease, err = manager.acquire(context.Background(), "", []string{"a"}, time.Millisecond)
	if err == nil {
		t.Errorf("Expected error for empty taskID")
	}
	lease, err = manager.acquire(context.Background(), "   ", []string{"a"}, time.Millisecond)
	if err == nil {
		t.Errorf("Expected error for whitespace taskID")
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

func TestWriteSetLockManager_UserExtremes(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())
	
	// Massive write set
	var massive []string
	for i := 0; i < 10000; i++ {
		massive = append(massive, fmt.Sprintf("file_%d.go", i))
	}
	lease, err := manager.acquire(context.Background(), "t1", massive, time.Millisecond)
	if err != nil {
		t.Errorf("Failed to acquire massive write set: %v", err)
	}
	if lease != nil { lease.release() }

	// Poll interval 1ns
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel()
	// Lock first
	l1, _ := manager.acquire(context.Background(), "t2", []string{"a"}, time.Millisecond)
	// Try to acquire with 1ns poll interval
	_, err = manager.acquire(ctx, "t3", []string{"a"}, time.Nanosecond)
	if !errors.Is(err, ErrWriteSetLockTimeout) && err != context.DeadlineExceeded {
		t.Errorf("Expected timeout, got %v", err)
	}
	if l1 != nil { l1.release() }
}

func TestWriteSetLockManager_StateConflicts(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())
	
	// Re-entrancy of task ID locks
	l1, err := manager.acquire(context.Background(), "t1", []string{"a"}, time.Millisecond)
	if err != nil { t.Fatal(err) }
	
	l2, err := manager.acquire(context.Background(), "t1", []string{"a"}, time.Millisecond)
	if err != nil {
		t.Errorf("Expected success for re-entrant lock, got %v", err)
	}
	
	l2.release()
	
	// Check if lock is completely released or still held by l1
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()
	_, err = manager.acquire(ctx2, "t2", []string{"a"}, time.Millisecond)
	if err == nil {
		t.Errorf("Lock was completely released by l2, breaking idempotency for l1")
	}
	l1.release()

	// Double release
	l3, _ := manager.acquire(context.Background(), "t3", []string{"b"}, time.Millisecond)
	manager.releasePaths("t3", []string{"b"})
	l3.release() // Should not panic
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

func TestWriteSetLockManager_TypeCoercion(t *testing.T) {
	manager := newWriteSetLockManager(t.TempDir())
	
	// Complex/Bizarre paths
	paths := []string{"\x00", "a/../../b", "unprintable_\u0000", strings.Repeat("A", 3000)}
	lease, err := manager.acquire(context.Background(), "t1", paths, time.Millisecond)
	if err != nil {
		t.Errorf("Acquire with bizarre paths failed: %v", err)
	}
	if lease != nil { lease.release() }
}
