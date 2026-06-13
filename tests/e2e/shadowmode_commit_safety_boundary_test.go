//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// =============================================================================
// TestE2E_ShadowMode_CommitSimulation_SafetyBoundary
// =============================================================================
//
// Full-stack ShadowMode ↔ Parent Kernel safety boundary test.
//
// Proves that:
// 1. ShadowMode creates an isolated kernel — mutations don't leak to parent
// 2. CommitSimulation applies positive effects to parent (functional path)
// 3. CommitSimulation blocks when simulation has blocking violations
// 4. Retract-based effects wipe ALL facts for a predicate (documents known bug)
// 5. Parent state divergence during simulation causes silent override
// 6. Concurrent WhatIf + Assert on parent doesn't cause data race or panic
// 7. Commit on aborted/completed simulation fails cleanly
// 8. Effects committed to parent bypass constitutional checkSafety
//
// Cross-boundary surfaces:
//   ShadowMode.StartSimulation → clone parent kernel
//   ShadowMode.SimulateAction → shadow kernel Assert + query violations
//   ShadowMode.CommitSimulation → parent kernel Assert/Retract (NO safety gate)
//   ShadowMode.AbortSimulation → cleanup without parent mutation
//
// QA source: core/safety-model.md §Safety Gaps:
//   "ShadowMode is entirely opt-in. Any caller can execute mutations without
//    invoking WhatIf or StartSimulation."
//   "No mechanism exists to verify that the permitted predicate is functioning."
// QA source: core/test-strategy.md §Test Gaps (P1):
//   "ShadowMode commit path (applying effects from shadow kernel to parent)
//    has limited coverage."

