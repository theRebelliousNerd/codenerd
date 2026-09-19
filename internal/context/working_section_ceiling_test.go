package context

import (
	"testing"

	"github.com/stretchr/testify/require"
)

// The observations section's ceiling is the working policy's, not a Go
// constant: one nerd fix run on 2026-09-18 sent 32k input tokens on its first
// call and 80-103k by its last, the growth being this section filling up to a
// 256 KiB constant inherited from the old transcript cap. The policy states
// the ceiling where a reader can see and change it, and Go reads it back.
func TestWorkingSectionCeilingIsThePolicys(t *testing.T) {
	w, err := NewWorkingSet(nil, t.TempDir(), "task")
	require.NoError(t, err)
	t.Cleanup(func() { _ = w.Close() })

	ceiling, err := w.SectionCeiling(t.Context())
	require.NoError(t, err)
	require.Equal(t, 131072, ceiling, "the policy's working_section_ceiling")
	require.Less(t, ceiling, 256*1024, "the section must not be back at the inherited 256 KiB transcript cap")
}
