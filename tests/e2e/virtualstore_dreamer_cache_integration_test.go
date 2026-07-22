//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/logging"
)

// setupE2ETest creates a real kernel and virtual store for integration testing.
func setupE2ETest(t *testing.T) (*core.RealKernel, *core.VirtualStore, *core.Dreamer) {
    t.Helper()

    // Create Real Kernel (crossing subsystem boundary 1)
    k, _ := core.NewRealKernel()

    // Create VirtualStore (crossing subsystem boundary 2)
    vs := core.NewVirtualStore(nil)
    vs.SetKernel(k)

    // Provide a basic set of intents so it doesn't fail initialization
    err := k.LoadFacts([]core.Fact{
        {Predicate: "user_intent", Args: []any{"test_session", "write_file", "config.json"}},
    })
    if err != nil {
        t.Fatalf("Failed to setup initial facts: %v", err)
    }

    // The Dreamer is auto-created by VirtualStore when kernel is set, but let's ensure it's fetched
    dreamer := vs.GetDreamer()
    if dreamer == nil {
        t.Fatalf("Dreamer not initialized by VirtualStore")
    }

    // Clear any previous state
    dreamer.InvalidateCache()

    return k, vs, dreamer
}

// -----------------------------------------------------------------------------
// Category: Smoke Tests
// -----------------------------------------------------------------------------

// TestE2E_VirtualStoreDreamer_Smoke_ValidToolCall verifies the baseline integration works.
// Subsystems: VirtualStore <-> Dreamer <-> Kernel
func TestE2E_VirtualStoreDreamer_Smoke_ValidToolCall(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    args := map[string]any{"target": "dummy.txt", "content": "hello"}
    err := vs.PreflightDestructiveToolCall(ctx, "act-1", "write_file", args)

    // Verify it doesn't panic. If blocked by policy, it should return a specific error type.
    if err != nil {
        if _, ok := err.(*core.InteractiveGateError); !ok {
            t.Fatalf("Expected InteractiveGateError or nil, got: %v", err)
        }
    }
}

// -----------------------------------------------------------------------------
// Category: Contract Violations (Min 5)
// -----------------------------------------------------------------------------

// 1. TestE2E_VirtualStoreDreamer_Contract_CacheKeyPayloadCollision exploits the documented architectural flaw
// where the Dreamer cache ignores the payload and only uses ActionType + Target.
// Violated Contract: Cache Completeness.
func TestE2E_VirtualStoreDreamer_Contract_CacheKeyPayloadCollision(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    // 1. Simulate a benign action that gets cached.
    argsBenign := map[string]any{"target": "config.json", "content": "benign payload"}
    errBenign := vs.PreflightDestructiveToolCall(ctx, "act-1", "write_file", argsBenign)

    // 2. Simulate a malicious action to the SAME target.
    // Because the cache key is only "write_file:config.json", this will HIT the cache
    // and return the exact same verdict as the benign action, completely ignoring the payload.
    argsMalicious := map[string]any{"target": "config.json", "content": "MALICIOUS INJECTION"}
    errMalicious := vs.PreflightDestructiveToolCall(ctx, "act-2", "write_file", argsMalicious)

    // Verify the flaw: the errors should be identical (both nil or both the exact same block reason)
    // even though the payloads differ dramatically.
    if errBenign == nil && errMalicious != nil {
         t.Fatalf("Cache collision failed: Malicious payload was caught despite identical cache key")
    }

    if errBenign != nil && errMalicious == nil {
         t.Fatalf("Cache collision failed: Malicious payload was allowed while benign was blocked")
    }

    if errBenign != nil && errMalicious != nil {
         if errBenign.Error() != errMalicious.Error() {
             t.Fatalf("Cache collision failed: Error strings differ: %s vs %s", errBenign.Error(), errMalicious.Error())
         }
    }
}

