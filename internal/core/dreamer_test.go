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

// TODO: TEST_GAP: Performance/OOM - Full Table Scan (O(N) Complexity)
// The codeGraphProjections function performs d.kernel.Query("code_defines") which fetches
// ALL definitions in the system.
// Mathematical Projection:
// - 1k facts: ~1ms
// - 100k facts: ~100ms
// - 1M facts: ~1s per simulation
// A load test is required with 100k+ facts to verify if the system hangs or OOMs.

// TODO: TEST_GAP: Concurrency - Race Condition (Pointer Safety)
// Dreamer.SetKernel (write) and Dreamer.SimulateAction (read) access the kernel pointer
// without a mutex. This is undefined behavior.
// A test with 10 concurrent readers and 1 concurrent writer is needed to prove the panic.

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

// TODO: TEST_GAP: Type Safety - Mangle Atom vs String Mismatch
// projectEffects uses string(req.Type) -> "delete_file" (String)
// But projected_fact uses MangleAtom("/file_missing") -> /file_missing (Atom)
// Mangle rules expecting /delete_file will FAIL to fire against "delete_file".
// A test must verify that the projected Go types align with the Mangle schema expectations.

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

// TODO: TEST_GAP: Input Extremes - Path Normalization & Canonicalization
// criticalPrefix uses naive strings.Contains.
// Missing coverage for:
// 1. "../" traversal (e.g. "internal/core/../foo")
// 2. Double slashes (e.g. "internal//core")
// 3. Case sensitivity on Linux vs Mac (e.g. "Internal/Core")
// 4. Unicode homoglyphs.

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

// TODO: TEST_GAP: Security - Exploit Scenario: Whitespace Expansion
// "rm  -rf /" (two spaces) bypasses "rm -rf" check.
// Test case needed to prove bypass.

// TODO: TEST_GAP: Security - Exploit Scenario: Flag Reordering
// "rm -fr /" bypasses "rm -rf" check.
// Test case needed to prove bypass.

// TODO: TEST_GAP: Security - Exploit Scenario: Flag Splitting
// "rm -r -f /" bypasses "rm -rf" check.
// Test case needed to prove bypass.

// TODO: TEST_GAP: Security - Exploit Scenario: Shell Features
// "eval $(echo ... | base64 -d)" executes hidden commands.
// Test case needed to prove bypass.

// TODO: TEST_GAP: Security - Exploit Scenario: Indirect Execution
// "python -c 'import os; ...'" executes commands.
// Test case needed to prove bypass.

// TODO: TEST_GAP: Resource Exhaustion - Unbounded DreamCache
// The DreamCache grows indefinitely.
// A test with 1M simulations is needed to verify OOM behavior.

// TODO: TEST_GAP: Performance - Kernel Clone Cost
// SimulateAction performs deep copy.
// A test measuring latency with 100k facts in kernel is needed.

// TODO: TEST_GAP: Fragile Defaults - Unknown Action Types
// New ActionTypes (e.g., ActionNetworkRequest) hit default switch case and project nothing.
// Test needed to verify behavior (likely false negative safety).

// TODO: TEST_GAP: Reliability - Panic Safety
// AssertWithoutEval can panic on malformed inputs.
// Fuzz test needed with random types in Fact Args.
