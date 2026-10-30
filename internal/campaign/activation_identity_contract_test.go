package campaign

import (
	"context"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"

	"github.com/stretchr/testify/require"
)

func TestCompressPhasePreservesStringActivationIdentity(t *testing.T) {
	k, err := core.NewRealKernel()
	require.NoError(t, err)
	require.NoError(t, k.Assert(core.Fact{
		Predicate: "phase_context_atom",
		Args:      []any{"phase-identity", types.MangleString("/workspace/file.go"), int64(90)},
	}))
	pager := NewContextPager(k, nil, 100000)
	_, count, _, err := pager.CompressPhase(context.Background(), &Phase{ID: "phase-identity", Name: "Identity"})
	require.NoError(t, err)
	require.Equal(t, 1, count)
	rows, err := k.Query(`activation("/workspace/file.go", -100)`)
	require.NoError(t, err)
	require.Len(t, rows, 1)
	rows, err = k.Query(`activation(/workspace/file.go, -100)`)
	require.NoError(t, err)
	require.Empty(t, rows)
}