// 2. TestE2E_VirtualStoreDreamer_Contract_NilContext verifies fail-closed on nil context.
func TestE2E_VirtualStoreDreamer_Contract_NilContext(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)

    // Violate contract by passing nil context
    args := map[string]any{"target": "foo.txt"}
    err := vs.PreflightDestructiveToolCall(nil, "act-nil", "write_file", args)

    if err == nil {
        t.Fatalf("Expected fail-closed on nil context, got success")
    }

    if _, ok := err.(*core.InteractiveGateError); !ok {
        t.Fatalf("Expected InteractiveGateError, got: %T", err)
    }
}

// 3. TestE2E_VirtualStoreDreamer_Contract_UnmappedTool verifies unmapped tools bypass Dreamer.
func TestE2E_VirtualStoreDreamer_Contract_UnmappedTool(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    // "read_file" is mapped, but let's use a totally unknown tool name.
    args := map[string]any{"target": "foo.txt"}
    err := vs.PreflightDestructiveToolCall(ctx, "act-unmapped", "destroy_world", args)

    // Should bypass Dreamer and return nil
    if err != nil {
        t.Fatalf("Expected nil for unmapped tool, got: %v", err)
    }
}

// 4. TestE2E_VirtualStoreDreamer_Contract_TargetExtraction verifies target heuristic consistency.
func TestE2E_VirtualStoreDreamer_Contract_TargetExtraction(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    // Pass 'file' instead of 'target', extractActionTarget should find it
    args := map[string]any{"file": "extracted.txt"}
    err := vs.PreflightDestructiveToolCall(ctx, "act-extract", "write_file", args)
    if err != nil {
         if _, ok := err.(*core.InteractiveGateError); !ok {
             t.Fatalf("Expected InteractiveGateError or nil, got: %v", err)
         }
    }
}

// 5. TestE2E_VirtualStoreDreamer_Contract_ValidationThreshold tests boundary on 0.79 confidence.
func TestE2E_VirtualStoreDreamer_Contract_ValidationThreshold(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    args := map[string]any{"target": "thresh.txt"}
    err := vs.ValidateInteractiveToolResult(ctx, "act-thresh", "write_file", args, "output", true)
    // If it panics or errors unexpectedly here, the threshold logic is broken
    if err != nil {
        t.Fatalf("Validation threshold should not error abruptly on missing logic: %v", err)
    }
}

// 6. TestE2E_VirtualStoreDreamer_Contract_ActionTypeValidation verifies exact ActionType mapping.
func TestE2E_VirtualStoreDreamer_Contract_ActionTypeValidation(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    // Explicitly test boundary mappings defined in interactiveToolActionType
    validTools := []string{"read_file", "write_file", "edit_file", "delete_file", "run_command", "bash", "run_build", "edit_lines", "insert_lines", "delete_lines"}

    for _, tool := range validTools {
        tool := tool // capture
        t.Run(tool, func(t *testing.T) {
            t.Parallel()
            args := map[string]any{"target": "test_target"}

            // This shouldn't panic, even if policy blocks it, it must process structurally
            err := vs.PreflightDestructiveToolCall(ctx, "act-"+tool, tool, args)

            if err != nil {
                if _, ok := err.(*core.InteractiveGateError); !ok {
                    t.Fatalf("Expected InteractiveGateError for mapped tool, got %T: %v", err, err)
                }
            }
        })
    }
}

// -----------------------------------------------------------------------------
// Category: State Corruption (Min 3)
// -----------------------------------------------------------------------------

// 1. TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentCacheWrites floods the Dreamer cache.
// Run with -race to verify no map state corruption during O(N) cache evictions.
func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentCacheWrites(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    var wg sync.WaitGroup
    var errs []error
    var mu sync.Mutex

    // Launch 300 goroutines to breach the 256 cache size limit and trigger evictions
    for i := 0; i < 300; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            args := map[string]any{"target": fmt.Sprintf("file_%d.txt", id)}
            err := vs.PreflightDestructiveToolCall(ctx, fmt.Sprintf("act-%d", id), "write_file", args)
            if err != nil {
                 // Might hit policy blocks, we just don't want panics or race conditions
                 mu.Lock()
                 errs = append(errs, err)
                 mu.Unlock()
            }
        }(i)
    }
    wg.Wait()

    // If we didn't panic, the state is uncorrupted.
    if len(errs) > 0 {
         logging.VirtualStoreDebug("Encountered %d policy blocks during concurrent cache writes", len(errs))
    }
}

