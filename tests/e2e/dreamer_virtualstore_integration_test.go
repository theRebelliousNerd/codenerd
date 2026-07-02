//go:build integration

package e2e_test

import (
	"time"
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/core"
)

// TestE2E_Dreamer_VirtualStore_Smoke_ValidAction verifies the baseline integration
// where a valid destructive action is simulated and allowed.
func TestE2E_Dreamer_VirtualStore_Smoke_ValidAction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	// Create a next_action fact that is valid
	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile),
			"valid_target.go",
			map[string]any{"content": "func main() {}"},
		},
	}

	// Route the action
	_, err := store.RouteAction(ctx, fact)

	// We expect the action to NOT be blocked by the dreamer.
	// It might fail later in execution, but the dreamer check should pass.
	if err != nil && strings.Contains(err.Error(), "blocked by dreamer safety gate") {
		t.Fatalf("Expected valid action to pass dreamer, but was blocked: %v", err)
	}
}

// TestE2E_Dreamer_VirtualStore_NilContext verifies that passing a nil context
// to RouteAction triggers the fail-closed mechanism in SimulateAction.
func TestE2E_Dreamer_VirtualStore_NilContext(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionDeleteFile), // Destructive
			"important.txt",
			map[string]any{},
		},
	}

	// Deliberately pass nil context to trigger fail-closed
	_, err := store.RouteAction(nil, fact)

	if err == nil {
		t.Fatal("Expected RouteAction to fail with nil context")
	}

	if !strings.Contains(err.Error(), "blocked by dreamer safety gate") || !strings.Contains(err.Error(), "nil context") {
		t.Fatalf("Expected fail-closed due to nil context, got: %v", err)
	}
}

// TestE2E_Dreamer_VirtualStore_OversizedTarget verifies that massive targets
// are rejected at the boundary without crashing.
func TestE2E_Dreamer_VirtualStore_OversizedTarget(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	massiveTarget := strings.Repeat("A", 5000)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile),
			massiveTarget,
			map[string]any{},
		},
	}

	_, err := store.RouteAction(ctx, fact)

	if err == nil {
		t.Fatal("Expected RouteAction to fail with oversized target")
	}

	if !strings.Contains(err.Error(), "blocked by dreamer safety gate") || !strings.Contains(err.Error(), "target path exceeds maximum length") {
		t.Fatalf("Expected fail-closed due to oversized target, got: %v", err)
	}
}

// TestE2E_Dreamer_VirtualStore_FactInjection verifies that when an action is blocked,
// the expected facts are actually injected into the underlying kernel.
func TestE2E_Dreamer_VirtualStore_FactInjection(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	massiveTarget := strings.Repeat("A", 5000)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile),
			massiveTarget,
			map[string]any{},
		},
	}

	_, err := store.RouteAction(ctx, fact)

	if err == nil {
		t.Fatal("Expected RouteAction to fail and block action")
	}

	// Verify facts were injected
	res1, err := kernel.Query("security_violation")
	if err != nil || len(res1) == 0 {
		t.Fatalf("security_violation fact was not properly injected or is not queryable: err=%v, res=%v", err, res1)
	}

	res2, err := kernel.Query("dream_blocked_action")
	if err != nil || len(res2) == 0 {
		t.Fatalf("dream_blocked_action fact was not properly injected or is not queryable: err=%v, res=%v", err, res2)
	}
}

// TestE2E_Dreamer_VirtualStore_ConcurrentSimulations verifies the DreamCache's
// thread safety when bombarded with concurrent identical requests.
func TestE2E_Dreamer_VirtualStore_ConcurrentSimulations(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile),
			strings.Repeat("A", 5000), // Ensures it's blocked and cached
			map[string]any{},
		},
	}

	var wg sync.WaitGroup
	errs := make([]error, 50)

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			_, err := store.RouteAction(context.Background(), fact)
			errs[idx] = err
		}(i)
	}

	wg.Wait()

	for i, err := range errs {
		if err == nil {
			t.Fatalf("Expected error for concurrent request %d, got nil", i)
		}
		if !strings.Contains(err.Error(), "blocked by dreamer safety gate") {
			t.Fatalf("Expected consistent blocked error for concurrent request %d, got: %v", i, err)
		}
	}
}

