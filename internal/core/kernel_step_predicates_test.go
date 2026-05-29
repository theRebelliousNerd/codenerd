package core

import (
	"testing"

	"codenerd/internal/types"
)

// kernel_step_predicates_test.go exercises the four executive-decision
// predicates that the Steps 2-5 migration moved from Go into the embedded
// Mangle policy corpus. These tests load the REAL corpus (NewRealKernel via
// setupMockKernel loads delegation.mg + validation.mg), assert the genuine EDB
// input facts that production asserts, and check the DERIVATIONS — proving the
// rules actually FIRE, not merely that the corpus loads.
//
// Predicates under test (all IDB / derived):
//   should_delegate/1          (delegation.mg, Step 4)
//   is_multi_step/0            (delegation.mg, Step 5)
//   action_complete_verified/1 (validation.mg, Step 2)
//   unvalidated_side_effect/2  (validation.mg, Step 3)
//
// Every negative case is paired with the minimal positive control (the single
// mutation that flips the derivation) so a "not derived" result can never be a
// silent false-negative from a malformed input fact rather than from the rule.

// queryDerived returns true iff the kernel derives at least one fact for the
// given query string. A fresh kernel is built per call so cases never leak
// facts into one another (the cross-turn retract behaviour is tested
// separately in the chat package round-trip test).
func queryDerived(t *testing.T, k *RealKernel, query string) bool {
	t.Helper()
	facts, err := k.Query(query)
	if err != nil {
		t.Fatalf("Query(%q) failed: %v", query, err)
	}
	return len(facts) > 0
}

// -----------------------------------------------------------------------------
// Step 4: should_delegate/1 — confidence gate (Conf >= 50, ShardType != /none)
// -----------------------------------------------------------------------------

func TestStep4_ShouldDelegate_ConfidenceGate(t *testing.T) {
	// delegation_candidate(/current_intent, ShardType, Conf) is the EDB the Go
	// shouldDelegate helper asserts. Mirror its exact arg types: arg1 plain
	// string, ShardType as MangleAtom, Conf as int64 (confidence*100).
	cases := []struct {
		name      string
		shard     types.MangleAtom
		conf      int64
		wantDeriv bool
	}{
		{"above_threshold_60", "/coder", 60, true},
		{"exact_threshold_50", "/coder", 50, true},  // Conf >= 50 is inclusive
		{"just_below_threshold_49", "/coder", 49, false},
		{"below_threshold_40", "/coder", 40, false},
		{"none_shard_high_conf", "/none", 90, false}, // /none rejected regardless of conf
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := setupMockKernel(t)
			if err := k.Assert(Fact{
				Predicate: "delegation_candidate",
				Args:      []any{"/current_intent", tc.shard, tc.conf},
			}); err != nil {
				t.Fatalf("assert delegation_candidate failed: %v", err)
			}

			got := queryDerived(t, k, "should_delegate")
			if got != tc.wantDeriv {
				t.Errorf("should_delegate derived=%v, want %v (shard=%s conf=%d)",
					got, tc.wantDeriv, tc.shard, tc.conf)
			}
		})
	}
}

// TestStep4_ShouldDelegate_BindsShardType confirms the derived fact carries the
// candidate's shard (not /none, not a wildcard) so the consumer can read it.
func TestStep4_ShouldDelegate_BindsShardType(t *testing.T) {
	k := setupMockKernel(t)
	if err := k.Assert(Fact{
		Predicate: "delegation_candidate",
		Args:      []any{"/current_intent", types.MangleAtom("/researcher"), int64(75)},
	}); err != nil {
		t.Fatalf("assert failed: %v", err)
	}

	// Pattern query: should_delegate(/researcher) must match; should_delegate(/none) must not.
	if !queryDerived(t, k, "should_delegate(/researcher)") {
		t.Error("should_delegate(/researcher) not derived for conf=75 candidate")
	}
	if queryDerived(t, k, "should_delegate(/none)") {
		t.Error("should_delegate(/none) wrongly derived")
	}
}

// -----------------------------------------------------------------------------
// Step 5: is_multi_step/0 — ORs multi_step_signal/1 (fires iff any signal)
// -----------------------------------------------------------------------------

