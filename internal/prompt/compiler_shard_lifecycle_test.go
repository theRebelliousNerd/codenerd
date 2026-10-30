package prompt

import (
	"context"
	"database/sql"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func shardLifecycleDB(t *testing.T) (*sql.DB, string) {
	t.Helper()
	path := filepath.Join(t.TempDir(), "expert.db")
	db, err := sql.Open("sqlite3", path)
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })
	require.NoError(t, NewAtomLoader(nil).EnsureSchema(t.Context(), db))
	return db, path
}

func TestShardDBLeaseBorrowsOwnerWithoutChangingSourceIdentity(t *testing.T) {
	compiler := bridgeCompiler(t, nil, nil)
	db, path := shardLifecycleDB(t)
	require.NoError(t, compiler.RegisterShardDB("Coder", db))
	epoch := compiler.sourceEpoch.Load()
	release, err := CreateJITDBRegistrar(compiler)(" /CODER ", path)
	require.NoError(t, err)
	t.Cleanup(release)
	current, ok := compiler.LookupShardDB("coder")
	require.True(t, ok)
	require.Same(t, db, current)
	release()
	release()
	current, ok = compiler.LookupShardDB("coder")
	require.True(t, ok)
	require.Same(t, db, current)
	require.NoError(t, db.Ping())
	require.Equal(t, epoch, compiler.sourceEpoch.Load(), "borrowing must not churn source identity or clear the prompt cache")
}

func TestShardDBTemporaryLeasesSharePoolUntilLastCompletion(t *testing.T) {
	compiler := bridgeCompiler(t, nil, nil)
	writer, path := shardLifecycleDB(t)
	require.NoError(t, writer.Close())
	first, err := compiler.AcquireShardDB("coder", path)
	require.NoError(t, err)
	t.Cleanup(first)
	db, ok := compiler.LookupShardDB("coder")
	require.True(t, ok)
	epoch := compiler.sourceEpoch.Load()
	second, err := compiler.AcquireShardDB("/CODER", path)
	require.NoError(t, err)
	t.Cleanup(second)
	first()
	current, ok := compiler.LookupShardDB("coder")
	require.True(t, ok)
	require.Same(t, db, current)
	require.NoError(t, db.Ping())
	require.Equal(t, epoch, compiler.sourceEpoch.Load())
	second()
	_, ok = compiler.LookupShardDB("coder")
	require.False(t, ok)
	require.Error(t, db.Ping(), "the final temporary owner must close its pool")
}

func TestShardDBOldLeaseCannotRetireReplacementOrSibling(t *testing.T) {
	compiler := bridgeCompiler(t, nil, nil)
	old, path := shardLifecycleDB(t)
	replacement, _ := shardLifecycleDB(t)
	sibling, _ := shardLifecycleDB(t)
	require.NoError(t, compiler.RegisterShardDB("coder", old))
	require.NoError(t, compiler.RegisterShardDB("sibling", sibling))
	release, err := compiler.AcquireShardDB("coder", path)
	require.NoError(t, err)
	t.Cleanup(release)
	require.NoError(t, compiler.RegisterShardDB("CODER", replacement))
	require.NoError(t, old.Ping(), "an acquired generation remains usable until release")
	release()
	require.Error(t, old.Ping())
	current, ok := compiler.LookupShardDB("coder")
	require.True(t, ok)
	require.Same(t, replacement, current)
	current, ok = compiler.LookupShardDB("sibling")
	require.True(t, ok)
	require.Same(t, sibling, current)
	require.NoError(t, replacement.Ping())
	require.NoError(t, sibling.Ping())
}

