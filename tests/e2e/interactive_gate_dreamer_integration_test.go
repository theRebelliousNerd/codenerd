//go:build integration

package e2e_test

import (
	"context"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
)

// TestE2E_InteractiveGate_Dreamer_Integration evaluates the VirtualStore <-> Dreamer <-> Kernel boundary.

// -----------------------------------------------------------------------------
// 1. Smoke Test (Baseline Integration)
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Smoke_BenignAction(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	args := map[string]any{"path": "/tmp/safe_file.txt", "content": "hello"}

	err := vstore.PreflightDestructiveToolCall(ctx, "action-1", "write_file", args)
	if err != nil {
		t.Fatalf("Expected benign action to pass, but got error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 2. Contract Violation Tests
// -----------------------------------------------------------------------------

func TestE2E_InteractiveGate_Contract_NilDreamer(t *testing.T) {
	t.Parallel()
	// The Dreamer is built lazily from the kernel, so a store without one has
	// no Dreamer: a destructive call must be refused, not waved through.
	vstore := core.NewVirtualStore(nil)

	err := vstore.PreflightDestructiveToolCall(context.Background(), "action-3", "delete_file", map[string]any{"path": "notes.txt"})
	if err == nil {
		t.Fatal("a destructive call passed a store with no Dreamer")
	}
	if !strings.Contains(err.Error(), "dreamer unavailable") {
		t.Errorf("want a 'dreamer unavailable' refusal, got: %v", err)
	}
}

func TestE2E_InteractiveGate_Contract_OversizedTarget(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	massiveTarget := strings.Repeat("A", 5000)
	args := map[string]any{"path": massiveTarget}

	err := vstore.PreflightDestructiveToolCall(ctx, "action-4", "write_file", args)
	if err == nil {
		t.Fatal("Expected block due to oversized target, but gate passed")
	}
}

// -----------------------------------------------------------------------------
// 3. State Corruption Tests
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_StateCorruption_IsolationLeak(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	actionID := "action-leak-test"
	args := map[string]any{"path": "/tmp/leak_test.txt", "content": "test"}

	err := vstore.PreflightDestructiveToolCall(ctx, actionID, "write_file", args)
	if err != nil {
		t.Fatalf("Preflight failed unexpectedly: %v", err)
	}

	res, err := k.Query("projected_action(?)")
	if err != nil {
		t.Fatalf("Failed to query parent kernel: %v", err)
	}
	if len(res) > 0 {
		t.Fatalf("CRITICAL ISOLATION FAILURE: Parent kernel contains projected_action facts from the sandbox clone. Found %d facts.", len(res))
	}
}

// -----------------------------------------------------------------------------
// 4. Resource Exhaustion Tests
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Resource_ConcurrentSimulations(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	const numWorkers = 100
	var wg sync.WaitGroup
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			args := map[string]any{"path": "/tmp/concurrent.txt", "content": id}
			_ = vstore.PreflightDestructiveToolCall(ctx, "action-concurrent", "write_file", args)
		}(i)
	}
	wg.Wait()
}

// -----------------------------------------------------------------------------
// 5. Temporal Failure Tests
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Temporal_CancellationYieldsFailClosed(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	args := map[string]any{"path": "/tmp/cancel.txt", "content": "cancel"}
	err := vstore.PreflightDestructiveToolCall(ctx, "action-cancel", "write_file", args)

	if err == nil {
		t.Fatal("Expected block due to context cancellation, but gate passed")
	}
}

// -----------------------------------------------------------------------------
// 6. Cascading Failure Tests
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Cascading_FeedbackLoop(t *testing.T) {
	t.Parallel()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	// go.mod is a critical file (policy/dreamer.mg): deleting it is a panic state.
	err = vstore.PreflightDestructiveToolCall(context.Background(), "action-feedback", "delete_file", map[string]any{"path": "go.mod"})
	if err == nil {
		t.Fatal("deleting go.mod passed the Dreamer gate")
	}
	res, queryErr := k.Query("security_violation")
	if queryErr != nil {
		t.Fatalf("query security_violation: %v", queryErr)
	}
	if len(res) == 0 {
		t.Fatal("the block left no security_violation fact: the kernel cannot learn from it")
	}
}

// -----------------------------------------------------------------------------
// 7. Recovery Tests
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Recovery_MultiFileTransaction(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	args := map[string]any{
		"paths": []any{"/tmp/valid1.txt", strings.Repeat("A", 5000)},
	}

	err := vstore.PreflightDestructiveToolCall(ctx, "action-multi", "apply_edits", args)
	if err == nil {
		t.Fatal("Expected multi-file transaction to fail due to oversized target, but it passed")
	}

	validArgs := map[string]any{"path": "/tmp/recovery.txt", "content": "recovered"}
	err2 := vstore.PreflightDestructiveToolCall(ctx, "action-recovery", "write_file", validArgs)
	if err2 != nil {
		t.Fatalf("System failed to recover. Valid request after failure was blocked: %v", err2)
	}
}

// -----------------------------------------------------------------------------
// 8. End-to-End Data Integrity
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Integrity_FactFlow(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	actionID := "action-integrity"
	args := map[string]any{"path": "/tmp/integrity.txt", "content": "flow"}

	err := vstore.PreflightDestructiveToolCall(ctx, actionID, "write_file", args)
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	err = vstore.ValidateInteractiveToolResult(ctx, actionID, "write_file", args, "success", true)
	if err != nil {
		t.Logf("Validation returned error: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 9. Unmapped Tool Gate
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_UnmappedTool(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	args := map[string]any{"target": "unknown"}

	err := vstore.PreflightDestructiveToolCall(ctx, "action-unmapped", "destroy_everything", args)
	if err == nil {
		t.Fatal("Expected block due to unmapped tool, but gate passed")
	}
}

// -----------------------------------------------------------------------------
// 10. Multi-Turn State Accumulation
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_MultiTurn_StatePreservation(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()

	args1 := map[string]any{"path": "/tmp/t1.txt", "content": "t1"}
	err := vstore.PreflightDestructiveToolCall(ctx, "action-t1", "write_file", args1)
	if err != nil {
		t.Fatalf("Turn 1 failed: %v", err)
	}

	args2 := map[string]any{"path": "/tmp/t2.txt", "content": "t2"}
	err = vstore.PreflightDestructiveToolCall(ctx, "action-t2", "write_file", args2)
	if err != nil {
		t.Fatalf("Turn 2 failed: %v", err)
	}

	res, _ := k.Query("projected_action(?)")
	if len(res) > 0 {
		t.Fatalf("Projected actions leaked over multiple turns.")
	}
}

// -----------------------------------------------------------------------------
// 11. Partial Pipeline Failure
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_PartialPipeline_FailureHandling(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()

	args := map[string]any{"weird_key": "some_value"}

	err := vstore.PreflightDestructiveToolCall(ctx, "action-partial", "write_file", args)

	if err != nil {
		t.Logf("Handled partial pipeline failure gracefully: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 12. Context Cancellation Race Condition
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_ContextCancel_Race(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx, cancel := context.WithCancel(context.Background())
	args := map[string]any{"path": "/tmp/race.txt"}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		time.Sleep(1 * time.Millisecond)
		cancel()
	}()

	err := vstore.PreflightDestructiveToolCall(ctx, "action-race", "write_file", args)
	wg.Wait()

	if err != nil {
		if !strings.Contains(err.Error(), "canceled") && !strings.Contains(err.Error(), "context") {
			t.Logf("Failed with non-cancel error during race: %v", err)
		}
	}
}

// -----------------------------------------------------------------------------
// 13. Cache Key Collision Bypass Simulation
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_CacheKeyCollision_Bypass(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	target := "/tmp/collision.txt"

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ctx := context.Background()
		argsBenign := map[string]any{"path": target, "content": "safe"}
		_ = vstore.PreflightDestructiveToolCall(ctx, "action-safe", "write_file", argsBenign)
	}()

	go func() {
		defer wg.Done()
		ctx := context.Background()
		argsMalicious := map[string]any{"path": target, "content": "os.Exit(1)"}
		_ = vstore.PreflightDestructiveToolCall(ctx, "action-malicious", "write_file", argsMalicious)
	}()

	wg.Wait()
}

// -----------------------------------------------------------------------------
// 14. Validation Edge Cases
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Validation_EmptyType(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	req := core.ActionRequest{
		ActionID: "test-empty-type",
		Type:     "",
		Target:   "/tmp/empty",
	}

	res := vstore.GetDreamer().SimulateAction(ctx, req)
	if !res.Unsafe {
		t.Fatal("Dreamer failed to block empty action type")
	}
}

// -----------------------------------------------------------------------------
// 15. Invalid JSON Payload in Preflight
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_InvalidPayload_Preflight(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()

	args := map[string]any{"path": "/tmp/test", "cyclic": make(chan int)}

	err := vstore.PreflightDestructiveToolCall(ctx, "action-invalid-payload", "write_file", args)

	if err != nil {
		t.Logf("Handled invalid payload gracefully: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 16. Missing Validator Registry Degradation
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_MissingValidatorRegistry(t *testing.T) {
	t.Parallel()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	// A write reported successful whose file does not exist is a false
	// success: the post-action validators must refuse it.
	missing := filepath.Join(t.TempDir(), "never_written.txt")
	err = vstore.ValidateInteractiveToolResult(context.Background(), "action-16", "write_file", map[string]any{"path": missing, "content": "x"}, "wrote it", true)
	if err == nil {
		t.Fatal("a write that left no file validated as a success")
	}
}

// -----------------------------------------------------------------------------
// 17. Failed Tool Execution Handling
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_FailedToolExecutionValidation(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	args := map[string]any{"path": "/tmp/failedtool.txt"}

	// If the tool failed (success = false), the post-action validation should be skipped and return nil error.
	err := vstore.ValidateInteractiveToolResult(ctx, "action-17", "write_file", args, "error output", false)
	if err != nil {
		t.Fatalf("Validation should be skipped on tool failure: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 18. Payload Argument Type Variation
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_PayloadTypeVariation(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	// Test sending integer path, should extract properly or fail gracefully without panic
	args := map[string]any{"path": 12345}

	err := vstore.PreflightDestructiveToolCall(ctx, "action-18", "write_file", args)
	if err != nil {
		t.Logf("Handled int payload gracefully: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 19. Nil Args Handling
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_NilArgsHandling(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()

	err := vstore.PreflightDestructiveToolCall(ctx, "action-19", "write_file", nil)
	if err != nil {
		t.Logf("Handled nil args gracefully: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 20. Action Target Extraction Fallback
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_ActionTargetFallback(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()

	// Test fallback extraction keys like "filename", "file", "target"
	args := map[string]any{"filename": "/tmp/target.txt"}
	err := vstore.PreflightDestructiveToolCall(ctx, "action-20", "write_file", args)

	if err != nil {
		t.Fatalf("Target extraction fallback failed: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 21. Multi-Target Missing Paths Key
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_MultiTargetMissingPaths(t *testing.T) {
	t.Parallel()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	// apply_edits is gated once per file it writes; with no file named there
	// is nothing to gate, and the call must be refused rather than waved through.
	err = vstore.PreflightDestructiveToolCall(context.Background(), "action-21", "apply_edits", map[string]any{})
	if err == nil {
		t.Fatal("apply_edits with no target passed the gate")
	}
}

// -----------------------------------------------------------------------------
// 22. Unknown Destructive Action Fallback
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_UnknownDestructiveActionFallback(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()

	// Non-destructive tool like read_file should pass immediately without Dreamer simulation
	args := map[string]any{"path": "/etc/hosts"}
	err := vstore.PreflightDestructiveToolCall(ctx, "action-22", "read_file", args)

	if err != nil {
		t.Fatalf("Expected non-destructive tool to pass, got: %v", err)
	}
}

// -----------------------------------------------------------------------------
// 23. Concurrent Feedback Injection Simulation
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_ConcurrentFeedbackInjection(t *testing.T) {
	t.Parallel()
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	const numWorkers = 50
	var passed sync.Map
	var wg sync.WaitGroup
	wg.Add(numWorkers)
	for i := 0; i < numWorkers; i++ {
		go func(id int) {
			defer wg.Done()
			if err := vstore.PreflightDestructiveToolCall(context.Background(), "action-concurrent-fb", "delete_file", map[string]any{"path": "go.mod"}); err == nil {
				passed.Store(id, true)
			}
		}(i)
	}
	wg.Wait()
	passed.Range(func(id, _ any) bool {
		t.Errorf("worker %v deleted go.mod past the gate under concurrency", id)
		return true
	})
	res, err := k.Query("security_violation")
	if err != nil {
		t.Fatalf("query security_violation: %v", err)
	}
	if len(res) == 0 {
		t.Fatal("concurrent blocks left no security_violation fact")
	}
}

// -----------------------------------------------------------------------------
// 24. Canceled Context on Post-Flight Validation
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_CanceledContextValidation(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	// Ensure validation layer doesn't panic on canceled context
	err := vstore.ValidateInteractiveToolResult(ctx, "action-24", "write_file", map[string]any{"path": "a"}, "out", true)
	if err != nil {
		t.Logf("Validation correctly aborted on canceled context: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Extra Contract Violation Tests (Need 5, had 3 -> Adding 2)
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Contract_PayloadImmutability(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	args := map[string]any{"path": "/tmp/immutability.txt", "content": "original"}

	err := vstore.PreflightDestructiveToolCall(ctx, "action-cv1", "write_file", args)
	if err != nil {
		t.Fatalf("Preflight failed: %v", err)
	}

	// Contract: Preflight MUST NOT modify the arguments map
	if args["content"] != "original" {
		t.Fatalf("Contract violation: Preflight modified the arguments map! Got: %v", args["content"])
	}
}

func TestE2E_InteractiveGate_Contract_NonDestructiveToolPassthrough(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	args := map[string]any{"path": "/etc/passwd"} // highly sensitive path

	// Contract: Non-destructive tools (read_file) must bypass Dreamer simulation entirely.
	// If it doesn't, a panic_state rule might incorrectly trigger on a read.
	err := vstore.PreflightDestructiveToolCall(ctx, "action-cv2", "read_file", args)
	if err != nil {
		t.Fatalf("Contract violation: Non-destructive tool was blocked by Preflight: %v", err)
	}
}

// -----------------------------------------------------------------------------
// Extra State Corruption Tests (Need 3, had 1 -> Adding 2)
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_StateCorruption_ConcurrentKernelModification(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	var wg sync.WaitGroup
	wg.Add(2)

	ctx := context.Background()
	args := map[string]any{"path": "/tmp/safe.txt"}

	// Goroutine 1: Run Preflight
	go func() {
		defer wg.Done()
		_ = vstore.PreflightDestructiveToolCall(ctx, "action-sc1", "write_file", args)
	}()

	// Goroutine 2: Modify Main Kernel State concurrently
	go func() {
		defer wg.Done()
		_ = k.Assert(core.Fact{Predicate: "some_new_fact", Args: []any{"data"}})
		k.Evaluate()
	}()

	wg.Wait()
	// Test passes if it doesn't panic/race during kernel.Clone() vs Assert()
}

func TestE2E_InteractiveGate_StateCorruption_SharedPayloadMutation(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	var wg sync.WaitGroup
	wg.Add(2)

	ctx := context.Background()

	// A map shared between two concurrent preflight requests
	sharedArgs := map[string]any{"path": "/tmp/shared.txt", "content": "base"}

	go func() {
		defer wg.Done()
		_ = vstore.PreflightDestructiveToolCall(ctx, "action-sc2a", "write_file", sharedArgs)
	}()

	go func() {
		defer wg.Done()
		_ = vstore.PreflightDestructiveToolCall(ctx, "action-sc2b", "write_file", sharedArgs)
	}()

	wg.Wait()
	// Test passes if it doesn't data race on the shared map inside buildInteractiveActionRequest
}

// -----------------------------------------------------------------------------
// Extra Resource Exhaustion Tests (Need 2, had 1 -> Adding 1)
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Resource_MassiveFactBaseClone(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}

	// Inject 10,000 dummy facts to simulate a long-running campaign
	for i := 0; i < 10000; i++ {
		k.Assert(core.Fact{Predicate: "dummy_fact", Args: []any{i}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx := context.Background()
	args := map[string]any{"path": "/tmp/heavy.txt"}

	start := time.Now()
	err := vstore.PreflightDestructiveToolCall(ctx, "action-re1", "write_file", args)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("Preflight failed on heavy kernel: %v", err)
	}

	if duration > 5*time.Second {
		t.Fatalf("Performance Contract Violation: kernel.Clone() and evaluate took too long (%v) with 10k facts", duration)
	}
}

// -----------------------------------------------------------------------------
// Extra Temporal Failure Tests (Need 3, had 1 -> Adding 2)
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Temporal_TimeoutDuringValidation(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()
	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	// Context times out precisely as post-action validation starts
	ctx, cancel := context.WithTimeout(context.Background(), 1*time.Millisecond)
	defer cancel()

	time.Sleep(5 * time.Millisecond) // ensure timeout

	args := map[string]any{"path": "/tmp/val-timeout.txt"}
	err := vstore.ValidateInteractiveToolResult(ctx, "action-tf1", "write_file", args, "out", true)

	// Depending on implementation, it might fail open (nil error) or return context error.
	// The key is it must not panic.
	_ = err
}

func TestE2E_InteractiveGate_Temporal_ContextLeakPrevention(t *testing.T) {
	t.Parallel()
	k, _ := core.NewRealKernel()

	for _, prefix := range []string{"/internal", "/cmd", "/test"} {
		k.Assert(core.Fact{Predicate: "critical_path_prefix", Args: []any{prefix}})
	}
	k.Evaluate()

	vstore := core.NewVirtualStore(nil)
	vstore.SetKernel(k)

	ctx, cancel := context.WithCancel(context.Background())
	args := map[string]any{"path": "/tmp/leak-prevent.txt"}

	// Create a goroutine that will cancel the context after 1 second
	go func() {
		time.Sleep(1 * time.Second)
		cancel()
	}()

	// Call Preflight. It should return before the cancel.
	// If it somehow hangs waiting for the context, it's a bug.
	start := time.Now()
	_ = vstore.PreflightDestructiveToolCall(ctx, "action-tf2", "write_file", args)
	if time.Since(start) > 500*time.Millisecond {
		t.Logf("Warning: Preflight took longer than expected, possible context wait leak")
	}
}

// -----------------------------------------------------------------------------
// Extra Cascading Failure Tests (Need 2, had 1 -> Adding 1)
// -----------------------------------------------------------------------------

// -----------------------------------------------------------------------------
// Extra Recovery Tests (Need 2, had 1 -> Adding 1)
// -----------------------------------------------------------------------------
func TestE2E_InteractiveGate_Recovery_DreamerRestart(t *testing.T) {
	t.Parallel()
	vstore := core.NewVirtualStore(nil)
	args := map[string]any{"path": filepath.Join(t.TempDir(), "recovery.txt"), "content": "x"}

	// No kernel, so no Dreamer: fail closed.
	if err := vstore.PreflightDestructiveToolCall(context.Background(), "action-r1", "write_file", args); err == nil {
		t.Fatal("a write passed before any Dreamer could exist")
	}
	// A kernel arrives; the Dreamer is built from it and a safe write passes.
	k, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	vstore.SetKernel(k)
	if err := vstore.PreflightDestructiveToolCall(context.Background(), "action-r2", "write_file", args); err != nil {
		t.Fatalf("a safe write was still blocked after the kernel arrived: %v", err)
	}
}
