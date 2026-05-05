package campaign

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
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

// TODO: TEST_GAP: Null/Undefined/Empty Inputs - `acquire` with `nil` or empty `writeSet` arrays.
// A test is missing to verify that `acquire` handles empty slice `[]string{}` or `nil` correctly,
// returning a `nil` lease without attempting map modifications or panicking.

// TODO: TEST_GAP: Null/Undefined/Empty Inputs - `acquire` with `nil` context.
// The code relies on a fallback to `context.Background()` when `ctx == nil`.
// Tests should verify this branch behaves safely without panics or blocking infinitely.

// TODO: TEST_GAP: Null/Undefined/Empty Inputs - `acquire` with `""` taskID.
// There is an early return for `if taskID == ""` which should be explicitly covered
// to ensure invalid task IDs do not bypass mutual exclusion checks.

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

// TODO: TEST_GAP: User Request Extremes - Massive Write Sets.
// If a user commands changes across 100,000 files, `normalizeWriteSetPaths` allocates map and sorts slice synchronously.
// A benchmark/boundary test should evaluate contention on `m.mu.Lock()` holding up other tasks for hundreds of milliseconds.

// TODO: TEST_GAP: Type Coercion / OS State - Case sensitivity and Unicode normalization boundaries.
// On Windows, `File.go` and `file.go` lock the same path, but Linux handles them separately.
// The tests do not mock `runtime.GOOS` or verify dual-locking vulnerabilities when processing un-normalized Unicode paths.

// TODO: TEST_GAP: State Conflicts / Race Conditions - Re-entrancy of Task ID locks.
// `tryAcquirePaths` ignores held locks if `owner == taskID`. The test suite lacks coverage
// validating what happens when a single task attempts to re-acquire its own locks,
// and whether a single lease `release()` removes the lock globally, potentially breaking idempotency.

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

// TODO: TEST_GAP: Null/Empty: Verify behavior when workspace is an empty string in newWriteSetLockManager (does it allow arbitrary system paths?).
// TODO: TEST_GAP: Null/Empty: Verify acquire behavior when writeSet contains empty strings, whitespace-only strings, or null characters.
// TODO: TEST_GAP: Null/Empty: Verify acquire fails safely when taskID is only whitespace.
// TODO: TEST_GAP: Type Coercion/Formatting: Verify path normalization and boundary checks with complex relative paths (e.g., symlinks pointing outside workspace, paths with null bytes \x00).
// TODO: TEST_GAP: User Request Extremes: Verify performance and stability when acquiring a write set of 100,000 files (checking for mutex starvation).
// TODO: TEST_GAP: User Request Extremes: Verify behavior when pollInterval is set to 1ns (does it cause CPU exhaustion?).
// TODO: TEST_GAP: State Conflicts: Verify that if two concurrent requests use the EXACT SAME taskID, they do not both get granted the lock (Task ID collision breaking mutual exclusion).
// TODO: TEST_GAP: State Conflicts: Verify behavior if a lease is manually released via manager.releasePaths instead of lease.release(), and then lease.release() is called.
// TODO: TEST_GAP: User Request Extremes: Verify behavior when path length exceeds OS MAX_PATH limits (e.g., 3000 chars) due to deeply nested generated code structures.
// TODO: TEST_GAP: State Conflicts: Verify lack of hierarchical locking (Task A locking directory "src/", Task B locking file "src/a.go" - they will not conflict, which might be a bug depending on system design).
// TODO: TEST_GAP: Null/Empty: Verify acquire behavior when ctx is nil, ensuring it doesn't cause a panic but still respects bounds.
// TODO: TEST_GAP: Type Coercion: Verify behavior of normalizeAbsolutePath when given bizarre inputs like unprintable Unicode or paths entirely constructed of backslashes.
// TODO: TEST_GAP: User Request Extremes: Verify Memory/GC pressure when `normalizeWriteSetPaths` is called repeatedly with huge slices (e.g., in a tight polling loop).
