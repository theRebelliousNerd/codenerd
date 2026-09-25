package mangle

import (
	"strings"
	"testing"
)

func grantPathOf(t *testing.T, src string) GrantPath {
	t.Helper()
	unit, err := ParseUnit(strings.NewReader(src))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	return GrantPathOf(unit.Clauses)
}

// The polarity walk: what can add a permitted fact is on the path, what can
// only remove one is not, and a negation under a negation turns back.
func TestGrantPathOf_FollowsPolarity(t *testing.T) {
	path := grantPathOf(t, `
permitted(A) :- allowed(A), !blocked(A).
allowed(A) :- listed(A).
blocked(A) :- flagged(A), !cleared(A).
`)
	for _, on := range []string{"permitted", "allowed", "listed", "cleared"} {
		if _, ok := path.Contains(on); !ok {
			t.Errorf("%s is not on the grant path %v", on, path.Sorted())
		}
	}
	for _, off := range []string{"blocked", "flagged"} {
		if via, ok := path.Contains(off); ok {
			t.Errorf("%s is on the grant path through %s; more of it only removes permitted facts", off, via)
		}
	}
	if via, _ := path.Contains("listed"); via != "allowed" {
		t.Errorf("listed reaches the root through %q, want allowed", via)
	}
}

// A rule that requires a human's consent widens its head only with it: its
// other premises are not grant path through it, the consent itself is.
func TestGrantPathOf_ConsentGuardsItsRule(t *testing.T) {
	path := grantPathOf(t, `
permitted(A) :- risky(A), signed_approval(A), admin_override(U).
permitted(A) :- trusted(A).
`)
	if via, ok := path.Contains("risky"); ok {
		t.Errorf("risky is on the path through %s; it widens permitted only with a signed approval", via)
	}
	for _, on := range []string{"signed_approval", "admin_override", "trusted"} {
		if _, ok := path.Contains(on); !ok {
			t.Errorf("%s is not on the grant path %v", on, path.Sorted())
		}
	}
}

// An aggregate can move either way when its input grows, so its premises are
// on the path whatever their polarity.
func TestGrantPathOf_AnAggregateIsReadBothWays(t *testing.T) {
	path := grantPathOf(t, `
permitted(A) :- quota(A, N), N < 3.
quota(A, N) :- used(A, X) |> do fn:group_by(A), let N = fn:count().
`)
	if _, ok := path.Contains("used"); !ok {
		t.Errorf("used feeds an aggregate that gates permitted, but is not on the path %v", path.Sorted())
	}
}

// A validator given a grant path refuses its heads, in any statement of the
// learned text, and still admits a predicate that only narrows.
func TestValidateLearnedRule_RefusesTheGrantPath(t *testing.T) {
	sv := NewSchemaValidator(`
Decl allowed(A) bound [/name].
Decl blocked(A) bound [/name].
Decl item(A) bound [/name].
`, "")
	if err := sv.LoadDeclaredPredicates(); err != nil {
		t.Fatal(err)
	}
	p := LearnedHeadProtection{GrantPath: grantPathOf(t, `permitted(A) :- allowed(A), !blocked(A).`)}

	if err := sv.ValidateLearnedRuleProtected(`allowed(A) :- item(A).`, p); err == nil || !strings.Contains(err.Error(), "allowed") {
		t.Errorf("a rule deriving allowed = %v, want it refused", err)
	}
	if err := sv.ValidateLearnedRuleProtected("blocked(A) :- item(A).\nallowed(/x).", p); err == nil {
		t.Error("a grant in the second statement was admitted")
	}
	if err := sv.ValidateLearnedRuleProtected(`blocked(A) :- item(A).`, p); err != nil {
		t.Errorf("a rule that only narrows permitted = %v, want it admitted", err)
	}
	// The static list holds for a caller with no program to derive from.
	if err := sv.ValidateLearnedRule("blocked(A) :- item(A).\npermitted(/x)."); err == nil {
		t.Error("a permitted fact in the second statement was admitted without a grant path")
	}
}
