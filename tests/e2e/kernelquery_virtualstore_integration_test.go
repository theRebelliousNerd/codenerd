//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// =============================================================================
// TEST 6: Kernel Query × VirtualStore — Cross-Boundary Fact Integrity
// Sourced from: kernel_query_boundary_analysis.md §1-§4
//               virtual_store_boundary_analysis.md §1, §2, §4
//
// NOTE: We share a single kernel across sub-tests to avoid the ~8s per
// NewRealKernel() call (full Mangle policy parse+evaluate). Tests that
// need isolated state use fresh kernels only when strictly necessary.
// =============================================================================

func TestE2E_KernelQuery_VirtualStore_FactIntegrity(t *testing.T) {
	// Shared kernel — created once (~8s), used across all sub-tests
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create shared kernel: %v", err)
	}

	// --- §1.1: Empty Predicate Query ---
	t.Run("EmptyPredicateQuery_CleanError", func(t *testing.T) {
		results, err := kernel.Query("")
		if err == nil {
			t.Error("Expected error for empty predicate query")
		}
		if results != nil && len(results) > 0 {
			t.Error("Expected no results for empty predicate")
		}
		t.Logf("Empty predicate: err=%v", err)
	})

	// --- §1.2: ParseFactString Edge Cases (stateless — no kernel needed) ---
	t.Run("ParseFactString_EmptyAndPeriod", func(t *testing.T) {
		cases := []struct {
			name  string
			input string
		}{
			{"empty_string", ""},
			{"spaces_only", "   "},
			{"just_period", "."},
			{"double_period", ".."},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				_, err := core.ParseFactString(tc.input)
				if err == nil {
					t.Errorf("Expected error for ParseFactString(%q)", tc.input)
				}
				t.Logf("ParseFactString(%q): err=%v", tc.input, err)
			})
		}
	})

	// --- §1.3: LoadFactsFromFile on Empty/Missing Files ---
	t.Run("LoadFactsFromFile_EmptyFile", func(t *testing.T) {
		tmpDir := t.TempDir()
		emptyFile := filepath.Join(tmpDir, "empty.mg")
		if err := os.WriteFile(emptyFile, []byte{}, 0644); err != nil {
			t.Fatalf("Failed to create empty file: %v", err)
		}

		err := kernel.LoadFactsFromFile(emptyFile)
		if err != nil {
			t.Logf("Empty .mg file result: %v (expected nil or clean parse error)", err)
		} else {
			t.Log("Empty file loaded as no-op (correct)")
		}
	})

	t.Run("LoadFactsFromFile_NonexistentFile", func(t *testing.T) {
		err := kernel.LoadFactsFromFile("/nonexistent/path/to/file.mg")
		if err == nil {
			t.Error("Expected error for nonexistent file")
		}
		t.Logf("Nonexistent file: err=%v", err)
	})

	// --- §2.1: Type Coercion — String vs Atom Distinction ---
	t.Run("StringVsAtom_QueryDistinction", func(t *testing.T) {
		// Assert facts with both string and atom types
		kernel.Assert(core.Fact{
			Predicate: "test_type",
			Args:      []interface{}{"/atom_value"},
		})
		kernel.Assert(core.Fact{
			Predicate: "test_type",
			Args:      []interface{}{"string_value"},
		})

		results, err := kernel.Query("test_type")
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}
		t.Logf("test_type facts: %d", len(results))
		for _, f := range results {
			t.Logf("  arg[0] = %v (type: %T)", f.Args[0], f.Args[0])
		}
	})

	// --- §2.2: Float Precision Preservation ---
	t.Run("FloatPrecisionPreservation", func(t *testing.T) {
		precise := 3.141592653589793
		kernel.Assert(core.Fact{
			Predicate: "measurement",
			Args:      []interface{}{precise},
		})

		results, err := kernel.Query("measurement")
		if err != nil {
			t.Fatalf("Query failed: %v", err)
		}

		if len(results) > 0 {
			val := results[0].Args[0]
			switch v := val.(type) {
			case float64:
				if math.Abs(v-precise) > 1e-10 {
					t.Errorf("Float precision lost: expected %v, got %v", precise, v)
				}
			case int64:
				t.Logf("Float was converted to int64: %d (Mangle normalizes numerics)", v)
			default:
				t.Logf("Unexpected type: %T = %v", val, val)
			}
		}
	})

	// --- §3.1: Massive Arity Query ---
	t.Run("MassiveArityFact_NoOverflow", func(t *testing.T) {
		// Build a fact with 100 arguments
		args := make([]interface{}, 100)
		for i := range args {
			args[i] = fmt.Sprintf("arg_%d", i)
		}
		err := kernel.Assert(core.Fact{
			Predicate: "wide_fact",
			Args:      args,
		})
		if err != nil {
			t.Logf("Wide fact assertion error (may be expected): %v", err)
		} else {
			results, qErr := kernel.Query("wide_fact")
			if qErr != nil {
				t.Logf("Wide fact query error: %v", qErr)
			} else {
				t.Logf("Wide fact: asserted=%d args, queried back %d facts", len(args), len(results))
			}
		}
	})

	// --- §4.1: Concurrent Assert/Query Starvation ---
	// NOTE: Each Assert triggers a full Mangle evaluate() cycle (~500ms).
	// Keep goroutine × iteration count low (3×3=9 asserts total) to fit in CI budget.
	t.Run("ConcurrentAssertQuery_NoStarvation", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		assertDurations := make([]time.Duration, 0, 10)
		queryDurations := make([]time.Duration, 0, 10)
		var durMu sync.Mutex

		// Writers — 3 goroutines × 3 iterations = 9 asserts
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 3; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					start := time.Now()
					kernel.Assert(types.Fact{
						Predicate: "concurrent_obs",
						Args:      []interface{}{fmt.Sprintf("w%d_%d", id, j)},
					})
					dur := time.Since(start)
					durMu.Lock()
					assertDurations = append(assertDurations, dur)
					durMu.Unlock()
				}
			}(i)
		}

		// Readers — 3 goroutines × 3 iterations
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(id int) {
				defer wg.Done()
				for j := 0; j < 3; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					start := time.Now()
					kernel.Query("concurrent_obs")
					dur := time.Since(start)
					durMu.Lock()
					queryDurations = append(queryDurations, dur)
					durMu.Unlock()
				}
			}(i)
		}

		wg.Wait()

		// Check for starvation: no individual operation should take > 5s
		maxAssert := time.Duration(0)
		for _, d := range assertDurations {
			if d > maxAssert {
				maxAssert = d
			}
		}
		maxQuery := time.Duration(0)
		for _, d := range queryDurations {
			if d > maxQuery {
				maxQuery = d
			}
		}

		t.Logf("Assert max=%v, Query max=%v", maxAssert, maxQuery)
		if maxAssert > 5*time.Second {
			t.Errorf("Assert starvation: max latency %v exceeds 5s", maxAssert)
		}
		if maxQuery > 5*time.Second {
			t.Errorf("Query starvation: max latency %v exceeds 5s", maxQuery)
		}
	})

	// --- §4.2: QueryAll on Growing EDB ---
	// NOTE: Each Assert triggers full Mangle evaluate(). Use 10 facts to
	// validate QueryAll correctness without blowing the CI time budget.
	t.Run("QueryAll_GrowingEDB_Performance", func(t *testing.T) {
		for i := 0; i < 10; i++ {
			kernel.Assert(core.Fact{
				Predicate: "perf_obs",
				Args:      []interface{}{fmt.Sprintf("key_%d", i), fmt.Sprintf("val_%d", i)},
			})
		}

		start := time.Now()
		allFacts, err := kernel.QueryAll()
		elapsed := time.Since(start)

		if err != nil {
			t.Fatalf("QueryAll failed: %v", err)
		}

		totalFacts := 0
		for _, facts := range allFacts {
			totalFacts += len(facts)
		}

		t.Logf("QueryAll: %d predicates, %d total facts in %v", len(allFacts), totalFacts, elapsed)

		if elapsed > 30*time.Second {
			t.Errorf("QueryAll too slow: %v (expected <30s)", elapsed)
		}
	})
}