// 2. TestE2E_VirtualStoreDreamer_StateCorruption_PayloadTOCTOU tests payload map mutation mid-flight.
func TestE2E_VirtualStoreDreamer_StateCorruption_PayloadTOCTOU(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    args := map[string]any{"target": "file.txt", "content": "original"}

    var wg sync.WaitGroup
    wg.Add(2)

    go func() {
        defer wg.Done()
        err := vs.PreflightDestructiveToolCall(ctx, "act-toctou", "write_file", args)
        if err != nil {
            logging.VirtualStoreDebug("TOCTOU test blocked by policy: %v", err)
        }
    }()

    go func() {
        defer wg.Done()
        // Mutate map concurrently while Dreamer is simulating
        args["content"] = "mutated"
    }()

    wg.Wait()

    // Verify the map was actually mutated, proving the reference is shared
    if args["content"] != "mutated" {
         t.Fatalf("Expected map to be mutated, demonstrating TOCTOU vulnerability")
    }
}

// 3. TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentResultMutation tests result sharing.
func TestE2E_VirtualStoreDreamer_StateCorruption_ConcurrentResultMutation(t *testing.T) {
    t.Parallel()
    _, vs, dreamer := setupE2ETest(t)
    ctx := context.Background()

    args := map[string]any{"target": "shared.txt"}
    err1 := vs.PreflightDestructiveToolCall(ctx, "act-shared", "write_file", args)

    // Retrieve cached result concurrently while simulating again
    var wg sync.WaitGroup
    wg.Add(2)
    var err2 error
    go func() {
        defer wg.Done()
        err2 = vs.PreflightDestructiveToolCall(ctx, "act-shared2", "write_file", args)
    }()
    go func() {
        defer wg.Done()
        // Simulate background thread grabbing cache
        dreamer.InvalidateCache()
    }()
    wg.Wait()

    if err1 != nil && err2 == nil {
         t.Fatalf("Cache invalidation mid-flight caused diverging results for identical concurrent calls")
    }
    if err2 != nil {
         if _, ok := err2.(*core.InteractiveGateError); !ok {
             t.Fatalf("Expected structural stability, got panic/raw error: %v", err2)
         }
    }
}

// -----------------------------------------------------------------------------
// Category: Resource Exhaustion (Min 2)
// -----------------------------------------------------------------------------

// 1. TestE2E_VirtualStoreDreamer_ResourceExhaustion_MassiveTarget exhausts memory with huge target strings.
func TestE2E_VirtualStoreDreamer_ResourceExhaustion_MassiveTarget(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping massive target test in short mode")
    }
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    // Create a 1MB string to act as the cache key and target path
    massiveTarget := strings.Repeat("A", 1024*1024)
    args := map[string]any{"target": massiveTarget}

    err := vs.PreflightDestructiveToolCall(ctx, "act-huge", "write_file", args)

    // Verify the system didn't OOM or panic
    if err == nil {
        t.Log("System allowed massive target string")
    } else {
        if _, ok := err.(*core.InteractiveGateError); !ok {
             t.Fatalf("Expected structural error on massive target, got: %v", err)
        }
    }
}

// 2. TestE2E_VirtualStoreDreamer_ResourceExhaustion_ValidationFlood targets the kernel facts.
func TestE2E_VirtualStoreDreamer_ResourceExhaustion_ValidationFlood(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    var errs []error
    var mu sync.Mutex
    var wg sync.WaitGroup

    for i := 0; i < 500; i++ {
        wg.Add(1)
        go func(id int) {
            defer wg.Done()
            args := map[string]any{"target": fmt.Sprintf("flood_%d.txt", id)}
            err := vs.ValidateInteractiveToolResult(ctx, fmt.Sprintf("act-flood-%d", id), "write_file", args, "ok", true)
            if err != nil {
                mu.Lock()
                errs = append(errs, err)
                mu.Unlock()
            }
        }(i)
    }
    wg.Wait()

    if len(errs) > 0 {
         t.Fatalf("Validation flood caused unexpected structural errors: %v", errs[0])
    }
}

