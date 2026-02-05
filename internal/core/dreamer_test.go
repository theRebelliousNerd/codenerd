package core

import (
	"context"
	"testing"
)

// setupTestDreamer initializes a Dreamer with a real kernel (embedded defaults).
// This relies on the kernel being able to boot from embedded defaults.
func setupTestDreamer(t *testing.T) (*Dreamer, *RealKernel) {
	// Initialize real kernel with defaults (embedded)
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	d := NewDreamer(k)
	return d, k
}

// TODO: TEST_GAP: Performance/OOM - Missing test for massive code graph (100k+ facts).
// The current implementation of codeGraphProjections performs full table scans of
// code_defines and code_calls for every simulation. In a large repository, this
// will cause massive heap allocations and potential OOM or timeouts.
// A test should be added that populates the kernel with 50k dummy definitions and calls,
// then asserts that SimulateAction completes within a strict budget (e.g., 500ms).

// TODO: TEST_GAP: Concurrency - Missing test for race conditions between SimulateAction and SetKernel.
// Dreamer struct lacks mutex protection. Parallel execution of SimulateAction (reading d.kernel)
// and SetKernel (writing d.kernel) causes undefined behavior or panic.
// A stress test with concurrent goroutines is needed to verify thread safety.

func TestDreamer_SimulateAction_Safe(t *testing.T) {
	d, _ := setupTestDreamer(t)
	ctx := context.Background()

	// 1. Simulate a safe action (e.g., read file)
	// No panic_state rules match by default given empty/default policy for this action
	req := ActionRequest{
		Type:   ActionReadFile,
		Target: "safe_file.txt",
	}

	result := d.SimulateAction(ctx, req)

	if result.Unsafe {
		t.Errorf("Expected action to be safe, got unsafe: %s", result.Reason)
	}
	if len(result.ProjectedFacts) == 0 {
		t.Error("Expected projected facts, got none")
	}
}

// TODO: TEST_GAP: Type Safety - Missing test for Atom/String dissonance in ActionType.
// projectEffects asserts projected_action with string(req.Type). If Mangle policy expects
// an Atom (e.g., /read_file) instead of String ("read_file"), safety checks may silently
// fail open. A test is needed to verify that the projected type matches the schema expectation.

func TestDreamer_SimulateAction_Unsafe(t *testing.T) {
	d, k := setupTestDreamer(t)
	ctx := context.Background()

	// 1. Inject a rule that flags specific actions as panic_state
	// Match against the projected action ID pattern or type
	// Note: We map ActionID in the policy rule to the first arg of panic_state
	policy := `
	panic_state(ActionID, "forbidden file") :-
		projected_action(ActionID, "read_file", "secret.txt").
	`
	k.AppendPolicy(policy)

	// 2. Simulate the forbidden action
	req := ActionRequest{
		Type:   ActionReadFile,
		Target: "secret.txt",
	}

	result := d.SimulateAction(ctx, req)

	if !result.Unsafe {
		t.Error("Expected action to be UNSAFE, got safe")
	}
	if result.Reason != "forbidden file" {
		t.Errorf("Expected reason 'forbidden file', got '%s'", result.Reason)
	}
	if result.ActionID == "" {
		t.Error("Expected ActionID to be set")
	}
}

// TODO: TEST_GAP: Input Extremes - Missing test for empty or massive target paths.
// 1. Empty Target: verify behavior when req.Target is empty string (potential match for current directory).
// 2. Massive Target: verify behavior when req.Target is a 1MB string (buffer overflow/DoS check).
// 3. Path Injection: verify that criticalPrefix handles "internal/../internal" correctly.

func TestDreamer_ProjectEffects(t *testing.T) {
	d, _ := setupTestDreamer(t)

	req := ActionRequest{
		Type:   ActionDeleteFile,
		Target: "internal/core/kernel.go",
	}

	// Access private method via test helper or just inspect result from Simulate (which internally calls projectEffects)
	// Since SimulateAction returns DreamResult with ProjectedFacts, we use that.
	ctx := context.Background()
	result := d.SimulateAction(ctx, req)

	foundMissing := false
	foundCritical := false

	for _, f := range result.ProjectedFacts {
		if f.Predicate == "projected_fact" && len(f.Args) > 1 {
			atom, ok := f.Args[1].(MangleAtom)
			if ok {
				if atom == "/file_missing" {
					foundMissing = true
				}
				if atom == "/critical_path_hit" {
					foundCritical = true
				}
			}
		}
	}

	if !foundMissing {
		t.Error("Expected /file_missing projection for delete_file")
	}
	if !foundCritical {
		t.Error("Expected /critical_path_hit projection for sensitive file")
	}
}
