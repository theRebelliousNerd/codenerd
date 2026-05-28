package core

import (
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"codenerd/internal/features"
)

// TestKernelEval_DiffEnginePathGatedByFeaturesRegistry exercises the
// internal/core ↔ internal/features boundary: kernel_eval.go calls
// features.IsDiffEvalEnabled() on every evaluate(), and that resolves
// against the active FeaturesConfig pointer. This complements the
// existing TestKernelDifferentialEval (which uses t.Setenv) by driving
// the gate via SetActive — the path .nerd/config.json actually takes.
//
// We assert state on k.diffEngine directly because we're in package
// core; the field is unexported but accessible from this same-package
// test, no production change required.
func TestKernelEval_DiffEnginePathGatedByFeaturesRegistry(t *testing.T) {
	// Ensure no stray env override interferes. CODENERD_DIFF_EVAL is the
	// only env that flips this flag, so unset it via t.Setenv("") (which
	// the resolveBool helper treats as "no override").
	if v := os.Getenv("CODENERD_DIFF_EVAL"); v != "" {
		t.Setenv("CODENERD_DIFF_EVAL", "")
	}
	t.Cleanup(func() { features.SetActive(nil) })

	policy := `
	Decl widget(Name).
	Decl big_widget(Name).
	big_widget(X) :- widget(X), :string:contains(X, "big").
	`

	t.Run("flag_off_skips_diff_engine", func(t *testing.T) {
		fa := false
		features.SetActive(&features.FeaturesConfig{DiffEval: &fa})
		require.False(t, features.IsDiffEvalEnabled(), "precondition: gate must be off")

		k := setupMockKernel(t)
		k.AppendPolicy(policy)
		require.NoError(t, k.Evaluate(), "first evaluate")

		require.NoError(t, k.Assert(Fact{Predicate: "widget", Args: []any{"/big_one"}}))
		require.NoError(t, k.Evaluate(), "second evaluate")

		k.mu.RLock()
		diff := k.diffEngine
		k.mu.RUnlock()
		require.Nil(t, diff, "flag off → diff engine MUST stay nil")
	})

	t.Run("flag_on_builds_diff_engine", func(t *testing.T) {
		ta := true
		features.SetActive(&features.FeaturesConfig{DiffEval: &ta})
		require.True(t, features.IsDiffEvalEnabled(), "precondition: gate must be on")

		k := setupMockKernel(t)
		k.AppendPolicy(policy)
		// First evaluate parses the policy + builds programInfo (full
		// path runs because there's no prior diff engine state).
		require.NoError(t, k.Evaluate(), "first evaluate")
		// Assert a fact so the next evaluate has non-empty delta — that
		// is when evaluateDiffLocked actually instantiates the engine
		// for the kernel's lazy-build contract.
		require.NoError(t, k.Assert(Fact{Predicate: "widget", Args: []any{"/big_one"}}))
		require.NoError(t, k.Evaluate(), "second evaluate triggers diff build")

		k.mu.RLock()
		diff := k.diffEngine
		mEng := k.diffMangleEngine
		k.mu.RUnlock()
		require.NotNil(t, diff, "flag on → diff engine must be built")
		require.NotNil(t, mEng, "diff engine must be backed by a mangle engine")
	})

	t.Run("toggling_off_after_on_does_not_resurrect_engine", func(t *testing.T) {
		// Document the invariant: SetActive does NOT retroactively tear
		// down a previously-built diff engine. It just affects which
		// path the NEXT evaluate() takes. The kernel only invalidates
		// the diff engine on retract/clear/policy change, so a build
		// that already happened persists across a flag flip until one
		// of those events. Test asserts the documented behaviour so a
		// future refactor changing it must update this test.
		ta, fa := true, false
		features.SetActive(&features.FeaturesConfig{DiffEval: &ta})
		k := setupMockKernel(t)
		k.AppendPolicy(policy)
		require.NoError(t, k.Evaluate())
		require.NoError(t, k.Assert(Fact{Predicate: "widget", Args: []any{"/seed"}}))
		require.NoError(t, k.Evaluate())

		k.mu.RLock()
		built := k.diffEngine != nil
		k.mu.RUnlock()
		require.True(t, built, "diff engine should be built while flag was on")

		features.SetActive(&features.FeaturesConfig{DiffEval: &fa})
		require.False(t, features.IsDiffEvalEnabled())
		// Another evaluate with no facts dirty is a no-op fast path; the
		// engine remains because nothing forced a teardown.
		require.NoError(t, k.Evaluate())

		k.mu.RLock()
		stillBuilt := k.diffEngine != nil
		k.mu.RUnlock()
		require.True(t, stillBuilt, "documented: flag flip alone does not tear down engine")
	})
}