func TestStep5_IsMultiStep_OrsSignals(t *testing.T) {
	t.Run("no_signal_not_derived", func(t *testing.T) {
		k := setupMockKernel(t)
		// Positive control is the sibling subtest below: with the SAME kernel
		// setup minus any signal, is_multi_step must stay false.
		if queryDerived(t, k, "is_multi_step") {
			t.Error("is_multi_step derived with zero multi_step_signal facts")
		}
	})

	t.Run("one_signal_derived", func(t *testing.T) {
		k := setupMockKernel(t)
		if err := k.Assert(Fact{
			Predicate: "multi_step_signal",
			Args:      []any{types.MangleAtom("/keyword_match")},
		}); err != nil {
			t.Fatalf("assert multi_step_signal failed: %v", err)
		}
		if !queryDerived(t, k, "is_multi_step") {
			t.Error("is_multi_step not derived with one multi_step_signal present")
		}
	})

	t.Run("multiple_signals_still_derived", func(t *testing.T) {
		k := setupMockKernel(t)
		for _, sig := range []string{"/campaign_verb", "/verb_count_high", "/compound_pattern"} {
			if err := k.Assert(Fact{
				Predicate: "multi_step_signal",
				Args:      []any{types.MangleAtom(sig)},
			}); err != nil {
				t.Fatalf("assert multi_step_signal %s failed: %v", sig, err)
			}
		}
		if !queryDerived(t, k, "is_multi_step") {
			t.Error("is_multi_step not derived with multiple signals present")
		}
	})
}

// -----------------------------------------------------------------------------
// Step 2: action_complete_verified/1 — POSITIVE validation only
//   (side_effect_attempted AND (action_validated OR critical_action_resolved))
// The soundness anchor: completion must NOT derive from mere absence of failure.
// -----------------------------------------------------------------------------

func TestStep2_ActionCompleteVerified_RequiresPositiveValidation(t *testing.T) {
	const id = "act-1"

	// Q4 SOUNDNESS CASE (the hole this rule closes): a side-effecting action
	// RAN (action_verified at conf 50 -> side_effect_attempted true) but did NOT
	// pass positive validation (50 < 80, so action_validated is false). The
	// action must NOT be reported complete.
	t.Run("attempted_but_unvalidated_not_complete", func(t *testing.T) {
		k := setupMockKernel(t)
		// action_verified(ActionID/string, ActionType/name, Method/name, Conf/number, Ts/number)
		mustAssert(t, k, "action_verified", id, types.MangleAtom("/write_file"), types.MangleAtom("/basic_validation"), int64(50), int64(1))
		if queryDerived(t, k, "action_complete_verified") {
			t.Error("action_complete_verified derived for an UNVALIDATED side effect (Q4 false-completion hole reopened)")
		}
	})

	// Positive control A: same action, confidence bumped to 90 (>= 80) ->
	// action_validated true -> completion derives via the validated arm.
	t.Run("positive_control_validated_arm", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssert(t, k, "action_verified", id, types.MangleAtom("/write_file"), types.MangleAtom("/basic_validation"), int64(90), int64(1))
		if !queryDerived(t, k, "action_complete_verified") {
			t.Error("action_complete_verified NOT derived for a validated side effect (conf 90)")
		}
	})

	// Positive control B: critical_action_resolved arm. Requires paranoid
	// validation at confidence 100 on a requires_paranoid_validation type
	// (/write_file is one). This drives critical_action_validated ->
	// critical_action_resolved -> action_complete_verified.
	t.Run("positive_control_critical_resolved_arm", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssert(t, k, "action_verified", id, types.MangleAtom("/write_file"), types.MangleAtom("/paranoid_validation"), int64(100), int64(1))
		if !queryDerived(t, k, "action_complete_verified") {
			t.Error("action_complete_verified NOT derived via critical_action_resolved arm (paranoid 100 on /write_file)")
		}
	})

	// Guard: a positively-validated action whose type is OUTSIDE the
	// interactive_side_effect_type allowlist must NOT count as an attempted
	// side effect, so completion does not derive. /unknown_tool is not in the
	// allowlist. This proves side_effect_attempted is the gating join, not just
	// action_validated.
	t.Run("validated_but_non_side_effecting_type_not_complete", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssert(t, k, "action_verified", id, types.MangleAtom("/unknown_tool"), types.MangleAtom("/basic_validation"), int64(90), int64(1))
		if queryDerived(t, k, "action_complete_verified") {
			t.Error("action_complete_verified derived for a non-allowlisted action type")
		}
	})
}

