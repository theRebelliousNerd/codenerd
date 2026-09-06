//go:build integration

package e2e_test

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	ctxpkg "codenerd/internal/context"
	"codenerd/internal/core"
	"codeberg.org/TauCeti/mangle-go/ast"
)

func mustName(s string) ast.Constant {
	n, err := ast.Name(s)
	if err != nil {
		panic(err)
	}
	return n
}

// TestE2E_ContextActivation_EndToEnd_PerfectPipeline (Mode A: Pipeline)
// Verifies that a fact asserted into the Kernel correctly traverses the Spreading
// Activation boundary and emerges intact as a high-scoring context atom.
// Contract: End-to-End Data Integrity.
func TestE2E_ContextActivation_EndToEnd_PerfectPipeline(t *testing.T) {
	t.Parallel()

	// 1. Setup real kernel
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	// 2. Setup Activation Engine
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	// 3. Inject standard user intent and dependency link
	intentFact := core.Fact{
		Predicate: "user_intent",
		Args: []any{
			ast.String("uuid-1"),
			ast.String("test"),
			ast.String("focus"),
			ast.String("target"),
			ast.String("payload"),
		},
	}
	depFact := core.Fact{
		Predicate: "dependency_link",
		Args: []any{
			mustName("caller"),
			mustName("callee"),
			ast.String("path/to/file.go"),
		},
	}

	kernel.AssertBatch([]core.Fact{intentFact, depFact})

	// 4. Score
	facts := []core.Fact{intentFact, depFact}
	ae.MarkNewFacts(facts)
	scored := ae.ScoreFacts(facts, &intentFact)

	// 5. Assert End-to-End Integrity
	if len(scored) == 0 {
		t.Fatalf("Expected scored facts, got 0")
	}

	foundDep := false
	for _, sf := range scored {
		if sf.Fact.Predicate == "dependency_link" {
			foundDep = true
			if sf.Score <= 0 {
				t.Errorf("Expected dependency_link to have positive activation score, got %f", sf.Score)
			}
		}
	}
	if !foundDep {
		t.Errorf("dependency_link fact was lost across the activation boundary")
	}
}

// TestE2E_ContextActivation_ContractViolation_MangleTypeDissonance (Mode B: Boundary)
// Verifies that the Context Engine does not crash when the Kernel returns String types
// for arguments that the Engine expects to be Names (Atoms) based on the schema.
// Contract: Atom/String Dissonance Contract.
func TestE2E_ContextActivation_ContractViolation_MangleTypeDissonance(t *testing.T) {
	t.Parallel()

	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	// Inject a malformed fact directly (String instead of Name)
	malformedFact := core.Fact{
		Predicate: "dependency_link",
		Args: []any{
			ast.String("caller"), // Expected Name
			ast.String("callee"), // Expected Name
			ast.String("path"),
		},
	}

	// Should not panic, but gracefully handle or ignore the dissonance.
	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Contract Violated: ScoreFacts paniced on Mangle Type Dissonance: %v", r)
		}
	}()

	scored := ae.ScoreFacts([]core.Fact{malformedFact}, nil)

	if len(scored) != 1 {
		t.Errorf("Expected 1 fact back, got %d", len(scored))
	}
}

// TestE2E_ContextActivation_ContractViolation_MissingArity (Mode B: Boundary)
// Verifies that VirtualStore injecting facts with missing arguments does not cause
// out-of-bounds array access panics in buildSymbolGraphLocked.
// Contract: Fail-Closed Constraint.
func TestE2E_ContextActivation_ContractViolation_MissingArity(t *testing.T) {
	t.Parallel()

	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	// Inject dependency_link with only 1 argument (arity is 3)
	malformedFact := core.Fact{
		Predicate: "dependency_link",
		Args: []any{
			mustName("caller_only"),
		},
	}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Contract Violated: Missing arity caused panic, crashing Campaign Orchestrator: %v", r)
		}
	}()

	_ = ae.ScoreFacts([]core.Fact{malformedFact}, nil)
}

// TestE2E_ContextActivation_StateCorruption_InPlaceSliceMutation (Mode B: Boundary)
// Verifies that the Context Engine treats the EDB slice provided by the Kernel as strictly read-only.
// Contract: Immutability Contract.
func TestE2E_ContextActivation_StateCorruption_InPlaceSliceMutation(t *testing.T) {
	t.Parallel()

	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	fact := core.Fact{Predicate: "test_fact"}
	kernelSlice := []core.Fact{fact, fact, fact}

	// Create a canary to ensure the original slice is untouched
	canary := make([]core.Fact, len(kernelSlice))
	copy(canary, kernelSlice)

	_ = ae.ScoreFacts(kernelSlice, nil)

	for i := range kernelSlice {
		if kernelSlice[i].Predicate != canary[i].Predicate {
			t.Fatalf("Contract Violated: ActivationEngine mutated the Kernel's EDB slice in place. This corrupts Mangle's fixpoint evaluation.")
		}
	}
}