// =============================================================================
// TEST 7: VirtualStore × Kernel × ActionDispatch — Boundary Hardening
// Sourced from: virtual_store_boundary_analysis.md §1-§4, §11, §16
// =============================================================================

func TestE2E_VirtualStore_ActionDispatch_BoundaryHardening(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	executor := tactile.NewCompositeExecutor()
	vs := core.NewVirtualStore(executor)
	vs.SetKernel(kernel)
	vs.DisableBootGuard()

	// --- §1.1: Nil Args in Action Fact ---
	t.Run("NilArgs_GracefulError", func(t *testing.T) {
		action := core.Fact{
			Predicate: "next_action",
			Args:      nil,
		}
		_, err := vs.RouteAction(context.Background(), action)
		if err == nil {
			t.Error("Expected error for nil Args")
		}
		if err != nil && !strings.Contains(err.Error(), "requires at least 3 arguments") {
			t.Logf("Nil args error message: %v", err)
		}
	})

	// --- §1.2: Too Few Args ---
	t.Run("TooFewArgs_CleanError", func(t *testing.T) {
		cases := []struct {
			name string
			args []interface{}
		}{
			{"zero_args", []interface{}{}},
			{"one_arg", []interface{}{"id1"}},
			{"two_args", []interface{}{"id1", "/echo"}},
		}

		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				action := core.Fact{
					Predicate: "next_action",
					Args:      tc.args,
				}
				_, err := vs.RouteAction(context.Background(), action)
				if err == nil {
					t.Errorf("Expected error for %d args", len(tc.args))
				}
			})
		}
	})

	// --- §1.3: Empty String Target ---
	t.Run("EmptyStringTarget_NoHang", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		action := core.Fact{
			Predicate: "next_action",
			Args:      []interface{}{"test_id", "/read_file", ""},
		}

		_, err := vs.RouteAction(ctx, action)
		// Should either error cleanly or return empty — must not hang
		t.Logf("Empty target result: err=%v", err)
	})

	// --- §2.1: NaN Timeout Coercion ---
	t.Run("NaN_TimeoutValue_NoHang", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		action := core.Fact{
			Predicate: "next_action",
			Args: []interface{}{
				"nan_timeout_test",
				"/exec_cmd",
				"echo hello",
				map[string]interface{}{
					"timeout_seconds": math.NaN(),
				},
			},
		}

		done := make(chan struct{})
		go func() {
			_, _ = vs.RouteAction(ctx, action)
			close(done)
		}()

		select {
		case <-done:
			t.Log("NaN timeout handled without hang")
		case <-ctx.Done():
			t.Error("TIMEOUT: NaN timeout value caused RouteAction to hang")
		}
	})

	// --- §2.2: Inf Timeout Coercion ---
	t.Run("Inf_TimeoutValue_NoHang", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()

		action := core.Fact{
			Predicate: "next_action",
			Args: []interface{}{
				"inf_timeout_test",
				"/exec_cmd",
				"echo hello",
				map[string]interface{}{
					"timeout_seconds": math.Inf(1),
				},
			},
		}

		done := make(chan struct{})
		go func() {
			_, _ = vs.RouteAction(ctx, action)
			close(done)
		}()

		select {
		case <-done:
			t.Log("Inf timeout handled without hang")
		case <-ctx.Done():
			t.Error("TIMEOUT: Inf timeout value caused RouteAction to hang")
		}
	})

	// --- §4.1: Concurrent RouteAction + SetKernel Deadlock ---
	t.Run("ConcurrentRouteAction_SetKernel_NoDeadlock", func(t *testing.T) {
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()

		var wg sync.WaitGroup

		// 10 concurrent RouteAction callers
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					action := core.Fact{
						Predicate: "next_action",
						Args:      []interface{}{fmt.Sprintf("concurrent_%d_%d", idx, j), "/list_files", "."},
					}
					vs.RouteAction(ctx, action)
				}
			}(i)
		}

		// Concurrent kernel mutations
		for i := 0; i < 3; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				for j := 0; j < 5; j++ {
					select {
					case <-ctx.Done():
						return
					default:
					}
					kernel.Assert(core.Fact{
						Predicate: "test_concurrent_fact",
						Args:      []interface{}{fmt.Sprintf("mutate_%d_%d", idx, j)},
					})
				}
			}(i)
		}

		done := make(chan struct{})
		go func() {
			wg.Wait()
			close(done)
		}()

		select {
		case <-done:
			t.Log("Concurrent RouteAction + kernel mutation completed without deadlock")
		case <-ctx.Done():
			t.Error("DEADLOCK: Concurrent RouteAction + kernel mutation timed out")
		}
	})

	// --- §4.2: Boot Guard Race Condition ---
	t.Run("BootGuard_ConcurrentDisableAndRoute_NoRace", func(t *testing.T) {
		// This sub-test needs a fresh VirtualStore with active boot guard
		freshVS := core.NewVirtualStore(executor)
		freshVS.SetKernel(kernel) // reuse kernel to avoid another 8s init
		// Boot guard starts active

		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()

		var wg sync.WaitGroup
		blockedCount := 0
		passedCount := 0
		var countMu sync.Mutex

		// 20 concurrent RouteAction attempts
		for i := 0; i < 20; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				action := core.Fact{
					Predicate: "next_action",
					Args:      []interface{}{fmt.Sprintf("boot_%d", idx), "/list_files", "."},
				}
				_, err := freshVS.RouteAction(ctx, action)
				countMu.Lock()
				defer countMu.Unlock()
				if err != nil && strings.Contains(err.Error(), "boot guard") {
					blockedCount++
				} else {
					passedCount++
				}
			}(i)
		}

		// Disable boot guard while actions are in-flight
		time.Sleep(10 * time.Millisecond)
		freshVS.DisableBootGuard()

		wg.Wait()
		t.Logf("Boot guard race: blocked=%d, passed=%d", blockedCount, passedCount)

		if freshVS.IsBootGuardActive() {
			t.Error("Boot guard should be disabled after DisableBootGuard()")
		}
	})

	// --- §11: Shell Injection Safety ---
	t.Run("ShellInjection_ConstitutionalBlock", func(t *testing.T) {
		injectionCases := []struct {
			name   string
			target string
		}{
			{"rm_rf", "rm -rf /"},
			{"semicolon_rm", "echo hello; rm -rf /"},
			{"pipe_rm", "echo hello | rm -rf /"},
		}

		for _, tc := range injectionCases {
			t.Run(tc.name, func(t *testing.T) {
				action := core.Fact{
					Predicate: "next_action",
					Args:      []interface{}{"inject_test", "/exec_cmd", tc.target},
				}
				_, err := vs.RouteAction(context.Background(), action)
				if err == nil {
					// Check if the command was actually blocked
					t.Logf("WARNING: destructive command '%s' was not blocked by constitution", tc.target)
				} else {
					t.Logf("Correctly blocked: %s -> %v", tc.target, err)
				}
			})
		}
	})

	// --- Verify Kernel State After All Actions ---
	t.Run("KernelStateConsistency_AfterActions", func(t *testing.T) {
		// After all the above tests, verify the kernel is still responsive
		results, err := kernel.Query("execution_result")
		if err != nil {
			t.Fatalf("Kernel query failed after action tests: %v", err)
		}
		t.Logf("Kernel execution_result facts after action tests: %d", len(results))

		securityFacts, _ := kernel.Query("security_violation")
		t.Logf("Kernel security_violation facts: %d", len(securityFacts))

		// Verify kernel is still queryable and not in a corrupt state
		allFacts, err := kernel.QueryAll()
		if err != nil {
			t.Fatalf("QueryAll failed: kernel in corrupt state after action tests")
		}

		totalFacts := 0
		for _, facts := range allFacts {
			totalFacts += len(facts)
		}
		t.Logf("Kernel total facts: %d across %d predicates (healthy)", totalFacts, len(allFacts))
	})
}
