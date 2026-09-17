package session

import (
	"testing"
)

// TestIntentRequiresReasoningModel_ConsultVerbReturnsFalseAndCaches pins the
// F-REC-3 fix: "/consult/reviewer" is not a valid Mangle atom (second "/"),
// so without the consult short-circuit every lookup re-validated, re-logged
// the "is not an atom" debug line, and never stored the answer. The false
// result itself is correct — policy has no /consult/* reasoning mapping
// (delegation.mg) — it just has to be cached like every other verdict.
func TestIntentRequiresReasoningModel_ConsultVerbReturnsFalseAndCaches(t *testing.T) {
	e, _, _ := newRoutingExecutor(t)

	if got := e.intentRequiresReasoningModel("/consult/reviewer"); got != false {
		t.Fatalf("intentRequiresReasoningModel(%q) = %v, want false", "/consult/reviewer", got)
	}
	cached, ok := e.reasoningVerbCache.Load("/consult/reviewer")
	if !ok {
		t.Fatalf("reasoningVerbCache has no entry for %q after first lookup", "/consult/reviewer")
	}
	if cached.(bool) != false {
		t.Fatalf("reasoningVerbCache[%q] = %v, want false", "/consult/reviewer", cached)
	}
	// Second call must still answer false (served from cache, no re-log).
	if got := e.intentRequiresReasoningModel("/consult/reviewer"); got != false {
		t.Fatalf("second intentRequiresReasoningModel(%q) = %v, want false", "/consult/reviewer", got)
	}
}

// TestIntentRequiresReasoningModel_InvalidVerbReturnsFalseAndCaches pins the
// second half of the F-REC-3 fix: the invalid-verb branch stores false before
// returning so the "is not an atom" debug line is emitted once per verb, not
// per lookup.
func TestIntentRequiresReasoningModel_InvalidVerbReturnsFalseAndCaches(t *testing.T) {
	e, _, _ := newRoutingExecutor(t)

	if got := e.intentRequiresReasoningModel("/bad verb"); got != false {
		t.Fatalf("intentRequiresReasoningModel(%q) = %v, want false", "/bad verb", got)
	}
	cached, ok := e.reasoningVerbCache.Load("/bad verb")
	if !ok {
		t.Fatalf("reasoningVerbCache has no entry for %q after first lookup", "/bad verb")
	}
	if cached.(bool) != false {
		t.Fatalf("reasoningVerbCache[%q] = %v, want false", "/bad verb", cached)
	}
}

// TestIntentRequiresReasoningModel_ReasoningVerbReturnsTrue guards the F-REC-3
// MUST-NOT-CHANGE: a valid reasoning verb from policy still answers true.
// /campaign is a direct reasoning_intensive_verb fact in delegation.mg.
func TestIntentRequiresReasoningModel_ReasoningVerbReturnsTrue(t *testing.T) {
	e, _, _ := newRoutingExecutor(t)

	if got := e.intentRequiresReasoningModel("/campaign"); got != true {
		t.Fatalf("intentRequiresReasoningModel(%q) = %v, want true", "/campaign", got)
	}
}