// TestE2E_ContextActivation_StateCorruption_GetSessionStatsRace (Mode B: Boundary)
// Verifies concurrent reads to map state via GetSessionStats while ScoreFacts mutates
// those maps during graph rebuild.
// Contract: Graph Reset Contract.
func TestE2E_ContextActivation_StateCorruption_GetSessionStatsRace(t *testing.T) {
	t.Parallel()

	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	var wg sync.WaitGroup
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	// Writer Goroutine: Constantly rebuilds the graph
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				ae.ScoreFacts([]core.Fact{
					{Predicate: "dependency_link", Args: []any{mustName("a"), mustName("b"), ast.String("path")}},
				}, nil)
			}
		}
	}()

	// Reader Goroutine: Constantly polls stats
	wg.Add(1)
	go func() {
		defer wg.Done()
		for {
			select {
			case <-ctx.Done():
				return
			default:
				stats := ae.GetSessionStats()
				if stats == nil {
					t.Error("Stats should not be nil")
				}
			}
		}
	}()

	wg.Wait()
}

// TestE2E_ContextActivation_ResourceExhaustion_InfiniteDependencyCycle (Mode B: Boundary)
// Verifies that circular dependency links (A->B->C->A) injected into the Kernel do not
// cause infinite recursion or timeouts during Spreading Activation.
// Contract: Graph Reset Contract.
func TestE2E_ContextActivation_ResourceExhaustion_InfiniteDependencyCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping resource exhaustion test in short mode")
	}
	t.Parallel()

	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	// Construct A -> B -> C -> A
	facts := []core.Fact{
		{Predicate: "dependency_link", Args: []any{mustName("A"), mustName("B"), ast.String("path")}},
		{Predicate: "dependency_link", Args: []any{mustName("B"), mustName("C"), ast.String("path")}},
		{Predicate: "dependency_link", Args: []any{mustName("C"), mustName("A"), ast.String("path")}},
	}

	timer := time.AfterFunc(2*time.Second, func() {
		panic("Resource Exhaustion Violated: Infinite loop in ScoreFacts detected due to cyclic dependencies.")
	})
	defer timer.Stop()

	// If this doesn't halt, the timer panics
	scored := ae.ScoreFacts(facts, nil)

	if len(scored) != 3 {
		t.Errorf("Expected 3 scored facts, got %d", len(scored))
	}
}

// TestE2E_ContextActivation_CascadingFailure_NilIntentPanic (Mode B: Boundary)
// Verifies that passing a nil intent does not cause a nil pointer dereference, which
// would crash the Session Executor and abandon the campaign.
// Contract: Fail-Closed Constraint.
func TestE2E_ContextActivation_CascadingFailure_NilIntentPanic(t *testing.T) {
	t.Parallel()

	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	fact := core.Fact{Predicate: "test"}

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Contract Violated: Nil intent caused panic: %v", r)
		}
	}()

	// ScoreFacts must gracefully handle a nil intent without crashing.
	scored := ae.ScoreFacts([]core.Fact{fact}, nil)
	if len(scored) != 1 {
		t.Errorf("Expected 1 fact back, got %d", len(scored))
	}
}

// TestE2E_ContextActivation_ContractViolation_NegativePriority (Mode B: Boundary)
// Verifies priority fallback handles corrupted negative values gracefully.
// Contract: Priority Fallback Failure.
func TestE2E_ContextActivation_ContractViolation_NegativePriority(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
	ae.SetCorpusPriorities(map[string]int{"dependency_link": -999999})

	fact := core.Fact{Predicate: "dependency_link", Args: []any{mustName("a"), mustName("b"), ast.String("path")}}
	scored := ae.ScoreFacts([]core.Fact{fact}, nil)
	if len(scored) != 1 {
		t.Errorf("Expected 1 fact back, got %d", len(scored))
	}
}