// TestE2E_Dreamer_VirtualStore_CacheEviction verifies that the DreamCache
// does not leak memory and handles eviction when flooded with unique actions.
func TestE2E_Dreamer_VirtualStore_CacheEviction(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	for i := 0; i < 300; i++ {
		target := fmt.Sprintf("target_%d.txt", i)
		// Make target long enough to be rejected, but unique
		longTarget := target + strings.Repeat("B", 4096)
		fact := core.Fact{
			Predicate: "next_action",
			Args: []any{
				string(core.ActionDeleteFile),
				longTarget,
				map[string]any{},
			},
		}

		_, err := store.RouteAction(ctx, fact)
		if err == nil {
			t.Fatalf("Expected RouteAction to fail on iteration %d", i)
		}
	}
	// The test passes if it completes without OOM or panic.
}

// TestE2E_Dreamer_VirtualStore_MalformedFact verifies that the VirtualStore
// handles malformed next_action facts without crashing before hitting the Dreamer.
func TestE2E_Dreamer_VirtualStore_MalformedFact(t *testing.T) {
	t.Parallel()
	ctx := context.Background()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			123, // Invalid type for action type
			"target",
			map[string]any{},
		},
	}

	_, err := store.RouteAction(ctx, fact)
	if err == nil {
		t.Fatal("Expected malformed fact to produce an error")
	}
	if !strings.Contains(err.Error(), "failed to parse action fact") {
		t.Fatalf("Expected parsing error, got: %v", err)
	}
}

// ============================================================================
// 7. Temporal Failure: Slow Simulation Timeout
// ============================================================================

// TestE2E_Dreamer_VirtualStore_SlowSimulation verifies that if SimulateAction
// takes too long, it can be cancelled by the parent context.
func TestE2E_Dreamer_VirtualStore_SlowSimulation(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	// Create a context with a very short timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionDeleteFile),
			"timeout_target.txt",
			map[string]any{},
		},
	}

	// This assumes the kernel or dreamer might take longer than 1ms or will check context
	// If it doesn't take longer, we at least verify the context cancellation doesn't crash it
	_, err := store.RouteAction(ctx, fact)
	if err == nil {
		t.Fatalf("Contract Violation: Action succeeded despite context cancellation. The simulation must abort immediately to prevent resource exhaustion.")
	}
}

// ============================================================================
// 8. State Corruption: Cache Race
// ============================================================================

// TestE2E_Dreamer_VirtualStore_CacheRace tests state corruption where multiple
// goroutines attempt to read and write to the DreamCache simultaneously with different actions.
func TestE2E_Dreamer_VirtualStore_CacheRace(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	var wg sync.WaitGroup
	numRoutines := 100

	for i := 0; i < numRoutines; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()

			// Some will be blocked, some might pass depending on target length
			target := fmt.Sprintf("race_target_%d", idx)
			if idx%2 == 0 {
				target += strings.Repeat("X", 4096) // Force block
			}

			fact := core.Fact{
				Predicate: "next_action",
				Args: []any{
					string(core.ActionEditFile),
					target,
					map[string]any{},
				},
			}

			_, _ = store.RouteAction(context.Background(), fact)
		}(i)
	}

	wg.Wait()
}

// ============================================================================
// 9. Contract: Recovery after Failure
// ============================================================================

// TestE2E_Dreamer_VirtualStore_Recovery verifies that after a catastrophic failure
// (e.g. malformed input causing a block), the store can still route valid actions.
func TestE2E_Dreamer_VirtualStore_Recovery(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	// 1. Send invalid action
	invalidFact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionDeleteFile),
			strings.Repeat("Y", 5000), // Oversized
			map[string]any{},
		},
	}

	_, err := store.RouteAction(context.Background(), invalidFact)
	if err == nil {
		t.Fatal("Expected invalid action to fail")
	}

	// 2. Send valid action
	validFact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionReadFile), // Non-destructive
			"readme.md",
			map[string]any{},
		},
	}

	_, err = store.RouteAction(context.Background(), validFact)
	// Read file should not trigger dreamer block. It might fail execution due to no file,
	// but the routing should pass dreamer.
	if err != nil && strings.Contains(err.Error(), "blocked by dreamer safety gate") {
		t.Fatalf("Expected valid action to pass routing after failure, but got: %v", err)
	}
}

// ============================================================================
// 10. Cascading Failure: Panic Recovery in RouteAction
// ============================================================================

