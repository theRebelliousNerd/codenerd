package prompt

import (
	"context"
	"database/sql"
	"errors"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

const generationTestSchema = `
	CREATE TABLE prompt_atoms (
		atom_id TEXT PRIMARY KEY,
		version INTEGER NOT NULL,
		content TEXT NOT NULL,
		token_count INTEGER NOT NULL,
		content_hash TEXT NOT NULL,
		description TEXT,
		content_concise TEXT,
		content_min TEXT,
		category TEXT NOT NULL,
		subcategory TEXT,
		importance INTEGER,
		is_mandatory BOOLEAN NOT NULL,
		is_exclusive TEXT,
		created_at DATETIME NOT NULL
	);
	CREATE TABLE atom_context_tags (
		atom_id TEXT NOT NULL,
		dimension TEXT NOT NULL,
		tag TEXT NOT NULL
	);`

func insertProjectAtom(t *testing.T, db *sql.DB, id string) {
	t.Helper()
	_, err := db.Exec(`INSERT INTO prompt_atoms
		(atom_id, version, content, token_count, content_hash, description,
		 content_concise, content_min, category, subcategory, importance,
		 is_mandatory, is_exclusive, created_at)
		VALUES (?, 1, ?, 5, ?, '', '', '', 'context', '', 3, 0, '', CURRENT_TIMESTAMP)`,
		id, "project content "+id, "hash-"+id)
	require.NoError(t, err)
}

// generationCompiler is cacheProbeCompiler plus a project corpus database,
// returned with a second, independent handle on the same file: the writer
// stands in for `nerd prompt sync` or another process.
func generationCompiler(t *testing.T) (*JITPromptCompiler, *sql.DB) {
	t.Helper()
	compiler := cacheProbeCompiler(t)
	path := filepath.Join(t.TempDir(), "corpus.db")
	writer, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = writer.Close() })
	_, err = writer.Exec(generationTestSchema)
	require.NoError(t, err)
	insertProjectAtom(t, writer, "project/one")
	require.NoError(t, compiler.RegisterDB("project", path))
	return compiler, writer
}

func generationContext() *CompilationContext {
	return NewCompilationContext().WithTokenBudget(10000, 1000)
}

func TestSourceVector_InPlaceProjectWriteIsNeverServedStale(t *testing.T) {
	compiler, writer := generationCompiler(t)
	ctx := context.Background()

	first, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.Equal(t, 1, first.Stats.ProjectAtoms)
	second, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.True(t, second.Stats.CacheHit, "unchanged sources must be served from the cache")

	// An in-place write through another connection: no registration, no
	// cache clear, same context, same kernel.
	insertProjectAtom(t, writer, "project/two")

	third, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.False(t, third.Stats.CacheHit, "a changed project corpus must not be served from the old generation")
	require.Equal(t, 2, third.Stats.ProjectAtoms)
}

func TestSourceVector_CatalogReusesTypedAtomsWhileGenerationHolds(t *testing.T) {
	compiler, writer := generationCompiler(t)
	ctx := context.Background()

	_, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	// A different context is a miss, but the project corpus is unchanged:
	// its atoms must come from the catalog, not a second 10,000-row join.
	_, err = compiler.Compile(ctx, generationContext().WithTokenBudget(12000, 1000))
	require.NoError(t, err)
	require.EqualValues(t, 1, compiler.GetStats().CatalogReuses)

	insertProjectAtom(t, writer, "project/three")
	res, err := compiler.Compile(ctx, generationContext().WithTokenBudget(14000, 1000))
	require.NoError(t, err)
	require.Equal(t, 2, res.Stats.ProjectAtoms, "a stale catalog must not outlive its generation")
	require.EqualValues(t, 1, compiler.GetStats().CatalogReuses)
}

func TestSourceVector_DegradedCompileIsNotCached(t *testing.T) {
	compiler, writer := generationCompiler(t)
	ctx := context.Background()
	// The tag join fails without its table: the project source degrades.
	_, err := writer.Exec(`DROP TABLE atom_context_tags`)
	require.NoError(t, err)

	for i := range 2 {
		res, err := compiler.Compile(ctx, generationContext())
		require.NoError(t, err, "a degraded optional source is not an outage")
		require.False(t, res.Stats.CacheHit, "compile %d: a degraded prompt must not be cached", i)
		require.Contains(t, res.Stats.DegradedSources, "project")
		require.Contains(t, res.Stats.Uncacheable, "degraded")
	}

	// Recovery: the source is healthy again and the next result is cached.
	_, err = writer.Exec(`CREATE TABLE atom_context_tags (atom_id TEXT NOT NULL, dimension TEXT NOT NULL, tag TEXT NOT NULL)`)
	require.NoError(t, err)
	recovered, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.Empty(t, recovered.Stats.DegradedSources)
	again, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.True(t, again.Stats.CacheHit)
	require.EqualValues(t, 2, compiler.GetStats().Degraded)
}

// churningStrategyProvider changes its own generation while a compile is
// reading it: the shape of a refinement committing mid-compile.
type churningStrategyProvider struct {
	generation atomic.Int64
	churn      atomic.Bool
	fail       bool
}

func (p *churningStrategyProvider) SourceGeneration(context.Context) (string, error) {
	if p.fail {
		return "", errors.New("strategy store unreadable")
	}
	return strconv.FormatInt(p.generation.Load(), 10), nil
}

func (p *churningStrategyProvider) StrategyAtoms(*CompilationContext) []*PromptAtom {
	if p.churn.Load() {
		p.generation.Add(1)
	}
	return nil
}

func TestSourceVector_OldWorkCannotPopulateANewGeneration(t *testing.T) {
	compiler := cacheProbeCompiler(t)
	provider := &churningStrategyProvider{}
	provider.churn.Store(true)
	compiler.RegisterStrategyProvider(provider)
	ctx := context.Background()

	res, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.Equal(t, "source generation changed during compile", res.Stats.Uncacheable)

	// Nothing was published under either generation, so a quiet source now
	// compiles afresh and only then caches.
	provider.churn.Store(false)
	again, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.False(t, again.Stats.CacheHit)
	third, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.True(t, third.Stats.CacheHit)
	require.EqualValues(t, 1, compiler.GetStats().SourceChurn)
}

func TestSourceVector_UntrackedSourceNeitherConsultsNorPopulatesTheCache(t *testing.T) {
	compiler := cacheProbeCompiler(t)
	compiler.RegisterStrategyProvider(&churningStrategyProvider{fail: true})
	ctx := context.Background()
	for range 2 {
		res, err := compiler.Compile(ctx, generationContext())
		require.NoError(t, err)
		require.False(t, res.Stats.CacheHit)
		require.Contains(t, res.Stats.Uncacheable, "untracked source: strategy")
	}
	require.EqualValues(t, 2, compiler.GetStats().TotalCompilations)
}

func TestSourceVector_RegistrationChangesIdentityEvenForTheSameGenerations(t *testing.T) {
	compiler := cacheProbeCompiler(t)
	src := compiler.snapshotSources(generationContext())
	before := src.captureSourceVector(context.Background())
	compiler.RegisterStrategyProvider(nil)
	after := compiler.snapshotSources(generationContext()).captureSourceVector(context.Background())
	require.NotEqual(t, before.token(), after.token())
}
