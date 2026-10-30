package prompt

import (
	"database/sql"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"
)

// Every consulted KB whose rows the final prompt lacked recorded a miss: a
// row the budget left out read as a corpus gap, a failed search too, and a
// later delivery left the miss standing. Only a search that succeeded and
// found nothing records a miss now, and a delivery recovers the question.
func TestRecordKnowledgeConsumption_OnlyAnEmptySearchIsAMiss(t *testing.T) {
	db := createExpertKnowledgeDB(t, filepath.Join(t.TempDir(), "expert.db"), "guide/K", "k row", 0.5)
	compiler, err := NewJITPromptCompiler()
	require.NoError(t, err)
	t.Cleanup(func() { _ = compiler.Close() })
	compiler.RegisterShardDB("kbexpert", db)
	misses := func(query string) (int, sql.NullString) {
		var table int
		require.NoError(t, db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type = 'table' AND name = 'knowledge_misses'`).Scan(&table))
		if table == 0 {
			return 0, sql.NullString{}
		}
		var n int
		var recovered sql.NullString
		err := db.QueryRow(`SELECT misses, recovered_at FROM knowledge_misses WHERE query = ?`, query).Scan(&n, &recovered)
		if err == sql.ErrNoRows {
			return 0, recovered
		}
		require.NoError(t, err)
		return n, recovered
	}
	record := func(query string, lookup KnowledgeLookup, delivered bool) {
		result := &CompilationResult{KnowledgeLookups: map[string]KnowledgeLookup{"kbexpert": lookup}}
		if delivered {
			atom := NewPromptAtom("knowledge/k", CategoryDomain, "k row")
			atom.KnowledgeExpert, atom.KnowledgeConcept = "kbexpert", "guide/K"
			result.IncludedAtoms = []*PromptAtom{atom}
		}
		compiler.recordKnowledgeConsumption(t.Context(), NewCompilationContext().WithSemanticQuery(query, 5), result)
	}

	record("budget squeezed", KnowledgeLookup{Found: 1}, false)
	record("search failed", KnowledgeLookup{Failed: true}, false)
	for _, q := range []string{"budget squeezed", "search failed"} {
		if n, _ := misses(q); n != 0 {
			t.Fatalf("%q recorded a miss though the search found rows or did not complete", q)
		}
	}

	record("truly empty", KnowledgeLookup{}, false)
	n, recovered := misses("truly empty")
	require.Equal(t, 1, n, "a search that found nothing is a miss")
	require.False(t, recovered.Valid)

	record("truly empty", KnowledgeLookup{Found: 1}, true)
	n, recovered = misses("truly empty")
	require.Equal(t, 1, n, "the miss stays as history")
	require.True(t, recovered.Valid, "a delivery for the question recovers it")
}
