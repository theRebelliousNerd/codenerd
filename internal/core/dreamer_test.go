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

// TODO: TEST_GAP: Boundary Value - Massive Inputs
// The current implementation of codeGraphProjections performs a full table scan
// of 'code_defines' and 'code_calls'. We need a test that injects 100k+ facts
// to verify this doesn't OOM or timeout on large repositories.

// TODO: TEST_GAP: State Conflict - Race Condition
// Dreamer.SetKernel and Dreamer.SimulateAction access the kernel pointer without
// synchronization. A concurrent test is needed to prove safety during kernel updates.

// TODO: TEST_GAP: Boundary Value - Null/Empty/Whitespace
// Verify behavior when ActionRequest.Target is empty, whitespace, or invalid.
// Should ensure critical_path_hit is not falsely triggered or bypassed.

// TODO: TEST_GAP: Negative Testing - Dangerous Command Evasion
// The isDangerousCommand check is simple string matching. We need to test:
// 1. Case variations ("RM -rf")
// 2. Argument reordering ("rm -r -f")
// 3. Path qualification ("/bin/rm")
// 4. Shell obfuscation
// 5. Chained commands ("echo safe; rm -rf /")

// TODO: TEST_GAP: Boundary Value - Nil Kernel Resilience
// Verify that Dreamer handles a nil kernel gracefully, especially if the kernel
// becomes nil between checks in the SimulateAction pipeline.