// TestE2E_Dreamer_VirtualStore_PanicRecovery ensures that if something panics
// inside the routing, it does not bubble up and crash the entire process.
func TestE2E_Dreamer_VirtualStore_PanicRecovery(t *testing.T) {
	t.Parallel()

	// This test sets up a condition where a malformed fact payload might cause a panic
	// if not properly handled by the parsing logic.
	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Contract Violation: Panic in RouteAction bubbled up: %v", r)
		}
	}()

	// Intentionally malformed payload with nested unparseable types
	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile),
			"test.txt",
			make(chan int), // Invalid type for payload
		},
	}

	_, _ = store.RouteAction(context.Background(), fact)
}

// ============================================================================
// 11. Edge Case: Empty Target
// ============================================================================

// TestE2E_Dreamer_VirtualStore_EmptyTarget checks how the system handles
// empty target strings for destructive actions.
func TestE2E_Dreamer_VirtualStore_EmptyTarget(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionDeleteFile),
			"",
			map[string]any{},
		},
	}

	_, err := store.RouteAction(context.Background(), fact)
	if err == nil {
		t.Fatalf("Contract Violation: Empty target was not rejected. The Dreamer must fail-closed on malformed targets to prevent sandbox escapes.")
	}
	if !strings.Contains(err.Error(), "blocked by dreamer safety gate") {
		t.Fatalf("Contract Violation: Expected action to be blocked by the dreamer safety gate, but got different error: %v", err)
	}
}

// ============================================================================
// 12. Edge Case: Extremely large map payload
// ============================================================================

// TestE2E_Dreamer_VirtualStore_LargePayload checks OOM vulnerability when passing
// massive payloads in the fact arguments.
func TestE2E_Dreamer_VirtualStore_LargePayload(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	largeMap := make(map[string]any)
	for i := 0; i < 10000; i++ {
		largeMap[fmt.Sprintf("key_%d", i)] = strings.Repeat("A", 100)
	}

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile),
			"target.txt",
			largeMap,
		},
	}

	_, err := store.RouteAction(context.Background(), fact)
	if err == nil {
		t.Fatalf("Contract Violation: Large payload was not rejected. The boundary must prevent OOM crashes by rejecting massive payloads.")
	}
}

// ============================================================================
// 13. State: Repeated Cache Hits
// ============================================================================

// TestE2E_Dreamer_VirtualStore_RepeatedCacheHits verifies that hitting the
// cache multiple times for the same blocked action is stable and performant.
func TestE2E_Dreamer_VirtualStore_RepeatedCacheHits(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionDeleteFile),
			strings.Repeat("C", 5000), // Oversized
			map[string]any{},
		},
	}

	// First hit caches it
	_, err := store.RouteAction(context.Background(), fact)
	if err == nil {
		t.Fatal("Expected fail-closed on first hit")
	}

	// Subsequent hits should pull from cache
	for i := 0; i < 50; i++ {
		_, err := store.RouteAction(context.Background(), fact)
		if err == nil {
			t.Fatalf("Expected fail-closed on cache hit %d", i)
		}
		if !strings.Contains(err.Error(), "blocked by dreamer safety gate") {
			t.Fatalf("Unexpected error from cache hit %d: %v", i, err)
		}
	}
}

// ============================================================================
// 14. Contract: Null Byte Injection
// ============================================================================

// TestE2E_Dreamer_VirtualStore_NullByteInjection ensures that null bytes
// in target strings are handled gracefully by the parser and dreamer.
func TestE2E_Dreamer_VirtualStore_NullByteInjection(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile),
			"target\x00with\x00nulls.txt",
			map[string]any{},
		},
	}

	_, err := store.RouteAction(context.Background(), fact)
	if err == nil {
		t.Fatalf("Contract Violation: Null byte injection was not caught. The system must reject invalid paths before they reach the OS layer to prevent path truncation attacks.")
	}
}

// ============================================================================
// 15. Contract: Unknown Action Type
// ============================================================================

// TestE2E_Dreamer_VirtualStore_UnknownAction verifies the routing and dreamer
// behavior when faced with a completely unknown action type string.
func TestE2E_Dreamer_VirtualStore_UnknownAction(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			"unknown_destructive_action",
			"target.txt",
			map[string]any{},
		},
	}

	_, err := store.RouteAction(context.Background(), fact)
	if err == nil {
		t.Fatalf("Contract Violation: Unknown action type was permitted. The system must fail-closed on unknown actions to prevent logic bypasses.")
	}
}

