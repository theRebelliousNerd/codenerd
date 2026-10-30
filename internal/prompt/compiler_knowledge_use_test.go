package prompt

import (
	"database/sql"
	"path/filepath"
	"testing"

	"codenerd/internal/store"

	"github.com/stretchr/testify/require"
)

// A compile that carries an expert's knowledge row records the row's use in
// the expert's KB; a compile that consulted the expert and carries none of
// its rows records the task's question as a miss. Both are what the
// knowledge refresh policy decides over (policy/knowledge_refresh.mg).
func TestCompileRecordsExpertKnowledgeUseAndMisses(t *testing.T) {
	db := createExpertKnowledgeDB(t, filepath.Join(t.TempDir(), "expert.db"),
		"guide/Z01", "zephyrquark compiled-only-witness", 0.4)
	identity := NewPromptAtom("identity/bridge", CategoryIdentity, "identity")
	identity.IsMandatory = true
	witness := knowledgeAtomToPromptAtom(store.KnowledgeAtom{
		Concept: "guide/Z01", Content: "zephyrquark compiled-only-witness", Confidence: 0.4,
	}, "expertbridgealpha", "expert:expertbridgealpha")
	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(NewEmbeddedCorpus([]*PromptAtom{identity})),
		WithKernel(&mockKernel{facts: atomsToFacts([]*PromptAtom{identity, witness})}),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = compiler.Close() })
	compiler.RegisterShardDB("expertbridgealpha", db)

	used := NewCompilationContext().WithShard("expertbridgealpha", "expertbridgealpha", "").
		WithSemanticQuery("zephyrquark", 5).WithTokenBudget(10000, 1000)
	result, err := compiler.Compile(t.Context(), used)
	require.NoError(t, err)
	require.True(t, containsKnowledgeMarker(result.IncludedAtoms, "compiled-only-witness"))
	require.Equal(t, KnowledgeLookup{Found: 1}, result.KnowledgeLookups["expertbridgealpha"])
	var retrieved int
	require.NoError(t, db.QueryRow(`SELECT retrieved FROM knowledge_usage WHERE concept = 'guide/Z01'`).Scan(&retrieved))
	require.Equal(t, 1, retrieved)

	missed := NewCompilationContext().WithShard("expertbridgealpha", "expertbridgealpha", "").
		WithSemanticQuery("xyzzyplover", 5).WithTokenBudget(10000, 1000)
	_, err = compiler.Compile(t.Context(), missed)
	require.NoError(t, err)
	var misses int
	err = db.QueryRow(`SELECT misses FROM knowledge_misses WHERE query = 'xyzzyplover'`).Scan(&misses)
	require.NoError(t, err)
	require.Equal(t, 1, misses)
	require.ErrorIs(t, db.QueryRow(`SELECT 1 FROM knowledge_misses WHERE query = 'zephyrquark'`).Scan(new(int)), sql.ErrNoRows,
		"a consultation that carried the expert's knowledge is not a miss")
}
