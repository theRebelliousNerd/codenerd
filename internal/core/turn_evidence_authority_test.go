package core

import (
	"strings"
	"testing"
)

// The model may report what it saw and what it did. It may not report the state
// of the workspace, because the kernel decides whether the turn is done by
// reading exactly that (turn_verified, coder_safety.mg).
//
// Two routes existed for it to do so before S4:
//
//  1. The SessionPlanner's Piggyback allowlist carries the prefix "build_"
//     (internal/shards/system/planner.go), so build_state(/passing). was a
//     prefix match away.
//  2. ModelObservationPolicy — the allowlist shared by the session executor
//     (internal/session/executor.go) and the chat turn (cmd/nerd/chat/
//     process.go) — named "test_state" outright, and the MANDATORY prompt atom
//     protocol/piggyback/mangle_updates taught the model to write it on every
//     turn.
//
// Both were inert while nothing read those predicates as turn evidence. S1 gave
// them a producer (recordBuildState) and S4 made them decide the verdict, so
// they are now host witnesses and are refused ahead of any caller policy.
func TestModelCannotAssertTurnDone(t *testing.T) {
	forbidden := []string{
		"turn_done(/turn_1).",
		"turn_executed(/turn_1).",
		"turn_verified(/turn_1).",
		"turn_unverified(/turn_1).",
		"turn_wrote(/turn_1).",
		"turn_build_failed(/turn_1).",
		"turn_missing_evidence(/turn_1, /tests_not_green).",
		"turn_evidence(/turn_1, /create, 1, 1, 1, /false, /false).",
		"turn_acceptance(/turn_1, \"contract\", \"snapshot\").",
		"has_turn_acceptance(/turn_1).",
		"turn_created_source(/turn_1, \"pkg/foo.go\").",
		"turn_created_test(/turn_1, \"pkg/foo_test.go\", \"pkg/foo.go\").",
		"turn_test_coverage(/turn_1, \"pkg/foo.go\").",
		"turn_missing_test(/turn_1, \"pkg/foo.go\").",
		"hollow_success(/turn_1, \"requires side effects but no tool call succeeded\").",
		"has_hollow_success(/turn_1).",
		"has_turn_tools(/turn_1).",
		"has_turn_write(/turn_1).",
		"has_turn_test(/turn_1).",

		"turn_cost(\"s\", 1, 1, 1, 1, /done).",
		"build_state(/passing).",
		"build_state(/failing).",
		"test_state(/passing).",
	}

	// The most permissive policy there is: everything allowed, no prefixes, no
	// predicates named — which FilterMangleUpdates treats as "allow all". If
	// the hard block did not run first, every line above would be accepted.
	permissive := MangleUpdatePolicy{MaxUpdates: len(forbidden)}

	facts, blocked := FilterMangleUpdates(nil, forbidden, permissive)
	if len(facts) != 0 {
		t.Fatalf("a model must not be able to assert host witnesses; accepted %d: %+v", len(facts), facts)
	}
	if len(blocked) != len(forbidden) {
		t.Fatalf("blocked %d of %d host-witness updates", len(blocked), len(forbidden))
	}
	for _, b := range blocked {
		if !strings.Contains(b.Reason, "not allowed in mangle_updates") {
			t.Errorf("update %q blocked for the wrong reason: %s", b.Update, b.Reason)
		}
	}
}

// The planner's own policy is the concrete caller that made build_state
// reachable: it allow-lists the prefix "build_". The hard block must beat it.
func TestPlannerPrefixPolicyCannotReachBuildState(t *testing.T) {
	// Mirrors internal/shards/system/planner.go routeControlPacketToKernel.
	plannerPolicy := MangleUpdatePolicy{
		AllowedPrefixes: []string{
			"campaign_", "phase_", "task_", "context_", "plan_", "replan_",
			"build_", "architectural_", "suspicious_", "eligible_",
		},
		MaxUpdates: 200,
	}

	facts, blocked := FilterMangleUpdates(nil, []string{
		"build_state(/passing).",
		"build_status(/green).", // not a host witness: the prefix still works
	}, plannerPolicy)

	if len(blocked) != 1 || blocked[0].Update != "build_state(/passing)." {
		t.Fatalf("the build_ prefix must not reach build_state; blocked=%+v", blocked)
	}
	if len(facts) != 1 {
		t.Fatalf("the prefix must still admit non-witness build_ predicates, got %+v", facts)
	}
}

// ModelObservationPolicy is the allowlist the session executor and the chat
// turn both use. It must not name a host witness, because a dead entry sitting
// behind a hard block is two truths coexisting: the next reader cannot tell
// which one is live.
func TestModelObservationPolicyNamesNoHostWitness(t *testing.T) {
	policy := ModelObservationPolicy()
	for _, witness := range []string{
		"build_state", "test_state", "turn_evidence", "turn_executed",
		"turn_verified", "turn_done", "turn_acceptance", "turn_cost",
	} {
		if _, named := policy.AllowedPredicates[witness]; named {
			t.Errorf("ModelObservationPolicy names host witness %q; it is hard-blocked, so the entry is dead", witness)
		}
	}
}

// The model still has to be able to report what it observed, or this seam has
// broken the protocol instead of narrowing it.
func TestModelObservationPolicyStillAdmitsObservations(t *testing.T) {
	allowed := []string{
		"observation(\"key\", \"what you saw\").",
		"failing_test(\"TestName\", \"message\").",
		"modified(\"path/file.go\").",
		"task_completed(\"task_id\").",
	}
	facts, blocked := FilterMangleUpdates(nil, allowed, ModelObservationPolicy())
	if len(blocked) != 0 {
		t.Fatalf("the observation protocol must still work, blocked=%+v", blocked)
	}
	if len(facts) != len(allowed) {
		t.Fatalf("accepted %d of %d observations", len(facts), len(allowed))
	}
}