// -----------------------------------------------------------------------------
// Step 3: unvalidated_side_effect/2 — the "no-opinion" partition
//   side_effect_attempted AND !complete AND !failed AND !escalated
// Must derive only when the action ran with NO validator opinion; must be
// excluded by each of the three negation arms.
// -----------------------------------------------------------------------------

func TestStep3_UnvalidatedSideEffect_NoOpinionPartition(t *testing.T) {
	// Base no-opinion case (positive control for the whole table): a
	// side-effecting action ran at conf 50 (attempted=true, validated=false,
	// not failed, not escalated) -> in validation limbo -> derives.
	t.Run("no_opinion_case_derives", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssert(t, k, "action_verified", "u-1", types.MangleAtom("/write_file"), types.MangleAtom("/basic_validation"), int64(50), int64(1))
		if !queryDerived(t, k, "unvalidated_side_effect(u-1, /write_file)") {
			t.Error("unvalidated_side_effect NOT derived for the genuine no-opinion case")
		}
	})

	// Exclusion arm 1: VALIDATED. Bump conf to 90 -> action_complete_verified
	// true -> !action_complete_verified excludes it.
	t.Run("excluded_when_validated", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssert(t, k, "action_verified", "u-2", types.MangleAtom("/write_file"), types.MangleAtom("/basic_validation"), int64(90), int64(1))
		// Positive control inside this subtest: confirm it WOULD have been a side
		// effect (completion derives), so exclusion is real, not a load failure.
		if !queryDerived(t, k, "action_complete_verified(u-2)") {
			t.Fatal("setup error: action_complete_verified(u-2) should derive at conf 90")
		}
		if queryDerived(t, k, "unvalidated_side_effect(u-2, /write_file)") {
			t.Error("unvalidated_side_effect derived for a VALIDATED action (should be excluded)")
		}
	})

	// Exclusion arm 2: FAILED. action_validation_failed -> action_failed_validation
	// true -> !action_failed_validation excludes it. (Same fact also makes
	// side_effect_attempted true via the second clause.)
	t.Run("excluded_when_failed", func(t *testing.T) {
		k := setupMockKernel(t)
		// action_validation_failed(ActionID/string, ActionType/name, Reason/name, Details/string, Ts/number)
		mustAssert(t, k, "action_validation_failed", "u-3", types.MangleAtom("/write_file"), types.MangleAtom("/hash_mismatch"), "content hash mismatch", int64(1))
		if queryDerived(t, k, "unvalidated_side_effect(u-3, /write_file)") {
			t.Error("unvalidated_side_effect derived for a FAILED action (should be excluded)")
		}
	})

	// Exclusion arm 3: ESCALATED. A no-opinion verify (conf 50, attempted=true,
	// not validated, not failed) PLUS an action_escalated fact for the same id
	// -> !action_escalated excludes it. Without the escalation it would derive
	// (the no_opinion_case_derives subtest is that control).
	t.Run("excluded_when_escalated", func(t *testing.T) {
		k := setupMockKernel(t)
		mustAssert(t, k, "action_verified", "u-4", types.MangleAtom("/write_file"), types.MangleAtom("/basic_validation"), int64(50), int64(1))
		// action_escalated(ActionID/string, Reason/name, Ts/number)
		mustAssert(t, k, "action_escalated", "u-4", types.MangleAtom("/max_retries"), int64(1))
		if queryDerived(t, k, "unvalidated_side_effect(u-4, /write_file)") {
			t.Error("unvalidated_side_effect derived for an ESCALATED action (should be excluded)")
		}
	})
}

// mustAssert asserts a fact and fails the test on error. Keeps the table cases
// readable while preserving the exact arg types production uses.
func mustAssert(t *testing.T, k *RealKernel, predicate string, args ...any) {
	t.Helper()
	if err := k.Assert(Fact{Predicate: predicate, Args: args}); err != nil {
		t.Fatalf("assert %s%v failed: %v", predicate, args, err)
	}
}
