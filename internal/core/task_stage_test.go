package core

import (
	"testing"

	"codenerd/internal/types"
)

// task_stage_test.go pins the Phase-1 intent -> lifecycle-stage table
// (internal/core/defaults/policy/task_stage.mg). The table must derive
// task_stage/1 from user_intent as a Mangle table, never a Go switch: every
// case asserts the genuine production-shaped user_intent EDB fact and checks
// the DERIVATION, so a boot failure (duplicate Decl, bad syntax) or a dead
// rule fails loudly here instead of silently mis-shaping context.
//
// Conventions reused from kernel_step_predicates_test.go: a fresh kernel per
// case via setupMockKernel (NewRealKernel boots the full embedded corpus,
// including task_stage.mg), production-shaped asserts via assertIntent /
// mustAssert, derivation checks via queryDerived. No new helpers are defined
// in this file — the shared ones already carry the exact arg types
// production uses (a bespoke multi-param helper trips the modularity guard).

// TestTaskStage_MapsRepresentativeVerbPerStage covers all 9 stages with one
// representative verb each, plus the key routing choices documented in
// task_stage.mg: /fix -> /debug (SWE-bench slice, not /implement),
// /security -> /harden, /document -> /design, /run -> /verify.
func TestTaskStage_MapsRepresentativeVerbPerStage(t *testing.T) {
	cases := []struct {
		name     string
		category string
		verb     string
		want     string
	}{
		{"research_is_ideate", "/query", "/research", "/ideate"},
		{"design_is_design", "/mutation", "/design", "/design"},
		{"plan_is_plan", "/instruction", "/plan", "/plan"},
		{"create_is_implement", "/mutation", "/create", "/implement"},
		{"test_is_verify", "/mutation", "/test", "/verify"},
		{"security_is_harden", "/mutation", "/security", "/harden"},
		{"fix_is_debug_not_implement", "/mutation", "/fix", "/debug"},
		{"refactor_is_refactor", "/mutation", "/refactor", "/refactor"},
		{"review_is_review", "/query", "/review", "/review"},
		// Category is ignored: stage follows the verb, not what the turn is about.
		{"fix_under_query_still_debug", "/query", "/fix", "/debug"},
		{"run_is_verify", "/mutation", "/run", "/verify"},
		{"document_is_design", "/query", "/document", "/design"},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			k := setupMockKernel(t)
			assertIntent(t, k, tc.category, tc.verb, "some target")
			if !queryDerived(t, k, "task_stage("+tc.want+")") {
				t.Errorf("task_stage(%s) not derived for verb %s", tc.want, tc.verb)
			}
			// Exactly one stage per verb: a verb mapping to two stages would
			// mis-shape the per-stage context policy downstream.
			facts, err := k.Query("task_stage")
			if err != nil {
				t.Fatalf("Query(task_stage) failed: %v", err)
			}
			if len(facts) != 1 {
				t.Errorf("task_stage derived %d facts for verb %s, want exactly 1", len(facts), tc.verb)
			}
		})
	}
}

// TestTaskStage_SubagentIntentID proves the derivation joins on verb for any
// intent ID: subagent turns (/task_intent_N) get a stage exactly like the
// interactive turn (/current_intent) does.
func TestTaskStage_SubagentIntentID(t *testing.T) {
	k := setupMockKernel(t)
	mustAssert(t, k, "user_intent",
		"/task_intent_3",
		types.MangleAtom("/mutation"),
		types.MangleAtom("/fix"),
		"auth middleware",
		"none",
	)
	if !queryDerived(t, k, "task_stage(/debug)") {
		t.Error("task_stage(/debug) not derived for subagent /task_intent_3 /fix intent")
	}
}

// TestTaskStage_UnmappedVerbsDeriveNothing: conversational and workflow verbs
// with no lifecycle slice derive NO stage. An absent stage is honest; a
// guessed stage would mis-shape context.
func TestTaskStage_UnmappedVerbsDeriveNothing(t *testing.T) {
	for _, verb := range []string{"/greet", "/help", "/commit", "/git"} {
		t.Run(verb, func(t *testing.T) {
			k := setupMockKernel(t)
			assertIntent(t, k, "/query", verb, "none")
			if queryDerived(t, k, "task_stage") {
				t.Errorf("task_stage derived for unmapped verb %s, want none", verb)
			}
		})
	}
}