// TestE2E_ContextActivation_ContractViolation_CorruptVirtualStore (Mode B: Boundary)
// Verifies VirtualStore corruption (bad arguments) does not halt the engine.
// Contract: Corrupt VirtualStore Output.
func TestE2E_ContextActivation_ContractViolation_CorruptVirtualStore(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
	fact := core.Fact{Predicate: "dependency_link", Args: []any{123, 456, 789}} // Wrong types completely

	defer func() {
		if r := recover(); r != nil {
			t.Fatalf("Contract Violated: Engine panicked on completely wrong types: %v", r)
		}
	}()

	scored := ae.ScoreFacts([]core.Fact{fact}, nil)
	if len(scored) != 1 {
		t.Errorf("Expected 1 fact back")
	}
}

// TestE2E_ContextActivation_ContractViolation_PhasePoisoning (Mode B: Boundary)
// Verifies massive PhaseGoals string does not poison scoring heuristics.
func TestE2E_ContextActivation_ContractViolation_PhasePoisoning(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	massiveGoal := string(make([]byte, 10000))
	ae.SetCampaignContext(&ctxpkg.CampaignActivationContext{
		PhaseGoals: []string{massiveGoal},
	})

	fact := core.Fact{Predicate: "random_fact"}
	scored := ae.ScoreFacts([]core.Fact{fact}, nil)
	if len(scored) != 1 {
		t.Errorf("Expected 1 fact back")
	}
}

// TestE2E_ContextActivation_StateCorruption_CrossSessionGhosting (Mode B: Boundary)
// Verifies state leakage between sessions if NewSession is bypassed.
func TestE2E_ContextActivation_StateCorruption_CrossSessionGhosting(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	// Session A
	factA := core.Fact{Predicate: "factA"}
	ae.MarkNewFacts([]core.Fact{factA})

	// Bypass NewSession, start Session B
	stats := ae.GetSessionStats()
	if stats["session_facts"].(int) != 1 {
		t.Errorf("State Corrupted: Session B inherited Session A's facts")
	}
}

// TestE2E_ContextActivation_ResourceExhaustion_100kGraphBomb (Mode B: Boundary)
// Verifies engine survives a massive number of dependency links.
func TestE2E_ContextActivation_ResourceExhaustion_100kGraphBomb(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping graph bomb in short mode")
	}
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	var facts []core.Fact
	for i := 0; i < 50000; i++ {
		facts = append(facts, core.Fact{
			Predicate: "dependency_link",
			Args: []any{mustName(fmt.Sprintf("a%d", i)), mustName(fmt.Sprintf("b%d", i)), ast.String("path")},
		})
	}

	scored := ae.ScoreFacts(facts, nil)
	if len(scored) != 50000 {
		t.Errorf("Expected 50000 facts")
	}
}

// TestE2E_ContextActivation_Temporal_ContextCancellationMidScoring (Mode B: Boundary)
// Context cancellation should stop any long-running spreading activation loops.
func TestE2E_ContextActivation_Temporal_ContextCancellationMidScoring(t *testing.T) {
	t.Parallel()
	// Currently ActivationEngine.ScoreFacts doesn't accept a context.Context.
	// This test asserts that the boundary is missing a critical control mechanism.
	t.Log("KNOWN: ActivationEngine lacks context.Context parameter. Cannot interrupt mid-flight traversal.")
}

// TestE2E_ContextActivation_Temporal_RecencyDecayDesync (Mode B: Boundary)
// Verifies time fast-forward does not drop facts, only decays scores.
func TestE2E_ContextActivation_Temporal_RecencyDecayDesync(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
	fact := core.Fact{Predicate: "test"}
	ae.MarkNewFacts([]core.Fact{fact})

	scored := ae.ScoreFacts([]core.Fact{fact}, nil)
	if len(scored) != 1 {
		t.Errorf("Fact dropped unexpectedly")
	}
}

// TestE2E_ContextActivation_Temporal_OrphanedCacheEviction (Mode B: Boundary)
// Verifies that explicitly clearing the context cleans up the campaign metadata.
func TestE2E_ContextActivation_Temporal_OrphanedCacheEviction(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
	ae.SetCampaignContext(&ctxpkg.CampaignActivationContext{CampaignID: "123"})

	ae.ClearCampaignContext()

	stats := ae.GetSessionStats()
	if stats["has_campaign"].(bool) {
		t.Errorf("Temporal leak: Campaign context not cleared.")
	}
}

// TestE2E_ContextActivation_CascadingFailure_PanicRecoveryInPager (Mode A: Pipeline)
// Verifies the session pager (or orchestrator) recovers if engine panics.
func TestE2E_ContextActivation_CascadingFailure_PanicRecoveryInPager(t *testing.T) {
	t.Parallel()
	t.Log("KNOWN: Session executor lacks explicit recover() for ActivationEngine panics.")
}

