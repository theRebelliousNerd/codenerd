package core

import "testing"

// A world-state gate (`world_states: [...]` on an atom, atom_tag(ID, /state, S)
// in the kernel) serves the atom exactly when the state holds for the compile:
// measured by the executor, or derived by the kernel as a need
// (policy/jit_needs.mg). Go's matchSelector has always read it that way; the
// kernel path read /state as situational, so a compile that carried no world
// state at all admitted every gated atom -- the need mechanism gated nothing
// on such a compile.
func TestWorldStateGatedAtomIsBlockedWhenTheStateDoesNotHold(t *testing.T) {
	for _, tc := range []struct {
		name    string
		context []string
		blocked bool
	}{
		{"no world state in the compile", nil, true},
		{"another world state holds", []string{`current_context(/state, /diagnostics)`}, true},
		{"the gating state holds", []string{`current_context(/state, /authoring_mangle)`}, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			k, err := NewRealKernel()
			if err != nil {
				t.Fatalf("NewRealKernel() error = %v", err)
			}
			facts := []string{
				`prompt_atom("gated/mangle", /language, 70, 10, /true)`,
				`atom_tag("gated/mangle", /state, /authoring_mangle)`,
				`prompt_atom("ungated/core", /methodology, 70, 10, /true)`,
			}
			assertPinFixture(t, k, append(facts, tc.context...)...)
			blocked := blockedAtoms(t, k)
			if blocked["gated/mangle"] != tc.blocked {
				t.Errorf("gated atom blocked = %v, want %v", blocked["gated/mangle"], tc.blocked)
			}
			if blocked["ungated/core"] {
				t.Error("an atom with no world-state gate was blocked")
			}
		})
	}
}
