package context

import (
	"testing"

	"codenerd/internal/config"

	"github.com/stretchr/testify/require"
)

// The caller share is a policy decision over measured facts, not a Go
// constant: all of a pool that fits the share, else the top N the allowance
// buys at the measured mean line. Each case below is hand-computed from the
// rule (allowance = budget * share / 100, N = allowance / avg) and proved
// through a real working set, so an empty join fails here instead of
// silently deriving nothing in production. These are the db869152 cases; the
// decider now takes the dimension and the numbers are unchanged.
func TestDecideRenderCount_Callers(t *testing.T) {
	spans := config.DefaultWorkingConfig()
	spans.HolographicCallersSharePercent = 10
	w, err := NewWorkingSet(t.TempDir(), "render", spans)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	ctx := t.Context()

	// 1000 bytes at 10% is 100 bytes. Three callers at 30 bytes need 90:
	// the pool fits, so all three render.
	n, err := w.DecideRenderCount(ctx, HolographicDimCallers, "fit.go", 3, 30, 1000)
	require.NoError(t, err)
	require.Equal(t, 3, n, "a pool that fits the share renders whole")

	// Ten callers at 30 bytes need 300 against the same 100-byte allowance:
	// 100/30 truncates to 3.
	n, err = w.DecideRenderCount(ctx, HolographicDimCallers, "tight.go", 10, 30, 1000)
	require.NoError(t, err)
	require.Equal(t, 3, n, "a pool past the share renders what the allowance buys")

	// A render replaces the last one's facts: the tight pool above must not
	// leak into this target's count.
	n, err = w.DecideRenderCount(ctx, HolographicDimCallers, "other.go", 2, 30, 1000)
	require.NoError(t, err)
	require.Equal(t, 2, n, "per-render facts must not accumulate across renders")
}

