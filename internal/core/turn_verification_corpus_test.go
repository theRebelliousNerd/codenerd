package core

import (
	"testing"

	"codenerd/internal/types"
)

// The verification rules S4 adds to policy/coder_safety.mg negate each other in
// a chain — turn_unverified negates turn_verified, which negates turn_wrote —
// and they join predicates declared in other files (turn_evidence and
// turn_gate in coder_safety.mg itself, write_oriented_intent from
// delegation.mg). A stratification cycle or a missing Decl in that chain does
// not fail loudly at the rule: it fails when the whole corpus is loaded.
//
// This boots the real kernel with the real shipped corpus — the same one
// production loads — and then exercises the chain end to end, because a corpus
// that loads is not the same as a corpus that derives. Every predicate below is
// queried through the kernel, so an undeclared one fails here rather than
// silently deriving nothing forever (the Decl-contract trap in
// internal/mangle/agents.md).
func TestCorpus_TurnVerificationRulesLoadAndDerive(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load and stratify: %v", err)
	}

	// Declaration is checked directly, not through Query: a query against an
	// UNDECLARED predicate returns an empty result set on this engine rather
	// than an error, so "the query did not fail" proves nothing at all. That is
	// the Decl-contract trap in miniature — both halves internally consistent,
	// nothing to fail — and it was confirmed while capturing this test's
	// fail-before run, where every one of these queries came back clean against
	// a corpus that declared none of them.
	for _, pred := range []struct {
		name  string
		arity int
	}{
		{"turn_verified", 1}, {"turn_unverified", 1}, {"turn_wrote", 1},
		{"turn_build_failed", 1}, {"turn_missing_evidence", 2},
		{"has_turn_acceptance", 1}, {"turn_done", 1}, {"turn_executed", 1},
		{"turn_gate", 3}, {"turn_build_green", 1}, {"turn_build_red", 1},
		{"turn_tests_green", 1}, {"turn_tests_red", 1},
		{"turn_evidence", 7}, {"hollow_success", 2}, {"has_hollow_success", 1},
		{"turn_created_source", 2}, {"turn_created_test", 3}, {"turn_missing_test", 2},
	} {
		if ok, reason := validatePredicateDeclaration(k, pred.name, pred.arity); !ok {
			t.Fatalf("%s/%d must be declared in the shipped corpus: %s", pred.name, pred.arity, reason)
		}
	}

	assert := func(f types.Fact) {
		t.Helper()
		if aerr := k.Assert(f); aerr != nil {
			t.Fatalf("assert %s%v: %v", f.Predicate, f.Args, aerr)
		}
	}
	count := func(pred string) int {
		t.Helper()
		facts, qerr := k.Query(pred)
		if qerr != nil {
			t.Fatalf("query %s: %v", pred, qerr)
		}
		return len(facts)
	}

	// A write-oriented turn that wrote, with both of ITS gates measured green.
	// The verdict reads turn_gate (this turn's measurement), not the
	// session-global build_state/test_state; those are asserted too, as
	// recordBuildState does, and prove nothing on their own (F2).
	assert(types.Fact{Predicate: "build_state", Args: []any{types.MangleAtom("/passing")}})
	assert(types.Fact{Predicate: "test_state", Args: []any{types.MangleAtom("/passing")}})
	turn := types.MangleAtom("/turn_corpus")
	assert(types.Fact{Predicate: "turn_gate", Args: []any{turn, types.MangleAtom("/build"), types.MangleAtom("/passing")}})
	assert(types.Fact{Predicate: "turn_gate", Args: []any{turn, types.MangleAtom("/test"), types.MangleAtom("/passing")}})
	assert(types.Fact{Predicate: "turn_evidence", Args: []any{
		turn, types.MangleAtom("/create"), 1, 1, 1,
		types.MangleAtom("/false"), types.MangleAtom("/false"),
	}})

	if got := count("turn_wrote"); got != 1 {
		t.Fatalf("turn_wrote = %d, want 1 for a /create that wrote", got)
	}
	if got := count("turn_verified"); got != 1 {
		t.Fatalf("turn_verified = %d, want 1 with both gates green", got)
	}
	if got := count("turn_done"); got != 1 {
		t.Fatalf("turn_done = %d, want 1 — completion must be reachable without acceptance", got)
	}
	if got := count("turn_unverified"); got != 0 {
		t.Fatalf("turn_unverified = %d, want 0 for a verified turn", got)
	}
	if got := count("turn_missing_evidence"); got != 0 {
		t.Fatalf("turn_missing_evidence = %d, want 0 for a verified turn", got)
	}
}

// External audit F1 (2026-09-19): the verdict relations were keyed by verb, so
// two turns with the same verb on one kernel -- a campaign's tasks share it --
// were one turn to the corpus. Turn A measured both gates green; turn B, the
// same /fix, measured its build only. B is not verified by A's test gate, B's
// missing test evidence is named against B alone, and A's verdict is A's.
func TestCorpus_TurnVerdictsAreKeyedByTurnNotVerb(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load and stratify: %v", err)
	}
	a, b := types.MangleAtom("/turn_a"), types.MangleAtom("/turn_b")
	for _, f := range []types.Fact{
		{Predicate: "turn_gate", Args: []any{a, types.MangleAtom("/build"), types.MangleAtom("/passing")}},
		{Predicate: "turn_gate", Args: []any{a, types.MangleAtom("/test"), types.MangleAtom("/passing")}},
		{Predicate: "turn_evidence", Args: []any{a, types.MangleAtom("/fix"), 1, 1, 1, types.MangleAtom("/false"), types.MangleAtom("/false")}},
		{Predicate: "turn_gate", Args: []any{b, types.MangleAtom("/build"), types.MangleAtom("/passing")}},
		{Predicate: "turn_evidence", Args: []any{b, types.MangleAtom("/fix"), 1, 1, 0, types.MangleAtom("/false"), types.MangleAtom("/false")}},
	} {
		if aerr := k.Assert(f); aerr != nil {
			t.Fatalf("assert %s%v: %v", f.Predicate, f.Args, aerr)
		}
	}
	rows := func(pred string) map[string][]string {
		t.Helper()
		facts, qerr := k.Query(pred)
		if qerr != nil {
			t.Fatalf("query %s: %v", pred, qerr)
		}
		out := map[string][]string{}
		for _, f := range facts {
			key := types.ExtractString(f.Args[0])
			rest := ""
			if len(f.Args) > 1 {
				rest = types.ExtractString(f.Args[1])
			}
			out[key] = append(out[key], rest)
		}
		return out
	}
	done := rows("turn_done")
	if len(done[string(a)]) != 1 || len(done[string(b)]) != 0 {
		t.Fatalf("turn_done = %v, want turn A only: B's test gate never ran", done)
	}
	missing := rows("turn_missing_evidence")
	if got := missing[string(b)]; len(got) != 1 || got[0] != "/tests_not_green" {
		t.Fatalf("turn_missing_evidence for B = %v, want [/tests_not_green]", got)
	}
	if got := missing[string(a)]; len(got) != 0 {
		t.Fatalf("turn_missing_evidence for A = %v, want none: B's gap is not A's", got)
	}
}
