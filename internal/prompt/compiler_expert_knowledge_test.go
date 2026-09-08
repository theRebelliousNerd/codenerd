package prompt

import (
	"context"
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/store"

	"github.com/stretchr/testify/require"
)

func createExpertKnowledgeDB(t *testing.T, path, concept, content string, confidence float64) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	_, err = db.Exec(`CREATE TABLE IF NOT EXISTS knowledge_atoms (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		concept TEXT NOT NULL,
		content TEXT NOT NULL,
		confidence REAL DEFAULT 1.0,
		content_hash TEXT,
		created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
		updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
	)`)
	require.NoError(t, err)
	_, err = db.Exec(
		`INSERT INTO knowledge_atoms (concept, content, confidence, content_hash) VALUES (?, ?, ?, ?)`,
		concept, content, confidence, "testhash-"+concept,
	)
	require.NoError(t, err)
	return db
}

func createProjectKnowledgeStore(t *testing.T, concept, content string) *store.LocalStore {
	t.Helper()
	ls, err := store.NewLocalStore(filepath.Join(t.TempDir(), "knowledge.db"))
	require.NoError(t, err)
	require.NoError(t, ls.StoreKnowledgeAtom(concept, content, 0.9))
	return ls
}

func bridgeCompiler(t *testing.T, project *store.LocalStore, experts map[string]*sql.DB) *JITPromptCompiler {
	t.Helper()
	compiler, err := NewJITPromptCompiler()
	require.NoError(t, err)
	if project != nil {
		compiler.SetLocalDB(project)
	}
	for id, db := range experts {
		compiler.RegisterShardDB(id, db)
	}
	t.Cleanup(func() { _ = compiler.Close() })
	return compiler
}

func bridgeContext(shardID, semanticQuery string) *CompilationContext {
	cc := NewCompilationContext()
	cc.ShardID = shardID
	cc.SemanticQuery = semanticQuery
	return cc
}

func containsKnowledgeMarker(atoms []*PromptAtom, marker string) bool {
	for _, atom := range atoms {
		if atom != nil && strings.Contains(atom.Content, marker) {
			return true
		}
	}
	return false
}

func TestExpertKnowledgeBridge_SelectedExpertRetrieved(t *testing.T) {
	expertDB := createExpertKnowledgeDB(t,
		filepath.Join(t.TempDir(), "expertbridgealpha.db"),
		"doc/registry/zephyrquark",
		"zephyrquark registry control for expertbridgealpha with quarzkrypton detail",
		0.9,
	)
	siblingDB := createExpertKnowledgeDB(t,
		filepath.Join(t.TempDir(), "siblingbeta.db"),
		"doc/cooking/unrelated",
		"unrelated sibling content about cooking with no shared tokens",
		0.9,
	)
	compiler := bridgeCompiler(t, nil, map[string]*sql.DB{
		"expertbridgealpha": expertDB,
		"siblingbeta":       siblingDB,
	})

	matched := compiler.collectKnowledgeAtoms(
		context.Background(),
		bridgeContext("expertbridgealpha", "zephyrquark registry control expertbridgealpha"),
	)
	require.NotEmpty(t, matched, "selected expert fact must be retrieved")
	require.True(t, containsKnowledgeMarker(matched, "quarzkrypton"))
	require.Contains(t, matched[0].Content, `source="expert:expertbridgealpha"`)
	require.Contains(t, matched[0].Content, `concept="doc/registry/zephyrquark"`)
	require.Contains(t, matched[0].Content, "confidence=0.9")
	require.NoError(t, expertDB.PingContext(t.Context()), "retrieval must not close its borrowed DB")
	aliased := compiler.collectKnowledgeAtoms(t.Context(), bridgeContext(" /ExpertBridgeAlpha ", "zephyrquark"))
	require.True(t, containsKnowledgeMarker(aliased, "quarzkrypton"))
	require.Equal(t, []string{"expertbridgealpha"}, aliased[0].ShardTypes)

	sibling := compiler.collectKnowledgeAtoms(
		context.Background(),
		bridgeContext("siblingbeta", "zephyrquark registry control expertbridgealpha"),
	)
	require.False(t, containsKnowledgeMarker(sibling, "quarzkrypton"),
		"sibling compile must never see the selected expert fact")

	unscoped := compiler.collectKnowledgeAtoms(
		context.Background(),
		bridgeContext("", "zephyrquark registry control expertbridgealpha"),
	)
	require.False(t, containsKnowledgeMarker(unscoped, "quarzkrypton"),
		"compile without a shard must not reach expert knowledge")

	for _, atom := range matched {
		require.Equal(t, CategoryKnowledge, atom.Category)
		require.Equal(t, 85, atom.Priority)
		require.False(t, atom.IsMandatory)
		require.Equal(t, []string{"expertbridgealpha"}, atom.ShardTypes)
	}
}

func TestExpertKnowledgeBridge_ReachesCompiledContext(t *testing.T) {
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
	cc := NewCompilationContext().WithShard("expertbridgealpha", "expertbridgealpha", "").
		WithSemanticQuery("zephyrquark", 5).WithTokenBudget(10000, 1000)
	before, err := compiler.Compile(t.Context(), cc)
	require.NoError(t, err)
	require.False(t, containsKnowledgeMarker(before.IncludedAtoms, "compiled-only-witness"),
		"negative control: a selection result cannot fabricate missing source knowledge")
	compiler.RegisterShardDB("expertbridgealpha", db)
	after, err := compiler.Compile(t.Context(), cc)
	require.NoError(t, err)
	require.True(t, containsKnowledgeMarker(after.IncludedAtoms, "compiled-only-witness"))
}

func TestExpertKnowledgeBridge_ProjectCoexistence(t *testing.T) {
	project := createProjectKnowledgeStore(t,
		"doc/project/coexistence",
		"zephyrquark project coexistence marker coexproj detail",
	)
	t.Cleanup(func() { project.Close() })
	expertDB := createExpertKnowledgeDB(t,
		filepath.Join(t.TempDir(), "expertbridgealpha.db"),
		"doc/registry/zephyrquark",
		"zephyrquark registry control for expertbridgealpha with quarzkrypton detail",
		0.9,
	)
	compiler := bridgeCompiler(t, project, map[string]*sql.DB{
		"expertbridgealpha": expertDB,
	})
	combined := compiler.collectKnowledgeAtoms(
		context.Background(),
		bridgeContext("expertbridgealpha", "zephyrquark"),
	)
	require.True(t, containsKnowledgeMarker(combined, "quarzkrypton"))
	require.True(t, containsKnowledgeMarker(combined, "coexproj"))
	require.LessOrEqual(t, len(combined), 10)
}

func TestExpertKnowledgeBridge_Cancellation(t *testing.T) {
	project := createProjectKnowledgeStore(t,
		"doc/project/coexistence",
		"zephyrquark project coexistence marker coexproj detail",
	)
	t.Cleanup(func() { project.Close() })
	expertDB := createExpertKnowledgeDB(t,
		filepath.Join(t.TempDir(), "expertbridgealpha.db"),
		"doc/registry/zephyrquark",
		"zephyrquark registry control for expertbridgealpha with quarzkrypton detail",
		0.9,
	)
	compiler := bridgeCompiler(t, project, map[string]*sql.DB{
		"expertbridgealpha": expertDB,
	})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	got := compiler.collectKnowledgeAtoms(ctx, bridgeContext("expertbridgealpha", "zephyrquark"))
	require.Empty(t, got, "canceled retrieval must stay visibly missing")
}