// The share the policy divides by is the config's, not a literal in the
// rule: the same pool and budget render differently under two shares.
func TestDecideRenderCount_CallerShareComesFromConfig(t *testing.T) {
	decide := func(t *testing.T, share int) int {
		t.Helper()
		spans := config.DefaultWorkingConfig()
		spans.HolographicCallersSharePercent = share
		w, err := NewWorkingSet(t.TempDir(), "share", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		n, err := w.DecideRenderCount(t.Context(), HolographicDimCallers, "s.go", 10, 30, 1000)
		require.NoError(t, err)
		return n
	}
	// 10% of 1000 is 100 bytes: 100/30 = 3.
	require.Equal(t, 3, decide(t, 10))
	// 50% of 1000 is 500 bytes: ten callers need 300, so all render.
	require.Equal(t, 10, decide(t, 50))
}

// One decider, keyed by the dimension. Count dimensions each spend their own
// share. /outline_signature spends the outline block's allowance divided by
// the entries kept, and never goes below the floor. A character allowance is
// not clamped to the entry count: two entries can keep 250 runes each.
func TestDecideRenderCount_RuleTable(t *testing.T) {
	t.Run("shares are independent", func(t *testing.T) {
		spans := config.DefaultWorkingConfig()
		spans.HolographicCallersSharePercent = 10
		spans.HolographicSignaturesSharePercent = 50
		spans.HolographicTypesSharePercent = 2
		w, err := NewWorkingSet(t.TempDir(), "dims", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		ctx := t.Context()

		// 10% of 1000 is 100 bytes: 100/30 = 3.
		n, err := w.DecideRenderCount(ctx, HolographicDimCallers, "s.go", 10, 30, 1000)
		require.NoError(t, err)
		require.Equal(t, 3, n)

		// 50% of 1000 is 500 bytes: ten lines need 300, so all render. A
		// leaked caller pool would still say 3.
		n, err = w.DecideRenderCount(ctx, HolographicDimSignatures, "s.go", 10, 30, 1000)
		require.NoError(t, err)
		require.Equal(t, 10, n)

		// 2% of 1000 is 20 bytes: 20/30 = 0. The signature decision above
		// must not leave its pool behind.
		n, err = w.DecideRenderCount(ctx, HolographicDimTypes, "s.go", 10, 30, 1000)
		require.NoError(t, err)
		require.Equal(t, 0, n)

		// And the type decision must not stick: two callers at 30 bytes
		// need 60 of the 100-byte caller allowance.
		n, err = w.DecideRenderCount(ctx, HolographicDimCallers, "s.go", 2, 30, 1000)
		require.NoError(t, err)
		require.Equal(t, 2, n)
	})

	t.Run("outline signature floor wins under the division", func(t *testing.T) {
		// 10% of 1000 is 100 bytes. Four entries: 100/4 = 25, and the floor
		// is 40, so the floor wins.
		spans := config.DefaultWorkingConfig()
		spans.HolographicOutlineSharePercent = 10
		spans.HolographicOutlineSignatureFloor = 40
		w, err := NewWorkingSet(t.TempDir(), "floor", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		n, err := w.DecideRenderCount(t.Context(), HolographicDimOutlineSignature, "s.go", 4, 1, 1000)
		require.NoError(t, err)
		require.Equal(t, 40, n)
	})

	t.Run("outline signature division wins over the floor", func(t *testing.T) {
		// 40% of 1000 is 400 bytes. Four entries: 400/4 = 100, above the
		// floor, so the division wins.
		spans := config.DefaultWorkingConfig()
		spans.HolographicOutlineSharePercent = 40
		spans.HolographicOutlineSignatureFloor = 40
		w, err := NewWorkingSet(t.TempDir(), "div", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		n, err := w.DecideRenderCount(t.Context(), HolographicDimOutlineSignature, "s.go", 4, 1, 1000)
		require.NoError(t, err)
		require.Equal(t, 100, n)
	})

	t.Run("outline signature equality is one row", func(t *testing.T) {
		// 40% of 400 is 160 bytes. Four entries: 160/4 = 40, equal to the
		// floor. The two rules split on Floor <= Per against Floor > Per,
		// so this boundary derives one allowance; a second row is an error
		// from DecideRenderCount rather than a number.
		spans := config.DefaultWorkingConfig()
		spans.HolographicOutlineSharePercent = 40
		spans.HolographicOutlineSignatureFloor = 40
		w, err := NewWorkingSet(t.TempDir(), "eq", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		n, err := w.DecideRenderCount(t.Context(), HolographicDimOutlineSignature, "s.go", 4, 1, 400)
		require.NoError(t, err)
		require.Equal(t, 40, n)
	})

	t.Run("character allowance is not clamped to the entry count", func(t *testing.T) {
		// 50% of 1000 is 500 bytes. Two entries: 500/2 = 250 runes. Clamping
		// to the entry count would return 2 and cut every signature to a
		// couple of glyphs.
		spans := config.DefaultWorkingConfig()
		spans.HolographicOutlineSharePercent = 50
		spans.HolographicOutlineSignatureFloor = 40
		w, err := NewWorkingSet(t.TempDir(), "chars", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		n, err := w.DecideRenderCount(t.Context(), HolographicDimOutlineSignature, "s.go", 2, 1, 1000)
		require.NoError(t, err)
		require.Equal(t, 250, n)
	})

	t.Run("unknown or malformed dimension", func(t *testing.T) {
		spans := config.DefaultWorkingConfig()
		spans.HolographicCallersSharePercent = 10
		w, err := NewWorkingSet(t.TempDir(), "bad", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		ctx := t.Context()
		for _, dim := range []string{"callers", "/Callers", "/callers x", "", "/", "/nope"} {
			_, err := w.DecideRenderCount(ctx, dim, "s.go", 3, 30, 1000)
			require.Error(t, err, dim)
		}
		// A rejected dimension must not poison the engine: three callers at
		// 30 bytes need 90 of the 100-byte allowance.
		n, err := w.DecideRenderCount(ctx, HolographicDimCallers, "s.go", 3, 30, 1000)
		require.NoError(t, err)
		require.Equal(t, 3, n)
	})

	t.Run("outline signature with no entries does not query", func(t *testing.T) {
		spans := config.DefaultWorkingConfig()
		spans.HolographicCallersSharePercent = 10
		w, err := NewWorkingSet(t.TempDir(), "zero", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		ctx := t.Context()
		_, err = w.DecideRenderCount(ctx, HolographicDimOutlineSignature, "s.go", 0, 1, 1000)
		require.Error(t, err)
		n, err := w.DecideRenderCount(ctx, HolographicDimCallers, "s.go", 3, 30, 1000)
		require.NoError(t, err)
		require.Equal(t, 3, n, "a refused signature allowance must leave the engine able to decide")
	})
}