func TestShardDBOwnerRemovalKeepsBorrowedGenerationUntilRelease(t *testing.T) {
	compiler := bridgeCompiler(t, nil, nil)
	db, path := shardLifecycleDB(t)
	require.NoError(t, compiler.RegisterShardDB("coder", db))
	release, err := compiler.AcquireShardDB("coder", path)
	require.NoError(t, err)
	t.Cleanup(release)
	compiler.UnregisterShardDB("coder")
	_, ok := compiler.LookupShardDB("coder")
	require.False(t, ok)
	require.NoError(t, db.Ping())
	release()
	require.Error(t, db.Ping())
}

func TestShardDBReplacementWaitsForCanceledCompileWork(t *testing.T) {
	for _, pushed := range []bool{false, true} {
		t.Run(map[bool]string{false: "speaker", true: "pushed expert"}[pushed], func(t *testing.T) {
			compiler := cacheProbeCompiler(t)
			old, _ := shardLifecycleDB(t)
			replacement, _ := shardLifecycleDB(t)
			require.NoError(t, NewAtomLoader(nil).StoreAtom(t.Context(), old, &PromptAtom{
				ID: "context/slow", Category: CategoryContext, Content: "Hold this source reader during replacement.", TokenCount: 9,
			}))
			require.NoError(t, compiler.RegisterShardDB("coder", old))
			search := &configUpdateBarrierSearcher{entered: make(chan struct{}), release: make(chan struct{})}
			compiler.vectorSearcher = search
			compiler.selector.SetVectorSearcher(search)
			config := compiler.GetConfig()
			config.VectorSearchTimeout = time.Minute
			compiler.SetConfig(config)
			// Cleanup must release the worker before the compiler waits for it.
			t.Cleanup(func() {
				select {
				case <-search.release:
				default:
					close(search.release)
				}
			})
			cc := generationContext().WithSemanticQuery("hold source readers", 1)
			if pushed {
				cc.Experts = []string{"coder"}
			} else {
				cc.ShardID = "coder"
			}
			ctx, cancel := context.WithCancel(t.Context())
			defer cancel()
			done := make(chan error, 1)
			go func() { _, err := compiler.Compile(ctx, cc); done <- err }()
			select {
			case <-search.entered:
			case <-time.After(5 * time.Second):
				t.Fatal("compile did not reach vector search")
			}
			require.NoError(t, compiler.RegisterShardDB("coder", replacement))
			require.NoError(t, old.Ping(), "replacement closed an active source snapshot")
			cancel()
			select {
			case err := <-done:
				require.ErrorIs(t, err, context.Canceled)
			case <-time.After(5 * time.Second):
				t.Fatal("canceled compile waiter did not return")
			}
			require.NoError(t, old.Ping(), "caller cancellation must not revoke singleflight's source handles")
			close(search.release)
			// The production completion goroutine releases the source pin.
			require.Eventually(t, func() bool { return old.Ping() != nil }, 5*time.Second, time.Millisecond)
			require.NoError(t, replacement.Ping())
		})
	}
}

func TestShardDBShutdownClosesRetiredAndTemporaryPools(t *testing.T) {
	compiler := bridgeCompiler(t, nil, nil)
	old, path := shardLifecycleDB(t)
	require.NoError(t, compiler.RegisterShardDB("coder", old))
	releaseOld, err := compiler.AcquireShardDB("coder", path)
	require.NoError(t, err)
	t.Cleanup(releaseOld)
	replacement, _ := shardLifecycleDB(t)
	require.NoError(t, compiler.RegisterShardDB("coder", replacement))
	releaseTemporary, err := compiler.AcquireShardDB("temporary", path)
	require.NoError(t, err)
	t.Cleanup(releaseTemporary)
	temporary, _ := compiler.LookupShardDB("temporary")
	require.NoError(t, compiler.Close())
	for _, db := range []*sql.DB{old, replacement, temporary} {
		require.Error(t, db.Ping())
	}
	releaseOld()
	releaseTemporary()
	_, err = compiler.AcquireShardDB("late", path)
	require.Error(t, err)
	require.Empty(t, compiler.shardDBs)
	require.Empty(t, compiler.shardHandles)
}
