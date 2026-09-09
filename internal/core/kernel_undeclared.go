package core

import (
	"sync"

	"codeberg.org/TauCeti/mangle-go/ast"
	"codenerd/internal/logging"
)

// Asserting a predicate with no Decl is silent, and the fact is unreachable.
//
// factToAtom is Decl-blind, so a fact whose predicate was never declared
// converts cleanly, lands in the EDB, and returns nil from Assert — every
// signal a caller has says it worked. But the fixpoint only derives what the
// program declares, so Query never sees it. The fact is written into a space
// nothing reads.
//
// This is not a hypothetical typo. A static scan of non-test Go against the
// 1839 Decls a booted kernel loads found 82 distinct predicates asserted from
// production code with no declaration anywhere — whole state machines
// (campaign_paused, tdd_phase, ouroboros_phase, python_snapshot) whose facts
// have never been visible to a rule. It surfaced through a test that asserted
// a hundred facts concurrently, was told a hundred times that each succeeded,
// read back zero, and reported "Concurrency lost data".
//
// Assert deliberately does NOT start returning an error for these. Eighty-two
// live call sites would begin failing at once, and the honest fix for each is
// a per-predicate decision — declare it and wire a consumer, or drop the
// assert — not a blanket rejection chosen here. What was missing was any
// signal at all, so this provides one: a warning naming the predicate and its
// arity, once per predicate per process, plus a budget test
// (TestUndeclaredAssertBudget) that stops the count from growing.
//
// Arity is part of the identity on purpose. A fact asserted at an arity the
// Decl does not declare is invisible for exactly the same reason as one with
// no Decl at all, and reads as a much more confusing bug.
type undeclaredWarner struct {
	seen sync.Map // ast.PredicateSym -> struct{}
}

// warnIfUndeclaredLocked reports a predicate the loaded program does not
// declare. Call holding k.mu.
//
// Before the first evaluation programInfo is nil and nothing is known yet;
// staying quiet there avoids warning about every boot fact. Those facts are
// re-checked on later asserts of the same predicate only if they recur, which
// is the right trade: this is a diagnostic, not an accounting.
func (k *RealKernel) warnIfUndeclaredLocked(f Fact) {
	if k.sandbox {
		// Sandbox kernels trial-compile candidate rules; undeclared predicates
		// are an expected intermediate state there, not a defect.
		return
	}
	if k.programInfo == nil || k.programInfo.Decls == nil {
		return
	}
	sym := ast.PredicateSym{Symbol: f.Predicate, Arity: len(f.Args)}
	if _, declared := k.programInfo.Decls[sym]; declared {
		return
	}
	if _, already := k.undeclared.seen.LoadOrStore(sym, struct{}{}); already {
		return
	}
	logging.Get(logging.CategoryKernel).Warn(
		"Assert: %s/%d has no Decl — the fact is stored but no rule can read it, and Query will not return it. "+
			"Declare it (and give it a consumer) or drop the assert.",
		f.Predicate, len(f.Args))
}
