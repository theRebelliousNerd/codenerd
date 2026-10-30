package prompt

import (
	"context"
	"testing"

	"codenerd/internal/core"

	"github.com/stretchr/testify/require"
)

// Exercise the real selector, its candidate producer, and the embedded policy.
// The old equal-score rule compared string IDs with integer-only <, aborting
// evaluation instead of selecting the lexical winner.
func TestSelectorEqualScoreConflictsUseOneLexicalOrder(t *testing.T) {
	for _, reverse := range []bool{false, true} {
		k, err := core.NewRealKernel()
		require.NoError(t, err)
		s := NewAtomSelector()
		s.SetKernel(&realKernelQuerier{k: k})
		s.SetVectorSearcher(fixedSearcher{"capability/a": 0.8, "capability/b": 0.8})
		atoms := []*PromptAtom{
			{ID: "identity/base", Category: CategoryIdentity, IsMandatory: true, Content: "base", TokenCount: 1},
			{ID: "capability/a", Category: CategoryCapability, Content: "a", TokenCount: 1, ConflictsWith: []string{"capability/b"}},
			{ID: "capability/b", Category: CategoryCapability, Content: "b", TokenCount: 1, ConflictsWith: []string{"capability/a"}},
		}
		if reverse {
			atoms[1], atoms[2] = atoms[2], atoms[1]
		}
		cc := NewCompilationContext()
		cc.SemanticQuery = "select capabilities"
		cc.SemanticTopK = 2
		selected, err := s.SelectAtoms(context.Background(), atoms, cc)
		require.NoError(t, err)
		ids := map[string]bool{}
		for _, atom := range selected {
			ids[atom.Atom.ID] = true
		}
		require.True(t, ids["capability/a"], "lexical winner must survive: %v", ids)
		require.False(t, ids["capability/b"], "equal-score loser must be suppressed: %v", ids)
		rows, err := k.Query(`beats("capability/a", "capability/b")`)
		require.NoError(t, err)
		require.Len(t, rows, 1)
		rows, err = k.Query(`atom_ordinal("identity/base", 2)`)
		require.NoError(t, err)
		require.Len(t, rows, 1, "ordinal order spans skeleton and flesh")
	}
}
