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
// silently deriving nothing in production.
func TestDecideCallerLimit(t *testing.T) {
	spans := config.DefaultWorkingConfig()
	spans.HolographicCallerSharePercent = 10
	w, err := NewWorkingSet(t.TempDir(), "render", spans)
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })
	ctx := t.Context()

	// 1000 bytes at 10% is 100 bytes. Three callers at 30 bytes need 90:
	// the pool fits, so all three render.
	n, err := w.DecideCallerLimit(ctx, "fit.go", 3, 30, 1000)
	require.NoError(t, err)
	require.Equal(t, 3, n, "a pool that fits the share renders whole")

	// Ten callers at 30 bytes need 300 against the same 100-byte allowance:
	// 100/30 truncates to 3.
	n, err = w.DecideCallerLimit(ctx, "tight.go", 10, 30, 1000)
	require.NoError(t, err)
	require.Equal(t, 3, n, "a pool past the share renders what the allowance buys")

	// A render replaces the last one's facts: the tight pool above must not
	// leak into this target's count.
	n, err = w.DecideCallerLimit(ctx, "other.go", 2, 30, 1000)
	require.NoError(t, err)
	require.Equal(t, 2, n, "per-render facts must not accumulate across renders")
}

// The share the policy divides by is the config's, not a literal in the
// rule: the same pool and budget render differently under two shares.
func TestDecideCallerLimit_ShareComesFromConfig(t *testing.T) {
	decide := func(t *testing.T, share int) int {
		t.Helper()
		spans := config.DefaultWorkingConfig()
		spans.HolographicCallerSharePercent = share
		w, err := NewWorkingSet(t.TempDir(), "share", spans)
		require.NoError(t, err)
		t.Cleanup(func() { _ = w.Close() })
		n, err := w.DecideCallerLimit(t.Context(), "s.go", 10, 30, 1000)
		require.NoError(t, err)
		return n
	}
	// 10% of 1000 is 100 bytes: 100/30 = 3.
	require.Equal(t, 3, decide(t, 10))
	// 50% of 1000 is 500 bytes: ten callers need 300, so all render.
	require.Equal(t, 10, decide(t, 50))
}
