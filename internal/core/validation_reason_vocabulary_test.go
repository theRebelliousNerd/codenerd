package core

import (
	"testing"
	"time"
)

// action_validation_failed's Reason slot is declared /name (schemas_execution.mg)
// and policy/validation.mg branches on specific atoms in it. Nothing connects the
// two automatically: ValidationResult.ToFacts derives the atom in Go, the rules
// name it in Mangle, and for a long time they did not agree at all - the rules
// matched the prose ("content hash mismatch") that ToFacts puts in the *Details*
// slot, so no strategy rule could ever fire and every failure fell through to the
// /escalate catch-all.
//
// This test is the join between them. It runs the real producer against the real
// corpus and asserts the strategy the rules pick matches SelfHealer's own table in
// self_healing.go. Reword a validator message without updating
// validationReasonPrefixes, or add a reason atom without a rule, and it fails here
// rather than silently degrading recovery to escalation in production.
func TestValidationReason_WhenToFactsCategorizes_ShouldDriveTheSameStrategyAsSelfHealer(t *testing.T) {
	for _, tc := range []struct {
		name     string
		errText  string
		derived  string // intermediate predicate, "" when the rule reads the reason directly
		strategy string
	}{
		{"hash mismatch", "content hash mismatch", "validation_hash_mismatch", "/retry"},
		{"hash mismatch, paranoid second read", "content hash mismatch (second read)", "validation_hash_mismatch", "/retry"},
		{"syntax", "syntax validation failed", "validation_syntax_error", "/rollback"},
		{"codedom syntax", "Go syntax error after CodeDOM edit", "validation_syntax_error", "/rollback"},
		{"element lost", "target element no longer exists after edit", "validation_element_lost", "/rollback"},
		{"read back", "cannot read back file", "", "/retry"},
		{"edit not applied", "file hash unchanged after edit - edit may not have been applied", "", "/retry"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, err := NewRealKernelWithWorkspace(t.TempDir())
			if err != nil {
				t.Fatalf("NewRealKernelWithWorkspace: %v", err)
			}
			// Canary: a corpus that fails analysis derives nothing at all, which
			// would make every assertion below pass vacuously if they were
			// negative and mislead badly if they were not.
			safe, err := k.Query("safe_action")
			if err != nil {
				t.Fatalf("Query(safe_action): %v", err)
			}
			if len(safe) == 0 {
				t.Fatal("safe_action derived 0 facts - the kernel did not reach a fixpoint")
			}

			vr := &ValidationResult{
				ActionID:   "act-1",
				ActionType: ActionWriteFile,
				Verified:   false,
				Method:     ValidationMethodHash,
				Error:      tc.errText,
				Timestamp:  time.Now(),
			}
			for _, f := range vr.ToFacts() {
				if err := k.Assert(f); err != nil {
					t.Fatalf("assert %s: %v", f.Predicate, err)
				}
			}

			if tc.derived != "" {
				rows, err := k.Query(tc.derived)
				if err != nil {
					t.Fatalf("Query(%s): %v", tc.derived, err)
				}
				if len(rows) == 0 {
					t.Errorf("%s derived nothing from ToFacts(%q); the reason atom and the rule disagree",
						tc.derived, tc.errText)
				}
			}

			heal, err := k.Query("needs_self_healing")
			if err != nil {
				t.Fatalf("Query(needs_self_healing): %v", err)
			}
			found := false
			for _, h := range heal {
				if len(h.Args) == 2 && h.Args[1] == tc.strategy {
					found = true
				}
			}
			if !found {
				t.Errorf("needs_self_healing did not derive %s for %q; got %v", tc.strategy, tc.errText, heal)
			}
		})
	}
}

// goal_topic's Topic slot is /name because Decomposer.seedDocFacts asserts
// fmt.Sprintf("/%s", topic), which types.Fact.ToAtom promotes to a name constant.
// The Decl said /string for a long time while campaign_rules.mg matched
// /migration, /refactor and /greenfield - a disagreement no head-only scan could
// see, since goal_topic has no head anywhere in the corpus.
func TestGoalTopic_WhenAssertedAsTheDecomposerBuildsIt_ShouldMatchTheCampaignRules(t *testing.T) {
	k, err := NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	mustAssert := func(f Fact) {
		t.Helper()
		if err := k.Assert(f); err != nil {
			t.Fatalf("assert %s: %v", f.Predicate, err)
		}
	}
	mustAssert(Fact{Predicate: "campaign_goal", Args: []any{"camp1", "migrate the auth database"}})
	// Exactly the shape decomposer_documents.go builds.
	mustAssert(Fact{Predicate: "goal_topic", Args: []any{"camp1", "/migration"}})

	rows, err := k.Query("goal_requires_campaign")
	if err != nil {
		t.Fatalf("Query(goal_requires_campaign): %v", err)
	}
	if len(rows) == 0 {
		t.Error("goal_requires_campaign derived nothing from a producer-shaped goal_topic fact")
	}
}
