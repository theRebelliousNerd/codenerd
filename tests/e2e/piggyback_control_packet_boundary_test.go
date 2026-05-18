//go:build integration

package e2e_test

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// =============================================================================
// TestE2E_Piggyback_ValidMangleUpdate_Asserted
// =============================================================================
// Verifies that a valid mangle_update (e.g. task_status) is accepted by
// FilterMangleUpdates and produces a parseable Fact.

func TestE2E_Piggyback_ValidMangleUpdate_Asserted(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	policy := core.MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"task_status":    {},
			"task_completed": {},
			"observation":    {},
			"diagnostic":     {},
		},
		MaxUpdates: 100,
	}

	updates := []string{
		`task_status(/auth_fix, /complete)`,
		`observation(/performance, "response time improved by 30%")`,
	}

	facts, blocked := core.FilterMangleUpdates(kernel, updates, policy)

	t.Logf("Accepted facts: %d, Blocked: %d", len(facts), len(blocked))

	if len(blocked) > 0 {
		for _, b := range blocked {
			t.Logf("  Blocked: %q reason=%s", b.Update, b.Reason)
		}
		t.Error("Expected all updates to be accepted")
	}

	if len(facts) != len(updates) {
		t.Errorf("Expected %d facts, got %d", len(updates), len(facts))
	}

	// Verify the facts have the correct predicates
	for _, f := range facts {
		t.Logf("  Fact: predicate=%s args=%v", f.Predicate, f.Args)
		if f.Predicate != "task_status" && f.Predicate != "observation" {
			t.Errorf("Unexpected predicate: %s", f.Predicate)
		}
	}
}

// =============================================================================
// TestE2E_Piggyback_UnsafeMangleUpdate_Blocked
// =============================================================================
// Verifies that unsafe predicates (not in the whitelist) are blocked.

func TestE2E_Piggyback_UnsafeMangleUpdate_Blocked(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	policy := core.MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"task_status": {},
		},
		MaxUpdates: 100,
	}

	unsafeUpdates := []string{
		`permitted(/delete_all_files)`,
		`safe_action(/rm_rf)`,
		`admin_override("attacker")`,
		`next_action(/escalate)`,
		`user_intent(/current_intent, /mutation, /delete, "everything", "now")`,
	}

	facts, blocked := core.FilterMangleUpdates(kernel, unsafeUpdates, policy)

	t.Logf("Accepted facts: %d, Blocked: %d", len(facts), len(blocked))

	if len(facts) > 0 {
		for _, f := range facts {
			t.Errorf("UNSAFE: Predicate %q was accepted — should be blocked", f.Predicate)
		}
	}

	if len(blocked) != len(unsafeUpdates) {
		t.Errorf("Expected all %d updates to be blocked, got %d", len(unsafeUpdates), len(blocked))
	}

	for _, b := range blocked {
		t.Logf("  Correctly blocked: %q reason=%s", b.Update, b.Reason)
	}
}

// =============================================================================
// TestE2E_Piggyback_ExcessiveUpdates_Capped
// =============================================================================
// Verifies that >MaxUpdates updates are truncated.

func TestE2E_Piggyback_ExcessiveUpdates_Capped(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	policy := core.MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"observation": {},
		},
		MaxUpdates: 5,
	}

	// Generate 100 valid updates
	updates := make([]string, 100)
	for i := 0; i < 100; i++ {
		updates[i] = fmt.Sprintf(`observation(/item_%d, "value_%d")`, i, i)
	}

	facts, blocked := core.FilterMangleUpdates(kernel, updates, policy)

	t.Logf("Accepted: %d, Blocked: %d (input: %d, max: %d)", len(facts), len(blocked), len(updates), policy.MaxUpdates)

	// Total accepted should be capped at MaxUpdates
	if len(facts) > policy.MaxUpdates {
		t.Errorf("Accepted %d facts but MaxUpdates is %d — cap not enforced", len(facts), policy.MaxUpdates)
	} else {
		t.Logf("PASS: Cap enforced (accepted=%d, max=%d)", len(facts), policy.MaxUpdates)
	}
}

// =============================================================================
// TestE2E_Piggyback_MalformedAtomString_NoPanic
// =============================================================================
// Verifies that badly formed Mangle atom strings don't crash the parser.

func TestE2E_Piggyback_MalformedAtomString_NoPanic(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	policy := core.MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"task_status": {},
			"observation": {},
		},
		MaxUpdates: 100,
	}

	malformed := []string{
		``,                             // empty
		`(`,                            // unclosed paren
		`)`,                            // unopened paren
		`task_status(`,                 // incomplete
		`task_status(, , ,)`,           // empty args
		`task_status("unclosed`,        // unclosed string
		strings.Repeat("x", 100000),    // very long
		`task_status(/a, /b). evil()`,  // injection attempt
		"task_status(/a, /b)\x00evil",  // null byte injection
		`task_status(/*/, /b)`,         // comment injection
		`task_status(/a) :- boom(X).`,  // rule injection
	}

	for i, update := range malformed {
		t.Run(fmt.Sprintf("malformed_%d", i), func(t *testing.T) {
			// This should NOT panic
			facts, blocked := core.FilterMangleUpdates(kernel, []string{update}, policy)
			t.Logf("Input: %q -> facts=%d blocked=%d", truncateForTest(update, 50), len(facts), len(blocked))

			// Either accepted (parsed successfully) or blocked (parse failed) — both ok
			// The key invariant: NO PANIC
		})
	}
}

// =============================================================================
// TestE2E_Piggyback_MixedUpdates_PartialAcceptance
// =============================================================================
// Verifies that a mix of valid and invalid updates results in partial acceptance.

func TestE2E_Piggyback_MixedUpdates_PartialAcceptance(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("Failed to create kernel: %v", err)
	}

	policy := core.MangleUpdatePolicy{
		AllowedPredicates: map[string]struct{}{
			"task_status": {},
			"observation": {},
		},
		MaxUpdates: 100,
	}

	mixed := []string{
		`task_status(/fix_bug, /complete)`,   // valid
		`permitted(/admin)`,                  // unsafe predicate
		`observation(/metric, "value")`,      // valid
		`next_action(/escalate)`,             // unsafe predicate
		`task_status(/review, /in_progress)`, // valid
	}

	facts, blocked := core.FilterMangleUpdates(kernel, mixed, policy)

	t.Logf("Mixed updates: accepted=%d blocked=%d", len(facts), len(blocked))

	// Expect exactly 3 accepted (task_status x2 + observation) and 2 blocked
	if len(facts) != 3 {
		t.Errorf("Expected 3 accepted facts, got %d", len(facts))
	}
	if len(blocked) != 2 {
		t.Errorf("Expected 2 blocked facts, got %d", len(blocked))
	}

	// Verify blocked predicates
	for _, b := range blocked {
		t.Logf("  Blocked: %q", b.Update)
	}

	// Assert to kernel to verify they're valid
	for _, f := range facts {
		if err := kernel.Assert(types.Fact{
			Predicate: f.Predicate,
			Args:      f.Args,
		}); err != nil {
			t.Errorf("Failed to assert accepted fact %s: %v", f.Predicate, err)
		}
	}
}

// truncateForTest shortens strings for test output readability.
func truncateForTest(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
