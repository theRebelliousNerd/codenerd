package policy

import (
	"testing"

	"go.uber.org/goleak"
)

// The turn restriction (constitution.mg, section 7D) at the level that decides
// it: a restriction only narrows, a barred call is never approvable, and
// confirm-first is discharged by the person's decision for that exact call.

func restrictedCall(id, action, effect string, kinds ...string) []approvalFact {
	facts := []approvalFact{
		pending(id, action, "out.txt", `{"path":"out.txt"}`),
		{"pending_action_turn", []any{id, "/turn_1"}},
		{"pending_action_effect", []any{id, effect}},
	}
	for _, k := range kinds {
		facts = append(facts, approvalFact{"turn_restriction", []any{"/turn_1", k}})
	}
	return facts
}

func TestTurnRestriction_BarsWritesAndKeepsReads(t *testing.T) {
	defer goleak.VerifyNone(t)
	for _, kind := range []string{"/no_changes", "/read_only", "/dry_run", "/hypothetical"} {
		facts := append(restrictedCall("exec-w", "/write_file", "/write", kind),
			pending("exec-r", "/read_file", "in.txt", `{"path":"in.txt"}`),
			approvalFact{"pending_action_turn", []any{"exec-r", "/turn_1"}},
			approvalFact{"pending_action_effect", []any{"exec-r", "/read"}},
		)
		got := permittedTuples(t, approvalEngine(t, facts...))
		if got[[3]string{"/write_file", "out.txt", `{"path":"out.txt"}`}] {
			t.Errorf("%s: the write was permitted", kind)
		}
		if !got[[3]string{"/read_file", "in.txt", `{"path":"in.txt"}`}] {
			t.Errorf("%s: the read was withheld", kind)
		}
	}
}

func TestTurnRestriction_UnknownOrUnmeasuredEffectIsBarred(t *testing.T) {
	defer goleak.VerifyNone(t)
	got := permittedTuples(t, approvalEngine(t, restrictedCall("exec-u", "/read_file", "/unknown", "/no_changes")...))
	if got[[3]string{"/read_file", "out.txt", `{"path":"out.txt"}`}] {
		t.Error("an unknown effect was assumed read-only")
	}
	// No effect fact at all: the measurement did not run.
	facts := []approvalFact{
		pending("exec-m", "/read_file", "out.txt", `{"path":"out.txt"}`),
		{"pending_action_turn", []any{"exec-m", "/turn_1"}},
		{"turn_restriction", []any{"/turn_1", "/no_changes"}},
	}
	if permittedTuples(t, approvalEngine(t, facts...))[[3]string{"/read_file", "out.txt", `{"path":"out.txt"}`}] {
		t.Error("an unmeasured call under a barring restriction was permitted")
	}
}

func TestTurnRestriction_ConfirmFirstIsDischargedByTheExactDecision(t *testing.T) {
	defer goleak.VerifyNone(t)
	tuple := [3]string{"/write_file", "out.txt", `{"path":"out.txt"}`}

	eng := approvalEngine(t, restrictedCall("exec-w", "/write_file", "/write", "/confirm_first")...)
	if permittedTuples(t, eng)[tuple] {
		t.Fatal("a confirm-first write was permitted before the person decided")
	}
	if !requiredIDs(t, eng)["exec-w"] {
		t.Fatal("no approval request was derived for the confirm-first write")
	}

	facts := append(restrictedCall("exec-w", "/write_file", "/write", "/confirm_first"),
		granted("exec-w", "/write_file", "out.txt", `{"path":"out.txt"}`))
	if !permittedTuples(t, approvalEngine(t, facts...))[tuple] {
		t.Fatal("the person's decision for this exact call did not admit it")
	}

	// A decision for a different call joins nothing.
	facts = append(restrictedCall("exec-w", "/write_file", "/write", "/confirm_first"),
		granted("exec-other", "/write_file", "out.txt", `{"path":"out.txt"}`))
	if permittedTuples(t, approvalEngine(t, facts...))[tuple] {
		t.Fatal("another call's decision admitted this one")
	}

	// Reading owes nothing.
	facts = []approvalFact{
		pending("exec-r", "/read_file", "in.txt", `{"path":"in.txt"}`),
		{"pending_action_turn", []any{"exec-r", "/turn_1"}},
		{"pending_action_effect", []any{"exec-r", "/read"}},
		{"turn_restriction", []any{"/turn_1", "/confirm_first"}},
	}
	if !permittedTuples(t, approvalEngine(t, facts...))[[3]string{"/read_file", "in.txt", `{"path":"in.txt"}`}] {
		t.Fatal("confirm-first gated a read")
	}
}

// A person cannot approve what the user's own request barred: consent
// satisfies an obligation, it does not lift a prohibition.
func TestTurnRestriction_ConsentDoesNotLiftABar(t *testing.T) {
	defer goleak.VerifyNone(t)
	facts := append(restrictedCall("exec-w", "/write_file", "/write", "/no_changes", "/confirm_first"),
		granted("exec-w", "/write_file", "out.txt", `{"path":"out.txt"}`))
	eng := approvalEngine(t, facts...)
	if permittedTuples(t, eng)[[3]string{"/write_file", "out.txt", `{"path":"out.txt"}`}] {
		t.Fatal("a granted approval lifted no_changes")
	}
	if requiredIDs(t, eng)["exec-w"] {
		t.Fatal("a person was asked to approve a barred call")
	}
}

// A restriction is a negation joined into every arm: it cannot grant, and the
// call of a turn with no restriction is untouched.
func TestTurnRestriction_NoRestrictionChangesNothing(t *testing.T) {
	defer goleak.VerifyNone(t)
	facts := []approvalFact{
		pending("exec-w", "/write_file", "out.txt", `{"path":"out.txt"}`),
		{"pending_action_turn", []any{"exec-w", "/turn_2"}},
		{"pending_action_effect", []any{"exec-w", "/write"}},
		// a restriction on ANOTHER turn
		{"turn_restriction", []any{"/turn_1", "/no_changes"}},
	}
	if !permittedTuples(t, approvalEngine(t, facts...))[[3]string{"/write_file", "out.txt", `{"path":"out.txt"}`}] {
		t.Fatal("a sibling turn's restriction barred this turn's write")
	}
}