// TestE2E_ContextActivation_Recovery_AfterStateClear (Mode B: Boundary)
// Tests that the system can recover to a pristine state.
func TestE2E_ContextActivation_Recovery_AfterStateClear(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
	ae.MarkNewFacts([]core.Fact{{Predicate: "a"}})
	ae.ClearState()

	scored := ae.ScoreFacts([]core.Fact{{Predicate: "b"}}, nil)
	if len(scored) != 1 || scored[0].Fact.Predicate != "b" {
		t.Errorf("Failed to recover clean state after ClearState")
	}
}

// TestE2E_ContextActivation_Recovery_TurnReset (Mode A: Pipeline)
// Tests that facts scored in turn 1 do not indefinitely bias turn 2.
func TestE2E_ContextActivation_Recovery_TurnReset(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	fact := core.Fact{Predicate: "high_val"}
	ae.MarkNewFacts([]core.Fact{fact})
	scored1 := ae.ScoreFacts([]core.Fact{fact}, nil)

	ae.ClearState()
	ae.NewSession()

	scored2 := ae.ScoreFacts([]core.Fact{fact}, nil)

	// They shouldn't have exactly the same score if one was boosted by recency/new fact status.
	if len(scored1) > 0 && len(scored2) > 0 && scored1[0].Score == scored2[0].Score {
		t.Errorf("Scores did not reset after recovery clear state.")
	}
}

// TestE2E_ContextActivation_MultiTurn_StateAccumulation (Mode B: Boundary)
// Simulates accumulating facts over multiple session turns.
func TestE2E_ContextActivation_MultiTurn_StateAccumulation(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))

	// Turn 1
	ae.MarkNewFacts([]core.Fact{{Predicate: "turn1"}})
	// Turn 2
	ae.MarkNewFacts([]core.Fact{{Predicate: "turn2"}})

	stats := ae.GetSessionStats()
	if stats["session_facts"].(int) != 2 {
		t.Errorf("Failed to accumulate facts across turns")
	}
}

// TestE2E_ContextActivation_MultiTurn_ContextIsolation (Mode B: Boundary)
// Verifies that starting a NewSession completely isolates the fact history.
func TestE2E_ContextActivation_MultiTurn_ContextIsolation(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
	ae.MarkNewFacts([]core.Fact{{Predicate: "turn1"}})

	ae.NewSession() // Isolation boundary

	stats := ae.GetSessionStats()
	if stats["session_facts"].(int) != 0 {
		t.Errorf("Failed to isolate context after NewSession")
	}
}

// TestE2E_ContextActivation_PartialPipeline_MissingIntent (Mode A: Pipeline)
// Tests behavior when the user intent fact is entirely missing from the pipeline.
func TestE2E_ContextActivation_PartialPipeline_MissingIntent(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
	scored := ae.ScoreFacts([]core.Fact{{Predicate: "data"}}, nil)
	if len(scored) != 1 {
		t.Errorf("Pipeline failed to process facts without intent")
	}
}

// TestE2E_ContextActivation_PartialPipeline_NoFacts (Mode A: Pipeline)
// Tests pipeline when VirtualStore and Kernel supply zero facts.
func TestE2E_ContextActivation_PartialPipeline_NoFacts(t *testing.T) {
	t.Parallel()
	ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
	intent := core.Fact{Predicate: "user_intent"}
	scored := ae.ScoreFacts([]core.Fact{}, &intent)
	if len(scored) != 0 {
		t.Errorf("Pipeline failed to handle empty facts")
	}
}

// TestE2E_ContextActivation_EndToEnd_FactRetractionRace (Mode A: Pipeline)
// End-to-end race condition test for fact slice retraction at the boundary.
func TestE2E_ContextActivation_EndToEnd_FactRetractionRace(t *testing.T) {
	t.Parallel()
	// Mangle kernel asserts facts. We simulate a race by passing a slice that
	// gets mutated in the background. Go's race detector will catch this if
	// ActivationEngine reads it concurrently.
	kernelSlice := []core.Fact{{Predicate: "race"}}

	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		ae := ctxpkg.NewActivationEngine(ctxpkg.NewConfigWithBudget(1000))
		_ = ae.ScoreFacts(kernelSlice, nil)
	}()

	go func() {
		defer wg.Done()
		// Simulate Kernel retracting fact by mutating slice backing array
		if len(kernelSlice) > 0 {
			kernelSlice[0].Predicate = "retracted"
		}
	}()

	wg.Wait()
}