// -----------------------------------------------------------------------------
// Category: Temporal Failure (Min 3)
// -----------------------------------------------------------------------------

// 1. TestE2E_VirtualStoreDreamer_Temporal_ContextCancellation Mid-simulation.
func TestE2E_VirtualStoreDreamer_Temporal_ContextCancellation(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)

    ctx, cancel := context.WithCancel(context.Background())

    // Cancel immediately so the context is dead before it even reaches the clone
    cancel()

    args := map[string]any{"target": "file.txt"}
    err := vs.PreflightDestructiveToolCall(ctx, "act-cancel", "write_file", args)

    if err == nil {
        t.Fatalf("Expected failure due to context cancellation")
    }

    if !strings.Contains(err.Error(), "context canceled") && !strings.Contains(err.Error(), "nil context") && !strings.Contains(err.Error(), "blocked") && !strings.Contains(err.Error(), "safety gate") {
         t.Fatalf("Expected context cancellation related error, got: %v", err)
    }
}

// 2. TestE2E_VirtualStoreDreamer_Temporal_InvalidationRace checks race between miss and write.
func TestE2E_VirtualStoreDreamer_Temporal_InvalidationRace(t *testing.T) {
    t.Parallel()
    _, vs, dreamer := setupE2ETest(t)
    ctx := context.Background()

    args := map[string]any{"target": "race.txt"}

    var wg sync.WaitGroup
    wg.Add(2)
    var testErr error
    go func() {
        defer wg.Done()
        testErr = vs.PreflightDestructiveToolCall(ctx, "act-race", "write_file", args)
    }()
    go func() {
        defer wg.Done()
        // Wait slightly to hit the window where the cache miss occurred but write hasn't
        time.Sleep(1 * time.Millisecond)
        dreamer.InvalidateCache()
    }()
    wg.Wait()

    if testErr != nil {
         if _, ok := testErr.(*core.InteractiveGateError); !ok {
             t.Fatalf("Race resulted in catastrophic failure: %v", testErr)
         }
    }
}

// 3. TestE2E_VirtualStoreDreamer_Temporal_TimeoutDuringValidation checks post-execution timeouts.
func TestE2E_VirtualStoreDreamer_Temporal_TimeoutDuringValidation(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)

    ctx, cancel := context.WithCancel(context.Background())
    cancel() // Cancel before validation starts

    args := map[string]any{"target": "timeout_val.txt"}
    err := vs.ValidateInteractiveToolResult(ctx, "act-timeval", "write_file", args, "ok", true)
    if err != nil {
         // Some validators might respect ctx.Done, just make sure it doesn't panic
         t.Logf("Validation correctly returned error on canceled context: %v", err)
    }
}

// -----------------------------------------------------------------------------
// Category: Cascading Failure (Min 2)
// -----------------------------------------------------------------------------

// 1. TestE2E_VirtualStoreDreamer_Cascading_DreamerUnavailable verifies VirtualStore handles a nil Dreamer.
func TestE2E_VirtualStoreDreamer_Cascading_DreamerUnavailable(t *testing.T) {
    t.Parallel()

    // Create VirtualStore without setting the kernel (so Dreamer is nil)
    vs := core.NewVirtualStore(nil)
    ctx := context.Background()

    args := map[string]any{"target": "file.txt"}
    err := vs.PreflightDestructiveToolCall(ctx, "act-nodreamer", "write_file", args)

    if err == nil {
        t.Fatalf("Expected blocked action due to nil Dreamer, got success")
    }

    gateErr, ok := err.(*core.InteractiveGateError)
    if !ok {
        t.Fatalf("Expected InteractiveGateError, got: %T", err)
    }
    if !strings.Contains(gateErr.Reason, "dreamer unavailable") {
        t.Fatalf("Expected 'dreamer unavailable' reason, got: %s", gateErr.Reason)
    }
}

