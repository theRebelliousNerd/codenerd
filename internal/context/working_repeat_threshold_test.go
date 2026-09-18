package context

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The span that makes a repeated trace a cycle is a fact in working_set.mg,
// beside working_nudge_rounds, working_commit_rounds, working_finalize_rounds
// and working_stall_rounds — not a Go constant and not
// core_limits.tool_loop_repeat_threshold, which is where it lived until
// 2026-09-18. The loop is the only side that can see the tool trace, so it
// does the measuring; the span it measures against is policy's.
func TestRepeatThreshold_IsAFact(t *testing.T) {
	w, err := NewWorkingSet(nil, t.TempDir(), "threshold")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	got, err := w.RepeatThreshold(t.Context())
	require.NoError(t, err)
	require.Equal(t, 2, got, "the policy's working_repeat_threshold")

	// It is declared and given a value in the policy corpus itself, in the
	// same block as the four spans it belongs with. A threshold that only
	// exists because Go passed a number in is not policy.
	require.Contains(t, workingSetPolicy, "Decl working_repeat_threshold(N) bound [/number].")
	require.Contains(t, workingSetPolicy, "working_repeat_threshold(2).")
}
