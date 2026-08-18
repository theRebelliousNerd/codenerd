package core

import (
	"testing"
)

// Provider/model pinning is enforced on the LIVE path by jit_compiler.mg, whose
// regime_dimension block makes /provider and /model fail-closed. Go's
// MatchesContext implements the same rule, but it only runs in
// fallbackFleshSelection -- the path used when Mangle is unavailable. So the Go
// tests in internal/prompt cannot show that a pinned atom is actually blocked in
// production; only evaluating the real kernel can.
//
// The facts asserted here are exactly what AtomSelector.buildContextFacts emits:
// current_context(/provider, /anthropic) from CompilationContext.GenerateFacts,
// and atom_tag(ID, /provider, /anthropic) from addTags.
func assertPinFixture(t *testing.T, k *RealKernel, facts ...string) {
	t.Helper()
	for _, f := range facts {
		if err := k.AssertString(f); err != nil {
			t.Fatalf("AssertString(%q) error = %v", f, err)
		}
	}
}

func blockedAtoms(t *testing.T, k *RealKernel) map[string]bool {
	t.Helper()
	results, err := k.Query("blocked_by_context")
	if err != nil {
		t.Fatalf("Query(blocked_by_context) error = %v", err)
	}
	blocked := make(map[string]bool, len(results))
	for _, f := range results {
		if len(f.Args) == 0 {
			continue
		}
		if id, ok := f.Args[0].(string); ok {
			blocked[id] = true
		}
	}
	return blocked
}

func TestPinnedAtomBlockedOnDifferentProvider(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	assertPinFixture(t, k,
		`prompt_atom("pinned/anthropic", /methodology, 70, 10, /false)`,
		`atom_tag("pinned/anthropic", /provider, /anthropic)`,
		`prompt_atom("unpinned/shared", /methodology, 70, 10, /false)`,
		// The compile is served by a different vendor.
		`current_context(/provider, /openai)`,
	)

	blocked := blockedAtoms(t, k)

	if !blocked["pinned/anthropic"] {
		t.Error("an atom pinned to /anthropic was NOT blocked on an /openai compile; " +
			"vendor-specific guidance is leaking into another vendor's prompt")
	}
	if blocked["unpinned/shared"] {
		t.Error("an unpinned atom was blocked; pinning must not touch the rest of the corpus")
	}
}

// The fail-closed half: a compile that names no provider cannot demonstrate the
// pin holds, so the atom sits out rather than being admitted by default. This is
// the case the permissive blocked_by_context rule alone would get wrong, and the
// reason /provider had to join regime_dimension.
func TestPinnedAtomBlockedWhenProviderUnset(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	assertPinFixture(t, k,
		`prompt_atom("pinned/anthropic", /methodology, 70, 10, /false)`,
		`atom_tag("pinned/anthropic", /provider, /anthropic)`,
		`prompt_atom("unpinned/shared", /methodology, 70, 10, /false)`,
		// Deliberately no current_context(/provider, ...).
		`current_context(/shard, /coder)`,
	)

	blocked := blockedAtoms(t, k)

	if !blocked["pinned/anthropic"] {
		t.Error("a provider-pinned atom must be blocked when the compile names no provider; " +
			"regime_dimension(/provider) is what makes this fail-closed")
	}
	if blocked["unpinned/shared"] {
		t.Error("an unpinned atom was blocked on a compile with no provider")
	}
}

func TestPinnedAtomAdmittedOnMatchingProviderAndModel(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	assertPinFixture(t, k,
		`prompt_atom("pinned/anthropic", /methodology, 70, 10, /false)`,
		`atom_tag("pinned/anthropic", /provider, /anthropic)`,
		`atom_tag("pinned/anthropic", /model, /claude_opus_4)`,
		`current_context(/provider, /anthropic)`,
		// CompilationContext emits BOTH the exact and the family token, which is
		// what lets an atom pin at family granularity survive a dated snapshot.
		`current_context(/model, /claude_opus_4_20260501)`,
		`current_context(/model, /claude_opus_4)`,
	)

	if blockedAtoms(t, k)["pinned/anthropic"] {
		t.Error("an atom pinned to the serving provider and model family was blocked; " +
			"fail-closed must not starve a model of the guidance learned for it")
	}
}

// A family pin must not match a different model from the same vendor.
func TestPinnedAtomBlockedOnDifferentModelSameProvider(t *testing.T) {
	k, err := NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	assertPinFixture(t, k,
		`prompt_atom("pinned/opus", /methodology, 70, 10, /false)`,
		`atom_tag("pinned/opus", /provider, /anthropic)`,
		`atom_tag("pinned/opus", /model, /claude_opus_4)`,
		`current_context(/provider, /anthropic)`,
		`current_context(/model, /claude_sonnet_4_20260501)`,
		`current_context(/model, /claude_sonnet_4)`,
	)

	if !blockedAtoms(t, k)["pinned/opus"] {
		t.Error("an atom pinned to claude_opus_4 was admitted on a claude_sonnet_4 compile")
	}
}
