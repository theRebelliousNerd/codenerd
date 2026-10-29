package system

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
)

// TestPolicyDerivesNextAction_AuditRoutesToReviewer is the F-ROUTE-2 regression.
//
// The one-shot `nerd run` path (cmd/nerd/cmd_instruction.go) queries next_action
// and hard-fails with "no action derived from policy" when nothing is derived.
// /audit is a workhorse (delegating) verb — the taxonomy maps "analyze/audit X"
// to /audit — but it had no action_mapping in delegation.mg, so an audit
// instruction produced no next_action and the whole run failed (observed live:
// `nerd run "Analyze internal/perception ..."` -> exit 1). It must route to the
// reviewer exactly like its siblings /review, /analyze, and /security.
func TestPolicyDerivesNextAction_AuditRoutesToReviewer(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	// Mirror the live-failing intent: category /query, verb /audit.
	if err := kernel.Assert(core.Fact{
		Predicate: "user_intent",
		Args:      []any{"/current_intent", "/query", "/audit", "internal/perception", ""},
	}); err != nil {
		t.Fatalf("assert user_intent: %v", err)
	}

	facts, err := kernel.Query("next_action")
	if err != nil {
		t.Fatalf("Query(next_action) error = %v", err)
	}

	found := false
	for _, f := range facts {
		if len(f.Args) > 0 && strings.Contains(fmt.Sprintf("%v", f.Args[0]), "delegate_reviewer") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected next_action(/delegate_reviewer) for /audit; got %d facts: %+v", len(facts), facts)
	}
}
