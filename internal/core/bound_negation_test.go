package core

import (
	"strings"
	"testing"
)

// Bound-negation regression suite.
//
// A negated literal containing ANY anonymous wildcard is a no-op in this Mangle
// build: the wildcard leaves the literal unbound rather than existentially
// quantified, so the negation excludes nothing. Only a fully-bound negated
// literal filters.
//
// Several .mg files already carried prose warnings about this (shards.mg:26,
// validation.mg:117 and :300, codedom_safety.mg:175, prompt_northstar.mg:188),
// but prose cannot fail a build, and the workaround had never been applied to
// the constitution. Three safety rules were silently inert as a result.
//
// These tests exist so the semantics are pinned and the three fixed rules
// cannot quietly regress. If a future Mangle upgrade makes wildcard negation
// work, TestMangleNegation_WildcardFormsDoNotExclude is the test that will
// fail, and that failure is the signal to simplify the helpers away.

// TestMangleNegation_WildcardFormsDoNotExclude characterizes the engine.
// It asserts the CURRENT behavior, including the broken cases, so the
// workaround's justification is executable rather than a comment.
func TestMangleNegation_WildcardFormsDoNotExclude(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	k.LoadSchemas(`
Decl bn_item(Id) bound [/string].
Decl bn_one(Id) bound [/string].
Decl bn_two(Id, V) bound [/string, /string].
Decl bn_has_two(Id) bound [/string].
Decl bn_neg_bound(Id) bound [/string].
Decl bn_neg_helper(Id) bound [/string].
Decl bn_neg_tail_wild(Id) bound [/string].
Decl bn_neg_all_wild(Id) bound [/string].
`)
	k.LoadPolicy(`
bn_has_two(Id)       :- bn_two(Id, V).
bn_neg_bound(Id)     :- bn_item(Id), !bn_one(Id).
bn_neg_helper(Id)    :- bn_item(Id), !bn_has_two(Id).
bn_neg_tail_wild(Id) :- bn_item(Id), !bn_two(Id, _).
bn_neg_all_wild(Id)  :- bn_item(Id), !bn_one(_).
`)

	for _, id := range []string{"a", "b"} {
		if err := k.Assert(Fact{Predicate: "bn_item", Args: []any{id}}); err != nil {
			t.Fatalf("assert bn_item: %v", err)
		}
	}
	// Only "a" has the things being negated.
	if err := k.Assert(Fact{Predicate: "bn_one", Args: []any{"a"}}); err != nil {
		t.Fatalf("assert bn_one: %v", err)
	}
	if err := k.Assert(Fact{Predicate: "bn_two", Args: []any{"a", "x"}}); err != nil {
		t.Fatalf("assert bn_two: %v", err)
	}

	count := func(pred string) int {
		facts, err := k.Query(pred)
		if err != nil {
			t.Fatalf("query %s: %v", pred, err)
		}
		return len(facts)
	}

	// Working forms: only "b" survives, because "a" is excluded.
	if got := count("bn_neg_bound"); got != 1 {
		t.Errorf("!bn_one(Id) (fully bound) derived %d rows, want 1 — "+
			"fully-bound negation must exclude", got)
	}
	if got := count("bn_neg_helper"); got != 1 {
		t.Errorf("!bn_has_two(Id) (bound helper) derived %d rows, want 1 — "+
			"this is the pattern every wildcard negation was rewritten to use", got)
	}

	// Broken forms, asserted as broken. A change here is not a regression in
	// this package — it means the engine's negation semantics changed, and the
	// bound helpers introduced for it can be revisited.
	if got := count("bn_neg_tail_wild"); got != 2 {
		t.Errorf("!bn_two(Id, _) derived %d rows, want 2. If this is now 1, "+
			"wildcard negation started working: revisit the bound helpers in "+
			"schemas_safety.mg SECTION 11C and the .mg comments citing this test", got)
	}
	if got := count("bn_neg_all_wild"); got != 2 {
		t.Errorf("!bn_one(_) derived %d rows, want 2. If this is now 0, "+
			"wildcard negation started working: revisit the bound helpers", got)
	}
}

