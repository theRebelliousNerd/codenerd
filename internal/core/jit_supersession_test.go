package core

import (
	"testing"
)

// Supersession is what lets an optimized, model-pinned variant of an atom stand
// in for the shipped one without deleting it.
//
// It needed its own predicate because `prohibited` could not do the job: that
// predicate is consulted only on the vector/flesh path and the atom_requires
// pull-in, never for a mandatory atom, so the pre-existing rule
// `prohibited(B) :- atom_conflicts(A, B), mandatory_selection(A)` derived a fact
// with no effect. These tests were written first as a probe against the real
// kernel and failed exactly that way -- both atoms came back in
// selected_result -- which is why they are kept.

func supersessionFixture(t *testing.T, k *RealKernel, facts ...string) {
	t.Helper()
	for _, f := range facts {
		if err := k.AssertString(f); err != nil {
			t.Fatalf("AssertString(%q) error = %v", f, err)
		}
	}
}

func selectedAtoms(t *testing.T, k *RealKernel) map[string]bool {
	t.Helper()
	rows, err := k.Query("selected_result")
	if err != nil {
		t.Fatalf("Query(selected_result) error = %v", err)
	}
	out := map[string]bool{}
	for _, f := range rows {
		if len(f.Args) < 3 {
			continue
		}
		if id, ok := f.Args[0].(string); ok {
			out[id] = true
		}
	}
	return out
}

// base is the shipped atom; variant is the marathon-optimized one, pinned to the
// serving model and declaring that it supersedes the base.
func overlayPair() []string {
	return []string{
		`atom("methodology/tdd")`,
		`is_mandatory("methodology/tdd")`,
		`atom_priority("methodology/tdd", 70)`,
		`atom_tag("methodology/tdd", /shard, /coder)`,

		`atom("methodology/tdd@claude_opus_4")`,
		`is_mandatory("methodology/tdd@claude_opus_4")`,
		`atom_priority("methodology/tdd@claude_opus_4", 80)`,
		`atom_tag("methodology/tdd@claude_opus_4", /shard, /coder)`,
		`atom_tag("methodology/tdd@claude_opus_4", /provider, /anthropic)`,
		`atom_tag("methodology/tdd@claude_opus_4", /model, /claude_opus_4)`,
		`atom_conflicts("methodology/tdd@claude_opus_4", "methodology/tdd")`,
	}
}

func TestSupersession_VariantReplacesBaseOnItsOwnModel(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	supersessionFixture(t, k, append(overlayPair(),
		`current_context(/shard, /coder)`,
		`current_context(/provider, /anthropic)`,
		`current_context(/model, /claude_opus_4)`,
	)...)

	sel := selectedAtoms(t, k)
	if !sel["methodology/tdd@claude_opus_4"] {
		t.Error("the pinned variant was not selected on the model it was written for")
	}
	if sel["methodology/tdd"] {
		t.Error("the base atom survived alongside its variant: the prompt now carries " +
			"both the original and the optimized guidance for the same concern")
	}
}

// The half that matters most. A superseding atom that is itself blocked this
// compile must not take its base down with it -- otherwise optimizing for one
// model would silently delete that guidance for every other model.
func TestSupersession_BaseSurvivesWhenVariantIsBlocked(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	supersessionFixture(t, k, append(overlayPair(),
		`current_context(/shard, /coder)`,
		`current_context(/provider, /openai)`,
		`current_context(/model, /gpt_4o)`,
	)...)

	sel := selectedAtoms(t, k)
	if sel["methodology/tdd@claude_opus_4"] {
		t.Error("the pinned variant leaked onto a model it was not written for")
	}
	if !sel["methodology/tdd"] {
		t.Error("BASE ATOM LOST on a non-matching model: supersession removed the " +
			"shipped atom without supplying a replacement")
	}
}

// Supersession is directional. A mutual pair would cancel both atoms and delete
// the guidance entirely, so producers must emit one direction only. This records
// what the kernel actually does if that rule is broken, so the consequence is
// documented rather than discovered in a prompt.
func TestSupersession_MutualConflictCancelsBoth(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	supersessionFixture(t, k,
		`atom("a")`, `is_mandatory("a")`, `atom_priority("a", 70)`,
		`atom("b")`, `is_mandatory("b")`, `atom_priority("b", 70)`,
		`atom_conflicts("a", "b")`,
		`atom_conflicts("b", "a")`,
		`current_context(/shard, /coder)`,
	)

	sel := selectedAtoms(t, k)
	if sel["a"] || sel["b"] {
		t.Errorf("expected a mutual conflict to cancel both atoms, got %v; if this "+
			"changed, the directionality warning in jit_compiler.mg needs updating", sel)
	}
}

// Supersession must not resurrect an atom that context already blocked, and must
// not fire from a superseding atom nobody selected.
func TestSupersession_InertWithoutConflicts(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	supersessionFixture(t, k,
		`atom("methodology/tdd")`,
		`is_mandatory("methodology/tdd")`,
		`atom_priority("methodology/tdd", 70)`,
		`atom_tag("methodology/tdd", /shard, /coder)`,
		`current_context(/shard, /coder)`,
	)

	if !selectedAtoms(t, k)["methodology/tdd"] {
		t.Error("an atom with no conflict declared against it was dropped; " +
			"supersession must be inert on the shipped corpus")
	}
}
