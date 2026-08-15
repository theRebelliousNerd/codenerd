package system

import (
	"context"
	"testing"

	"codenerd/internal/core"
)

// escalation_needed/3's Target slot is a closed vocabulary of name constants:
// policy/system_session.mg emits /session_planner, policy/system_ooda.mg emits
// /ooda_loop, policy/shards.mg and system_core.mg emit /system_health, and
// campaign_rules.mg emits /campaign. The two Go producers here asserted bare
// Go strings ("session_planner", "constitution_gate") into the same relation,
// and a bare string without a leading slash becomes a string constant, not a
// name — so the .mg half and the Go half of one relation never unified. No
// error, no warning; a bound query just saw half the rows.
//
// This drives both producers through the real code path and asserts the row is
// visible to the name-bound query the policy corpus uses, with the pre-fix
// string form as the negative control.
func TestEscalationNeededTargetIsNameConstant(t *testing.T) {
	t.Run("session planner", func(t *testing.T) {
		k, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("NewRealKernel() error = %v", err)
		}

		planner := NewSessionPlannerShard()
		planner.Kernel = k
		planner.agenda = []AgendaItem{{
			ID:          "item-1",
			Description: "blocked on a missing dependency",
			Status:      "blocked",
		}}
		planner.retryCount["item-1"] = planner.config.MaxRetriesPerTask

		planner.checkBlockedTasks()

		assertEscalationTarget(t, k, "/session_planner", "session_planner")
	})

	t.Run("constitution gate", func(t *testing.T) {
		k, err := core.NewRealKernel()
		if err != nil {
			t.Fatalf("NewRealKernel() error = %v", err)
		}

		gate := NewConstitutionGateShard()
		gate.Kernel = k

		gate.escalateToUser(context.Background(), "write_file", "/etc/hosts", "domain not in allowlist")

		assertEscalationTarget(t, k, "/constitution_gate", "constitution_gate")
	})
}

// assertEscalationTarget checks that the escalation row is reachable through a
// name-bound query and NOT through the string-bound query the producer used
// before the fix.
func assertEscalationTarget(t *testing.T, k *core.RealKernel, nameForm, stringForm string) {
	t.Helper()

	rows, err := k.Query("escalation_needed(" + nameForm + ", S, R)")
	if err != nil {
		t.Fatalf("Query(escalation_needed(%s, S, R)) error = %v", nameForm, err)
	}
	if len(rows) != 1 {
		t.Errorf("escalation_needed(%s, S, R) got %d rows, want 1 — the Go producer is not emitting a name constant: %v",
			nameForm, len(rows), rows)
	}

	stale, err := k.Query(`escalation_needed("` + stringForm + `", S, R)`)
	if err != nil {
		t.Fatalf("Query(escalation_needed(%q, S, R)) error = %v", stringForm, err)
	}
	if len(stale) != 0 {
		t.Errorf("escalation_needed(%q, S, R) got %d rows, want 0 — the pre-fix string Target is back: %v",
			stringForm, len(stale), stale)
	}
}