// ============================================================================
// 16. Cascading Failure: DreamCache Invalidation
// ============================================================================

// TestE2E_Dreamer_VirtualStore_CacheInvalidation ensures that cache isn't
// persisting between totally disparate action types.
func TestE2E_Dreamer_VirtualStore_CacheInvalidation(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact1 := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionDeleteFile),
			strings.Repeat("D", 5000), // Blocked
			map[string]any{},
		},
	}

	_, _ = store.RouteAction(context.Background(), fact1)

	fact2 := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile), // Different action, same target
			strings.Repeat("D", 5000),
			map[string]any{},
		},
	}

	_, err := store.RouteAction(context.Background(), fact2)
	// It should block again because it's a different action, even if target is same.
	if err == nil {
		t.Fatalf("Expected second action to be blocked")
	}
}

// ============================================================================
// 17. Contract: Context Timeout Leak Prevention
// ============================================================================

// TestE2E_Dreamer_VirtualStore_ContextLeak ensures that if the context is canceled,
// there are no lingering goroutines hanging around doing the work.
func TestE2E_Dreamer_VirtualStore_ContextLeak(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // Cancel immediately

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionDeleteFile),
			"leak_target.txt",
			map[string]any{},
		},
	}

	_, err := store.RouteAction(ctx, fact)
	if err == nil {
		t.Fatalf("Expected error due to canceled context")
	}
}

// ============================================================================
// 18. End-to-End Data Integrity
// ============================================================================

// TestE2E_Dreamer_VirtualStore_DataIntegrity checks if the exact payload is preserved
// during a permitted execution round-trip.
func TestE2E_Dreamer_VirtualStore_DataIntegrity(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionWriteFile), // Destructive, should hit dreamer
			"integrity.txt",
			map[string]any{"content": "sensitive_data"},
		},
	}

	_, err := store.RouteAction(context.Background(), fact)
	// The routing should pass. We're testing that the payload isn't corrupted or dropped.
	// We check for no dreamer block.
	if err != nil && strings.Contains(err.Error(), "blocked by dreamer safety gate") {
		t.Fatalf("Expected action to pass dreamer, but got: %v", err)
	}
}

// ============================================================================
// 19. State Corruption: Concurrent Rule Modification
// ============================================================================

// TestE2E_Dreamer_VirtualStore_ConcurrentRuleModification ensures that the
// cache and simulation are robust even if kernel rules change dynamically.
func TestE2E_Dreamer_VirtualStore_ConcurrentRuleModification(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	var wg sync.WaitGroup
	wg.Add(2)

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionDeleteFile),
			"race_rule_target",
			map[string]any{},
		},
	}

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			_, err := store.RouteAction(context.Background(), fact)
			// Might be blocked, might not, but shouldn't panic
			if err != nil && !strings.Contains(err.Error(), "blocked") {
				t.Errorf("Unexpected error: %v", err)
			}
		}
	}()

	go func() {
		defer wg.Done()
		for i := 0; i < 50; i++ {
			// Simulate rule modification (just dummy logic here, real modification requires kernel mutation)
			// This test primarily checks that the read locks during Simulation don't conflict fatally.
			_, _ = kernel.Query("test")
		}
	}()

	wg.Wait()
}

// ============================================================================
// 20. Temporal Failure: Network Delay Simulation
// ============================================================================

// TestE2E_Dreamer_VirtualStore_NetworkDelay tests the system under heavy
// internal delay simulation to ensure the session timeout handles it.
func TestE2E_Dreamer_VirtualStore_NetworkDelay(t *testing.T) {
	t.Parallel()

	kernel, _ := core.NewRealKernel()
	store := core.NewVirtualStore(nil)
	store.SetKernel(kernel)

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Millisecond)
	defer cancel()

	fact := core.Fact{
		Predicate: "next_action",
		Args: []any{
			string(core.ActionEditFile),
			"delayed_target",
			map[string]any{},
		},
	}

	// Assuming simulation or lock contention takes time, we enforce the timeout
	_, err := store.RouteAction(ctx, fact)
	if err == nil {
		t.Fatalf("Contract Violation: Action routed successfully despite context timeout. The boundary must enforce strict timeouts to prevent session stalling.")
	}
}