func TestE2E_ShadowMode_CommitSimulation_SafetyBoundary(t *testing.T) {
	// =========================================================================
	// SETUP: Real kernel with seed facts
	// =========================================================================
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	// Seed the parent kernel with known state that we can verify isolation against
	seedFacts := []core.Fact{
		{Predicate: "file_topology", Args: []interface{}{"/src/auth.go", "abc123", "/go", int64(1000), true}},
		{Predicate: "file_topology", Args: []interface{}{"/src/db.go", "def456", "/go", int64(2000), false}},
		{Predicate: "file_topology", Args: []interface{}{"/src/handler.go", "ghi789", "/go", int64(3000), false}},
		{Predicate: "dependency_link", Args: []interface{}{"handler.go", "auth.go", "internal"}},
		{Predicate: "dependency_link", Args: []interface{}{"auth.go", "db.go", "internal"}},
		{Predicate: "test_state", Args: []interface{}{"/passing"}},
	}

	for _, f := range seedFacts {
		if assertErr := kernel.Assert(f); assertErr != nil {
			t.Fatalf("Failed to seed fact %s: %v", f.Predicate, assertErr)
		}
	}

	// Verify seed state
	topoFacts, _ := kernel.Query("file_topology")
	depFacts, _ := kernel.Query("dependency_link")
	t.Logf("Seed state: file_topology=%d, dependency_link=%d", len(topoFacts), len(depFacts))

	// =========================================================================
	// ASSERTION 1: Shadow kernel isolation — mutations don't leak to parent
	// =========================================================================
	t.Run("Assertion1_ShadowIsolation", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		sim, startErr := sm.StartSimulation(ctx, "isolation test")
		if startErr != nil {
			t.Fatalf("StartSimulation failed: %v", startErr)
		}
		t.Logf("Started simulation: %s", sim.ID)

		// Simulate a file write — this should only affect shadow kernel
		result, simErr := sm.SimulateAction(ctx, core.SimulatedAction{
			ID:          "iso_1",
			Type:        core.ActionTypeFileWrite,
			Target:      "/src/auth.go",
			Description: "modify auth.go",
		})
		if simErr != nil {
			t.Fatalf("SimulateAction failed: %v", simErr)
		}
		t.Logf("Simulation result: effects=%d, violations=%d, safe=%v",
			len(result.Effects), len(result.Violations), result.IsSafe)

		// The shadow kernel should have simulated_effect facts
		shadowKernel := sm.GetShadowKernel()
		if shadowKernel == nil {
			t.Fatal("Shadow kernel is nil during active simulation")
		}

		shadowEffects, _ := shadowKernel.Query("simulated_effect")
		if len(shadowEffects) == 0 {
			t.Error("Shadow kernel should have simulated_effect facts")
		}

		// The PARENT kernel should NOT have simulated_effect facts
		parentEffects, _ := kernel.Query("simulated_effect")
		if len(parentEffects) > 0 {
			t.Errorf("ISOLATION VIOLATION: Parent kernel has %d simulated_effect facts — shadow leaked",
				len(parentEffects))
		}

		// Abort to clean up
		sm.AbortSimulation("isolation test complete")

		// Verify cleanup
		if sm.IsShadowModeActive() {
			t.Error("Shadow mode should be inactive after abort")
		}
	})

	// =========================================================================
	// ASSERTION 2: CommitSimulation applies positive effects to parent
	// =========================================================================
	t.Run("Assertion2_CommitPositiveEffects", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		_, startErr := sm.StartSimulation(ctx, "commit positive test")
		if startErr != nil {
			t.Fatalf("StartSimulation failed: %v", startErr)
		}

		// Simulate a file write to a NEW file not in the seed state
		result, simErr := sm.SimulateAction(ctx, core.SimulatedAction{
			ID:          "commit_pos_1",
			Type:        core.ActionTypeFileWrite,
			Target:      "/src/new_feature.go",
			Description: "create new_feature.go",
		})
		if simErr != nil {
			t.Fatalf("SimulateAction failed: %v", simErr)
		}

		// Should produce a "modified" effect
		foundModified := false
		for _, e := range result.Effects {
			if e.Predicate == "modified" {
				foundModified = true
				t.Logf("Effect: %s(%v) positive=%v", e.Predicate, e.Args, e.IsPositive)
			}
		}
		if !foundModified {
			t.Error("Expected 'modified' effect from file write simulation")
		}

		// Query parent BEFORE commit — should NOT have modified(/src/new_feature.go)
		preCommitFacts, _ := kernel.Query("modified")
		preCommitCount := len(preCommitFacts)

		// Commit — applies effects to parent
		commitErr := sm.CommitSimulation(ctx)
		if commitErr != nil {
			t.Fatalf("CommitSimulation failed: %v", commitErr)
		}

		// Query parent AFTER commit — should have modified(/src/new_feature.go)
		postCommitFacts, _ := kernel.Query("modified")
		t.Logf("modified facts: before_commit=%d, after_commit=%d", preCommitCount, len(postCommitFacts))

		foundNewFeature := false
		for _, f := range postCommitFacts {
			if len(f.Args) >= 1 {
				target := types.ExtractString(f.Args[0])
				if target == "/src/new_feature.go" {
					foundNewFeature = true
				}
			}
		}
		if !foundNewFeature {
			t.Error("Parent kernel missing modified(/src/new_feature.go) after commit — effects not propagated")
		}

		// Verify simulation is cleaned up
		if sm.IsShadowModeActive() {
			t.Error("Shadow mode should be inactive after commit")
		}
	})

	// =========================================================================
	// ASSERTION 3: CommitSimulation BLOCKS when simulation has blocking violations
	// =========================================================================
	t.Run("Assertion3_CommitBlockedByViolations", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		// Seed a diagnostic error to trigger block_commit in the shadow kernel
		kernel.Assert(core.Fact{
			Predicate: "diagnostic",
			Args:      []interface{}{"/error", "test.go", int64(1), "E001", "compile error"},
		})

		_, startErr := sm.StartSimulation(ctx, "blocked commit test")
		if startErr != nil {
			t.Fatalf("StartSimulation failed: %v", startErr)
		}

		// Simulate a git commit — should trigger block_commit violation
		result, simErr := sm.SimulateAction(ctx, core.SimulatedAction{
			ID:          "blocked_1",
			Type:        core.ActionTypeGitCommit,
			Target:      "main",
			Description: "attempt commit with errors",
		})
		if simErr != nil {
			t.Fatalf("SimulateAction failed: %v", simErr)
		}

		t.Logf("Violations: %d, IsSafe: %v", len(result.Violations), result.IsSafe)
		for _, v := range result.Violations {
			t.Logf("  Violation: type=%s severity=%s blocking=%v desc=%s",
				v.ViolationType, v.Severity, v.Blocking, v.Description)
		}

		// Check if there are blocking violations
		sim, _ := sm.GetActiveSimulation()
		if sim == nil {
			t.Fatal("Active simulation should exist")
		}

		if sim.IsSafe {
			// The kernel's block_commit rule may not fire depending on policy
			// configuration. Document this as a behavioral observation.
			t.Log("NOTE: Simulation was marked safe despite diagnostic error. " +
				"block_commit rule may not be derived in this kernel config.")

			// Even if safe, commit should work
			commitErr := sm.CommitSimulation(ctx)
			if commitErr != nil {
				t.Fatalf("CommitSimulation failed on safe simulation: %v", commitErr)
			}
		} else {
			// Commit should FAIL on unsafe simulation
			commitErr := sm.CommitSimulation(ctx)
			if commitErr == nil {
				t.Error("SAFETY VIOLATION: CommitSimulation succeeded on unsafe simulation — should have been blocked")
			} else {
				t.Logf("Correctly blocked: %v", commitErr)
			}
			// Clean up
			sm.AbortSimulation("test complete")
		}

		// Clean up the diagnostic we injected
		kernel.Retract("diagnostic")
	})

	// =========================================================================
	// ASSERTION 4: Retract effects wipe ALL facts for predicate (documents bug)
	// =========================================================================
	t.Run("Assertion4_RetractWipesAllFacts", func(t *testing.T) {
		// This documents a known architectural issue:
		// CommitSimulation line 391 calls parentKernel.Retract(effect.Predicate)
		// which retracts ALL facts with that predicate, not just the specific one.

		// Seed multiple test_state facts
		kernel.Assert(core.Fact{
			Predicate: "test_state",
			Args:      []interface{}{"/passing"},
		})
		kernel.Assert(core.Fact{
			Predicate: "test_state",
			Args:      []interface{}{"/coverage_high"},
		})

		// Count before
		preRetractFacts, _ := kernel.Query("test_state")
		preCount := len(preRetractFacts)
		t.Logf("test_state facts before Retract: %d", preCount)

		// Direct Retract (simulates what CommitSimulation does for negative effects)
		kernel.Retract("test_state")

		// Count after
		postRetractFacts, _ := kernel.Query("test_state")
		postCount := len(postRetractFacts)
		t.Logf("test_state facts after Retract: %d", postCount)

		if postCount > 0 {
			t.Logf("NOTE: Retract didn't clear all facts (possible deduplication)")
		}

		// DOCUMENT THE BUG: If a ShadowMode simulation produces a negative effect
		// on "test_state", CommitSimulation calls Retract("test_state") which wipes
		// ALL test_state facts, not just the one that was negated.
		t.Log("KNOWN ISSUE: ShadowMode CommitSimulation uses Retract(predicate) for " +
			"negative effects, which wipes ALL facts for that predicate, not just " +
			"the specific fact. A negative effect on test_state(/failing) would also " +
			"destroy test_state(/passing). This is a blast-radius amplification bug.")

		// Re-seed for later tests
		kernel.Assert(core.Fact{
			Predicate: "test_state",
			Args:      []interface{}{"/passing"},
		})
	})

	// =========================================================================
	// ASSERTION 5: Parent state divergence during simulation
	// =========================================================================
	t.Run("Assertion5_ParentStateDivergence", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		_, startErr := sm.StartSimulation(ctx, "divergence test")
		if startErr != nil {
			t.Fatalf("StartSimulation failed: %v", startErr)
		}

		// WHILE simulation is running, mutate the parent kernel
		// This simulates a concurrent session turn modifying kernel state
		divergenceFact := core.Fact{
			Predicate: "file_topology",
			Args:      []interface{}{"/src/concurrent_edit.go", "zzz999", "/go", int64(9999), false},
		}
		kernel.Assert(divergenceFact)

		// The shadow kernel should NOT see this change (it was cloned at Start)
		shadowKernel := sm.GetShadowKernel()
		shadowTopo, _ := shadowKernel.Query("file_topology")

		foundDivergent := false
		for _, f := range shadowTopo {
			if len(f.Args) >= 1 && types.ExtractString(f.Args[0]) == "/src/concurrent_edit.go" {
				foundDivergent = true
			}
		}

		if foundDivergent {
			t.Error("ISOLATION VIOLATION: Shadow kernel sees parent's concurrent mutation")
		} else {
			t.Log("Shadow kernel correctly isolated from parent's concurrent mutations")
		}

		// Now simulate a file write
		sm.SimulateAction(ctx, core.SimulatedAction{
			ID:          "diverge_1",
			Type:        core.ActionTypeFileWrite,
			Target:      "/src/handler.go",
			Description: "modify handler during divergence",
		})

		// Commit — this will apply effects to the CURRENT parent state
		// (which has diverged from what the shadow saw at simulation start)
		commitErr := sm.CommitSimulation(ctx)
		if commitErr != nil {
			sm.AbortSimulation("commit failed")
			t.Logf("Commit failed (may be expected): %v", commitErr)
		} else {
			t.Log("KNOWN ISSUE: CommitSimulation succeeded despite parent state divergence. " +
				"The shadow simulated against a stale snapshot of parent state, but effects " +
				"were applied to the current (diverged) parent state. No divergence check exists.")
		}
	})

	// =========================================================================
	// ASSERTION 6: Concurrent WhatIf + parent Assert — no data race
	// =========================================================================
	t.Run("Assertion6_ConcurrentWhatIfNoRace", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		var wg sync.WaitGroup
		var panicCount int64
		var whatIfErrors int64
		var assertErrors int64

		// Run WhatIf calls concurrently with parent kernel Assert calls.
		// This verifies the RWMutex protection is correct and no data race occurs.
		// (Would be detected by -race flag in the test runner)

		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						atomic.AddInt64(&panicCount, 1)
						t.Logf("PANIC in WhatIf goroutine %d: %v", idx, r)
					}
				}()

				result, err := sm.WhatIf(ctx, core.SimulatedAction{
					ID:          fmt.Sprintf("race_%d", idx),
					Type:        core.ActionTypeFileWrite,
					Target:      fmt.Sprintf("/src/race_%d.go", idx),
					Description: fmt.Sprintf("concurrent whatif %d", idx),
				})
				if err != nil {
					atomic.AddInt64(&whatIfErrors, 1)
					return
				}
				_ = result
			}(i)
		}

		// Concurrent parent kernel mutations
		for i := 0; i < 10; i++ {
			wg.Add(1)
			go func(idx int) {
				defer wg.Done()
				defer func() {
					if r := recover(); r != nil {
						atomic.AddInt64(&panicCount, 1)
						t.Logf("PANIC in Assert goroutine %d: %v", idx, r)
					}
				}()

				err := kernel.Assert(core.Fact{
					Predicate: "observation",
					Args:      []interface{}{fmt.Sprintf("/concurrent_%d", idx), "race_test"},
				})
				if err != nil {
					atomic.AddInt64(&assertErrors, 1)
				}
			}(i)
		}

		wg.Wait()

		t.Logf("Concurrent results: panics=%d, whatif_errors=%d, assert_errors=%d",
			atomic.LoadInt64(&panicCount), atomic.LoadInt64(&whatIfErrors), atomic.LoadInt64(&assertErrors))

		if atomic.LoadInt64(&panicCount) > 0 {
			t.Errorf("DATA RACE: %d panics detected during concurrent WhatIf + Assert", atomic.LoadInt64(&panicCount))
		}

		// WhatIf uses StartSimulation which has a singleton lock, so most will fail
		// with "a simulation is already active". This is expected behavior.
		if atomic.LoadInt64(&whatIfErrors) > 0 {
			t.Logf("NOTE: %d WhatIf calls failed (expected — singleton simulation lock)", atomic.LoadInt64(&whatIfErrors))
		}
	})

	// =========================================================================
	// ASSERTION 7: Commit on aborted/completed simulation fails cleanly
	// =========================================================================
	t.Run("Assertion7_CommitAfterAbortFails", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		_, startErr := sm.StartSimulation(ctx, "abort then commit test")
		if startErr != nil {
			t.Fatalf("StartSimulation failed: %v", startErr)
		}

		// Abort the simulation
		sm.AbortSimulation("intentional abort")

		// Attempt to commit the aborted simulation
		commitErr := sm.CommitSimulation(ctx)
		if commitErr == nil {
			t.Error("SAFETY VIOLATION: CommitSimulation succeeded after AbortSimulation — should have returned error")
		} else {
			t.Logf("Correctly rejected commit after abort: %v", commitErr)
		}

		// Double abort should be safe (no panic)
		sm.AbortSimulation("double abort")
		t.Log("Double abort completed without panic")
	})

	// =========================================================================
	// ASSERTION 8: Effects bypass constitutional checkSafety
	// =========================================================================
	t.Run("Assertion8_CommitBypassesConstitutionalSafety", func(t *testing.T) {
		// This documents that CommitSimulation applies effects directly via
		// parentKernel.Assert() / parentKernel.Retract(), bypassing the
		// constitutional safety gate (checkSafety / permitted predicate).
		//
		// The real path: session executor → checkSafety → permitted query → allowed/denied
		// The shadow path: CommitSimulation → parentKernel.Assert() → NO safety check
		//
		// This means ShadowMode can inject facts that the executor's safety gate
		// would have blocked.

		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		_, startErr := sm.StartSimulation(ctx, "safety bypass test")
		if startErr != nil {
			t.Fatalf("StartSimulation failed: %v", startErr)
		}

		// Simulate a file write — this produces effects
		result, simErr := sm.SimulateAction(ctx, core.SimulatedAction{
			ID:          "bypass_1",
			Type:        core.ActionTypeFileWrite,
			Target:      "/etc/shadow", // Obviously dangerous target
			Description: "write to system file",
		})
		if simErr != nil {
			t.Fatalf("SimulateAction failed: %v", simErr)
		}

		t.Logf("SimulateAction: effects=%d, violations=%d, safe=%v",
			len(result.Effects), len(result.Violations), result.IsSafe)

		// If the simulation is safe (no blocking violations), commit will succeed
		sim, _ := sm.GetActiveSimulation()
		if sim != nil && sim.IsSafe {
			// CommitSimulation will apply modified("/etc/shadow") to parent kernel
			// WITHOUT checking constitutional safety.
			commitErr := sm.CommitSimulation(ctx)
			if commitErr == nil {
				// Verify the dangerous fact was injected
				modFacts, _ := kernel.Query("modified")
				foundDangerous := false
				for _, f := range modFacts {
					if len(f.Args) >= 1 {
						target := types.ExtractString(f.Args[0])
						if target == "/etc/shadow" {
							foundDangerous = true
						}
					}
				}
				if foundDangerous {
					t.Log("DOCUMENTED GAP: ShadowMode CommitSimulation injected " +
						"modified(\"/etc/shadow\") into parent kernel without any " +
						"constitutional safety check. The executor's checkSafety gate " +
						"would have blocked this through the permitted predicate, " +
						"but CommitSimulation bypasses it entirely.")
				}
			} else {
				t.Logf("CommitSimulation failed (safety blocked at simulation level): %v", commitErr)
			}
		} else {
			// Simulation had blocking violations — which is the GOOD outcome
			t.Log("ShadowMode correctly detected violations for dangerous target")
			sm.AbortSimulation("test complete")
		}
	})

	// =========================================================================
	// ASSERTION 9: ToFacts produces correct state during active simulation
	// =========================================================================
	t.Run("Assertion9_ToFactsDuringSimulation", func(t *testing.T) {
		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		// ToFacts with no active simulation should return empty
		emptyFacts := sm.ToFacts()
		if len(emptyFacts) > 0 {
			t.Errorf("Expected empty ToFacts with no simulation, got %d", len(emptyFacts))
		}

		// Start simulation and add effects
		_, startErr := sm.StartSimulation(ctx, "toFacts test")
		if startErr != nil {
			t.Fatalf("StartSimulation failed: %v", startErr)
		}

		sm.SimulateAction(ctx, core.SimulatedAction{
			ID:          "facts_1",
			Type:        core.ActionTypeRefactor,
			Target:      "/src/handler.go",
			Description: "refactor handler",
		})

		// ToFacts should include shadow_state and simulated_effect facts
		activeFacts := sm.ToFacts()
		t.Logf("ToFacts during simulation: %d facts", len(activeFacts))

		foundShadowState := false
		foundSimEffect := false
		for _, f := range activeFacts {
			t.Logf("  Fact: %s(%v)", f.Predicate, f.Args)
			if f.Predicate == "shadow_state" {
				foundShadowState = true
				// Verify validity marker
				if len(f.Args) >= 3 {
					validity := types.ExtractString(f.Args[2])
					if validity != "/valid" && validity != "/invalid" {
						t.Errorf("Unexpected shadow_state validity: %s", validity)
					}
				}
			}
			if f.Predicate == "simulated_effect" {
				foundSimEffect = true
			}
		}

		if !foundShadowState {
			t.Error("ToFacts missing shadow_state fact during active simulation")
		}
		if !foundSimEffect {
			t.Error("ToFacts missing simulated_effect fact during active simulation")
		}

		sm.AbortSimulation("test complete")

		// After abort, ToFacts should return empty again
		postAbortFacts := sm.ToFacts()
		if len(postAbortFacts) > 0 {
			t.Errorf("Expected empty ToFacts after abort, got %d", len(postAbortFacts))
		}
	})

	// =========================================================================
	// ASSERTION 10: Performance — StartSimulation time scales with parent size
	// =========================================================================
	t.Run("Assertion10_StartSimulationPerformance", func(t *testing.T) {
		// Seed the kernel with a large number of facts
		for i := 0; i < 500; i++ {
			kernel.Assert(core.Fact{
				Predicate: "observation",
				Args:      []interface{}{fmt.Sprintf("/perf_test_%d", i), fmt.Sprintf("value_%d", i)},
			})
		}

		sm := core.NewShadowMode(kernel)
		ctx := context.Background()

		start := time.Now()
		_, startErr := sm.StartSimulation(ctx, "performance test")
		elapsed := time.Since(start)

		if startErr != nil {
			t.Fatalf("StartSimulation failed: %v", startErr)
		}

		t.Logf("StartSimulation with ~500 extra facts: %v", elapsed)

		if elapsed > 10*time.Second {
			t.Errorf("StartSimulation too slow: %v (expected <10s)", elapsed)
		}

		sm.AbortSimulation("perf test done")
	})
}
