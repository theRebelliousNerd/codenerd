package core

import (
	"testing"

	"codenerd/internal/types"
)

// The verification rules S4 adds to policy/coder_safety.mg negate each other in
// a chain — turn_unverified negates turn_verified, which negates turn_wrote —
// and they join predicates declared in three other files (turn_evidence and
// build_state from the shipped schemas, write_oriented_intent from
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

	// A write-oriented turn that wrote, with both gates measured green.
	assert(types.Fact{Predicate: "build_state", Args: []any{types.MangleAtom("/passing")}})
	assert(types.Fact{Predicate: "test_state", Args: []any{types.MangleAtom("/passing")}})
	assert(types.Fact{Predicate: "turn_evidence", Args: []any{
		types.MangleAtom("/create"), 1, 1, 1,
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
