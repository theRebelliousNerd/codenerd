package context

import (
	"testing"

	"codenerd/internal/config"

	"github.com/stretchr/testify/require"
)

// The specs block spends its own share like every counted block, and its
// per-entry excerpt allowance is that block's allowance divided by the entries
// kept, never below the floor and not clamped to the entry count.
func TestDecideRenderCount_SpecDimensions(t *testing.T) {
	t.Parallel()
	decide := func(t *testing.T, tune func(*config.WorkingConfig), dim string, total, avg, budget int) int {
		t.Helper()
		spans := config.DefaultWorkingConfig()
		tune(&spans)
		w, err := NewWorkingSet(t.TempDir(), "specs", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		n, err := w.DecideRenderCount(t.Context(), dim, "s.go", total, avg, budget)
		require.NoError(t, err)
		return n
	}
	t.Run("count spends the specs share", func(t *testing.T) {
		// 10% of 1000 is 100 bytes: 100/30 = 3 of ten entries.
		require.Equal(t, 3, decide(t, func(c *config.WorkingConfig) { c.HolographicSpecsSharePercent = 10 }, HolographicDimSpecs, 10, 30, 1000))
		// 50% holds all ten.
		require.Equal(t, 10, decide(t, func(c *config.WorkingConfig) { c.HolographicSpecsSharePercent = 50 }, HolographicDimSpecs, 10, 30, 1000))
	})
	t.Run("excerpt floor wins under the division", func(t *testing.T) {
		// 10% of 1000 is 100 bytes over 4 entries is 25 runes; the floor is 60.
		require.Equal(t, 60, decide(t, func(c *config.WorkingConfig) {
			c.HolographicSpecsSharePercent = 10
			c.HolographicSpecExcerptFloor = 60
		}, HolographicDimSpecExcerpt, 4, 1, 1000))
	})
	t.Run("excerpt division wins over the floor and is not clamped to the count", func(t *testing.T) {
		// 40% of 1000 is 400 bytes over 2 entries is 200 runes, above the 60 floor.
		require.Equal(t, 200, decide(t, func(c *config.WorkingConfig) {
			c.HolographicSpecsSharePercent = 40
			c.HolographicSpecExcerptFloor = 60
		}, HolographicDimSpecExcerpt, 2, 1, 1000))
	})
	t.Run("no entries has no allowance", func(t *testing.T) {
		spans := config.DefaultWorkingConfig()
		w, err := NewWorkingSet(t.TempDir(), "none", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		_, err = w.DecideRenderCount(t.Context(), HolographicDimSpecExcerpt, "s.go", 0, 1, 1000)
		require.Error(t, err)
	})
}