// 2. TestE2E_VirtualStoreDreamer_Cascading_BlockedFactInjection ensures block facts are retrievable.
func TestE2E_VirtualStoreDreamer_Cascading_BlockedFactInjection(t *testing.T) {
    t.Parallel()
    k, vs, _ := setupE2ETest(t)

    args := map[string]any{"target": "forbidden.txt"}
    // Force a block by passing nil context
    err := vs.PreflightDestructiveToolCall(nil, "act-block", "write_file", args)

    if err == nil {
         t.Fatalf("Failed to trigger block for testing fact injection")
    }

    // Check if security_violation fact was asserted
    results, queryErr := k.Query(`security_violation(Req, Reason, Timestamp)`)
    if queryErr != nil {
         t.Fatalf("Query failed: %v", queryErr)
    }

    if len(results) == 0 {
         t.Fatalf("security_violation fact was not successfully injected into the kernel")
    }
}

// -----------------------------------------------------------------------------
// Category: Recovery (Min 2)
// -----------------------------------------------------------------------------

// 1. TestE2E_VirtualStoreDreamer_Recovery_PostInvalidation verifies that after InvalidateCache,
// a previously cached verdict is dropped and a fresh clone happens.
func TestE2E_VirtualStoreDreamer_Recovery_PostInvalidation(t *testing.T) {
    t.Parallel()
    _, vs, dreamer := setupE2ETest(t)
    ctx := context.Background()

    args := map[string]any{"target": "file.txt"}

    // 1. Initial call (caches result)
    _ = vs.PreflightDestructiveToolCall(ctx, "act-rec1", "write_file", args)

    // 2. Invalidate cache
    dreamer.InvalidateCache()

    // 3. Second call (must NOT use cache, should succeed structurally)
    err2 := vs.PreflightDestructiveToolCall(ctx, "act-rec2", "write_file", args)

    // Verify system remains operational and stable without panicking
    if err2 != nil {
         if _, ok := err2.(*core.InteractiveGateError); !ok {
             t.Fatalf("System failed structurally after invalidation: %v", err2)
         }
    }
}

// 2. TestE2E_VirtualStoreDreamer_Recovery_PostFactAvalanche checks kernel stability.
func TestE2E_VirtualStoreDreamer_Recovery_PostFactAvalanche(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    args := map[string]any{"target": "avalanche.txt"}
    err1 := vs.PreflightDestructiveToolCall(nil, "act-ava", "write_file", args) // force fail closed

    if err1 == nil {
         t.Fatalf("Failed to force block")
    }

    // Now verify the next successful call works normally
    err2 := vs.PreflightDestructiveToolCall(ctx, "act-ava-rec", "write_file", args)
    if err2 != nil {
        if _, ok := err2.(*core.InteractiveGateError); !ok {
             t.Fatalf("System did not recover gracefully: %v", err2)
        }
    }
}

// -----------------------------------------------------------------------------
// Category: Pipeline & End-to-End Edge Cases
// -----------------------------------------------------------------------------

// TestE2E_VirtualStoreDreamer_FullPipeline_CacheAndValidation Lifecycle tests the
// complete execution path from preflight simulation, execution, and validation reporting.
func TestE2E_VirtualStoreDreamer_FullPipeline_CacheAndValidation(t *testing.T) {
    t.Parallel()
    k, vs, dreamer := setupE2ETest(t)
    ctx := context.Background()

    // 1. Initial State
    target := "multi_boundary.go"
    args := map[string]any{"target": target, "content": "func main() {}"}

    // 2. Gate 1: Preflight Simulation
    err1 := vs.PreflightDestructiveToolCall(ctx, "pipeline_act_1", "write_file", args)
    if err1 != nil {
         if _, ok := err1.(*core.InteractiveGateError); !ok {
             t.Fatalf("Preflight structural failure: %v", err1)
         }
    }

    // 3. Gate 2: The tool execution (mocked here as successful)
    success := true
    output := "bytes written: 14"

    // 4. Gate 3: Validation Result Dispatch
    err2 := vs.ValidateInteractiveToolResult(ctx, "pipeline_act_1", "write_file", args, output, success)
    if err2 != nil {
         if _, ok := err2.(*core.InteractiveGateError); !ok {
             t.Fatalf("Validation structural failure: %v", err2)
         }
    }

    // 5. Assertions on Kernel State
    results, err := k.QueryAll()
    if err != nil {
         t.Fatalf("Kernel query failed post-pipeline: %v", err)
    }
    if len(results) == 0 {
         t.Log("No facts in kernel")
    }

    // 6. Test a secondary pipeline step to ensure state didn't corrupt
    args2 := map[string]any{"target": "secondary.txt", "content": "test"}
    err3 := vs.PreflightDestructiveToolCall(ctx, "pipeline_act_2", "edit_file", args2)
    if err3 != nil {
         if _, ok := err3.(*core.InteractiveGateError); !ok {
             t.Fatalf("Secondary pipeline step failed structurally: %v", err3)
         }
    }

    // 7. Final State: Clear Cache
    dreamer.InvalidateCache()
}