// deniedFor reports whether permission_denied covers action.
func deniedFor(facts []Fact, action string) bool {
	for _, f := range facts {
		if strings.HasPrefix(f.String(), "permission_denied("+action+",") {
			return true
		}
	}
	return false
}

// TestConstitution_AdminOverrideSuppressesDangerousDenial is the behavioral
// regression for `!admin_override(_)`, which excluded nothing — so an admin
// override could not suppress the denial it exists to authorize.
//
// Both denial rules must be satisfied to escape: signed_approval covers the
// second rule, admin_override the first. /delete_file is in the constitution's
// built-in dangerous_action set.
func TestConstitution_AdminOverrideSuppressesDangerousDenial(t *testing.T) {
	withOverride, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	assertNegFact(t, withOverride, "signed_approval", "/delete_file")
	assertNegFact(t, withOverride, "admin_override", "alice")

	denied, err := withOverride.Query("permission_denied")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if deniedFor(denied, "/delete_file") {
		t.Error("an admin override did not suppress the dangerous-action denial")
	}
}

// TestConstitution_WithoutAdminOverride_ShouldStillDeny is the other half:
// the fix must not turn the rule off entirely.
func TestConstitution_WithoutAdminOverride_ShouldStillDeny(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	assertNegFact(t, k, "signed_approval", "/delete_file")

	denied, err := k.Query("permission_denied")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if !deniedFor(denied, "/delete_file") {
		t.Error("a dangerous action with no admin override must still be denied")
	}
}

// TestConstitution_BlockedCountReflectsDenials is the regression for
// `!action_denied(_, _)`, which excluded nothing — so the counter reported
// zero blocked actions while denials existed. A safety metric that always
// reads clean is worse than no metric.
func TestConstitution_BlockedCountReflectsDenials(t *testing.T) {
	withDenial, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	assertNegFact(t, withDenial, "action_denied", "/some_action", "/some_reason")

	counts, err := withDenial.Query("blocked_learned_action_count")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	for _, f := range counts {
		if strings.Contains(f.String(), "(0)") {
			t.Errorf("blocked_learned_action_count reported 0 while a denial existed: %v", counts)
		}
	}

	clean, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	cleanCounts, err := clean.Query("blocked_learned_action_count")
	if err != nil {
		t.Fatalf("query: %v", err)
	}
	if len(cleanCounts) == 0 {
		t.Error("with no denials the count should derive 0, not nothing")
	}
}

// TestConstitution_PermittedActionIsNotAlsoDenied is the regression for
// `!permitted(Action, _, _)`, which excluded nothing — so every candidate
// action was denied whether or not the constitution permitted it.
func TestConstitution_PermittedActionIsNotAlsoDenied(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	// /read_file is a safe_action in the constitution; make it a candidate and
	// give it a pending action so permitted/3 can derive.
	assertNegFact(t, k, "candidate_action", "/read_file")
	if err := k.Assert(Fact{
		Predicate: "pending_action",
		Args:      []any{"act-1", "/read_file", "some/target.go", "payload", 1},
	}); err != nil {
		t.Fatalf("assert pending_action: %v", err)
	}

	permitted, err := k.Query("action_is_permitted")
	if err != nil {
		t.Fatalf("query action_is_permitted: %v", err)
	}
	if len(permitted) == 0 {
		t.Skip("permitted/3 did not derive for /read_file in this fixture; " +
			"the denial assertion below would be vacuous")
	}

	denied, err := k.Query("action_denied")
	if err != nil {
		t.Fatalf("query action_denied: %v", err)
	}
	for _, f := range denied {
		if strings.Contains(f.String(), "Not constitutionally permitted") &&
			strings.Contains(f.String(), "/read_file") {
			t.Errorf("a permitted action was also marked denied: %v", f)
		}
	}
}

func assertNegFact(t *testing.T, k *RealKernel, predicate string, args ...string) {
	t.Helper()
	anyArgs := make([]any, len(args))
	for i, a := range args {
		anyArgs[i] = a
	}
	if err := k.Assert(Fact{Predicate: predicate, Args: anyArgs}); err != nil {
		t.Fatalf("assert %s: %v", predicate, err)
	}
}
