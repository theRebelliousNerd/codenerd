package core

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestCandidateBounds_RejectsAMistypedGeneratedRule(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.Evaluate())

	// campaign_task's TaskID is a /string; deriving it into a /number slot
	// compiles and derives nothing.
	bad := "Decl learned_task_cost(Cost) bound [/number].\nlearned_task_cost(T) :- campaign_task(T, _, _, _, _)."
	err = k.HotLoadRule(bad)
	require.Error(t, err)
	require.True(t, strings.Contains(err.Error(), "type bounds"), err.Error())

	good := "Decl learned_task_seen(TaskID) bound [/string].\nlearned_task_seen(T) :- campaign_task(T, _, _, _, _)."
	require.NoError(t, k.HotLoadRule(good))
}

// A candidate that only reads declared predicates and declares nothing new is
// the common generated-rule shape; it must not be rejected for want of Decls.
func TestCandidateBounds_UndeclaredHeadIsInferredNotRejected(t *testing.T) {
	t.Parallel()
	k, err := NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.Evaluate())
	require.NoError(t, checkCandidateBounds(
		"learned_phase_seen(P) :- campaign_phase(P, _, _, _, _, _).", k.GetProgramInfo().Decls))
}