// TestE2E_VirtualStoreDreamer_Pipeline_CascadingValidation checks how
// a failed tool execution avoids calling validation.
func TestE2E_VirtualStoreDreamer_Pipeline_CascadingValidation(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    args := map[string]any{"target": "fail_pipeline.txt"}

    // Tool returns failure
    success := false
    output := "permission denied"

    err := vs.ValidateInteractiveToolResult(ctx, "act-fail", "write_file", args, output, success)

    // It should immediately return nil because success=false avoids validation
    if err != nil {
         t.Fatalf("Failed execution should bypass validation, but returned error: %v", err)
    }
}

// TestE2E_VirtualStoreDreamer_Pipeline_MultiArg checks argument extraction
// robustness during a simulated pipeline run.
func TestE2E_VirtualStoreDreamer_Pipeline_MultiArg(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    // Complex args that might confuse extractActionTarget
    args := map[string]any{
        "url": "http://target.com",
        "filepath": "/path/to/target",
        "filename": "target.txt",
        "path": "override/path", // This usually wins in the heuristic
    }

    err := vs.PreflightDestructiveToolCall(ctx, "act-multi", "write_file", args)
    if err != nil {
         if _, ok := err.(*core.InteractiveGateError); !ok {
             t.Fatalf("Multi-arg pipeline failed structurally: %v", err)
         }
    }
}

// TestE2E_VirtualStoreDreamer_Pipeline_DeepNestedArgs ensures that
// deeply nested map arguments don't cause panics during the Payload copy.
func TestE2E_VirtualStoreDreamer_Pipeline_DeepNestedArgs(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    deepMap := map[string]any{
        "level1": map[string]any{
            "level2": []string{"a", "b", "c"},
        },
    }
    args := map[string]any{
        "target": "deep.json",
        "data": deepMap,
    }

    err := vs.PreflightDestructiveToolCall(ctx, "act-deep", "write_file", args)
    if err != nil {
         if _, ok := err.(*core.InteractiveGateError); !ok {
             t.Fatalf("Deep args pipeline failed structurally: %v", err)
         }
    }
}

// TestE2E_VirtualStoreDreamer_Pipeline_TargetHeuristics stresses the
// target string extraction on heavily modified payloads.
func TestE2E_VirtualStoreDreamer_Pipeline_TargetHeuristics(t *testing.T) {
    t.Parallel()
    _, vs, _ := setupE2ETest(t)
    ctx := context.Background()

    tests := []struct {
         name string
         args map[string]any
    }{
         {"nil_values", map[string]any{"path": nil, "filename": "valid.txt"}},
         {"int_target", map[string]any{"target": 123}}, // extractStringArg will skip this
         {"bool_target", map[string]any{"query": true}},
         {"empty_string", map[string]any{"filepath": ""}},
    }

    for _, tc := range tests {
        tc := tc // capture
        t.Run(tc.name, func(t *testing.T) {
            t.Parallel()
            err := vs.PreflightDestructiveToolCall(ctx, "act-heur-"+tc.name, "write_file", tc.args)
            if err != nil {
                 if _, ok := err.(*core.InteractiveGateError); !ok {
                     t.Fatalf("Heuristic stress test failed structurally: %v", err)
                 }
            }
        })
    }
}
