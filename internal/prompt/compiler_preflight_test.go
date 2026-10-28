package prompt

import (
	"context"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

// identityKernel is a scope-providing kernel that names its state. Every
// NewCompilationScope is counted, so a test can tell a reused preflight (no
// clone) from a fresh one.
type identityKernel struct {
	mu         sync.Mutex
	generation uint64
	reusable   bool
	injected   string
	scopes     atomic.Int64
}

func (k *identityKernel) Query(string) ([]Fact, error) { return nil, nil }
func (k *identityKernel) AssertBatch([]any) error      { return nil }

func (k *identityKernel) identity() KernelSnapshotIdentity {
	k.mu.Lock()
	defer k.mu.Unlock()
	return KernelSnapshotIdentity{Instance: 7, Generation: k.generation, Reusable: k.reusable}
}

func (k *identityKernel) SnapshotIdentity() (KernelSnapshotIdentity, bool) {
	return k.identity(), true
}

func (k *identityKernel) write(injected string) {
	k.mu.Lock()
	defer k.mu.Unlock()
	k.generation++
	k.injected = injected
}

func (k *identityKernel) NewCompilationScope() (KernelCompilationScope, error) {
	k.scopes.Add(1)
	k.mu.Lock()
	defer k.mu.Unlock()
	return &identityScope{
		base:     KernelSnapshotIdentity{Instance: 7, Generation: k.generation, Reusable: k.reusable},
		injected: k.injected,
	}, nil
}

type identityScope struct {
	base     KernelSnapshotIdentity
	injected string
}

func (s *identityScope) Query(predicate string) ([]Fact, error) {
	if predicate == "injectable_context" && s.injected != "" {
		return []Fact{{Predicate: predicate, Args: []any{"/_all", s.injected}}}, nil
	}
	return nil, nil
}
func (s *identityScope) AssertBatch([]any) error { return nil }
func (s *identityScope) Close() error            { return nil }
func (s *identityScope) QueryAll() (map[string][]Fact, error) {
	return map[string][]Fact{"marker": {{Predicate: "marker", Args: []any{s.injected}}}}, nil
}
func (s *identityScope) BaseIdentity() (KernelSnapshotIdentity, bool) { return s.base, true }

func identityCompiler(t *testing.T, kernel *identityKernel) *JITPromptCompiler {
	t.Helper()
	compiler, err := NewJITPromptCompiler(
		WithEmbeddedCorpus(NewEmbeddedCorpus([]*PromptAtom{{
			ID: "identity/coder", Category: CategoryIdentity, Content: "You are the Coder Shard.",
			Priority: 100, IsMandatory: true, TokenCount: 8,
		}})),
		WithKernel(kernel),
	)
	require.NoError(t, err)
	t.Cleanup(func() { _ = compiler.Close() })
	return compiler
}

func TestPreflight_CacheHitOverAnUnchangedKernelClonesNothing(t *testing.T) {
	kernel := &identityKernel{reusable: true, injected: "first"}
	compiler := identityCompiler(t, kernel)
	ctx := context.Background()

	first, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.False(t, first.Stats.PreflightReused)
	require.EqualValues(t, 1, kernel.scopes.Load())

	second, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.True(t, second.Stats.CacheHit)
	require.EqualValues(t, 1, kernel.scopes.Load(), "a hit over an unchanged kernel must not clone it")
	require.EqualValues(t, 1, compiler.GetStats().PreflightReuses)
}

func TestPreflight_KernelWriteIsNeverServedFromTheMemo(t *testing.T) {
	kernel := &identityKernel{reusable: true, injected: "before"}
	compiler := identityCompiler(t, kernel)
	ctx := context.Background()

	first, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	kernel.write("after")

	res, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.False(t, res.Stats.CacheHit, "new kernel evidence must change the identity and the key")
	require.NotEqual(t, injectedAtomID(t, first), injectedAtomID(t, res),
		"the kernel-injected atom must be rebuilt from the new evidence")
	require.EqualValues(t, 2, kernel.scopes.Load())
}

func TestPreflight_NonReusableKernelAlwaysTakesTheFullPath(t *testing.T) {
	kernel := &identityKernel{reusable: false, injected: "volatile"}
	compiler := identityCompiler(t, kernel)
	ctx := context.Background()
	for range 3 {
		_, err := compiler.Compile(ctx, generationContext())
		require.NoError(t, err)
	}
	require.EqualValues(t, 3, kernel.scopes.Load(), "external premises: every compile must re-snapshot")
	require.Zero(t, compiler.GetStats().PreflightReuses)
}

// A memo hit whose LRU entry is gone must still select on a private scope,
// and must reuse the memo's snapshot when the scope was cloned from the same
// state.
func TestPreflight_MissAfterMemoHitAcquiresAScopeOnce(t *testing.T) {
	kernel := &identityKernel{reusable: true, injected: "steady"}
	compiler := identityCompiler(t, kernel)
	ctx := context.Background()

	first, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	compiler.clearPromptCache("test")

	res, err := compiler.Compile(ctx, generationContext())
	require.NoError(t, err)
	require.False(t, res.Stats.CacheHit)
	require.True(t, res.Stats.PreflightReused)
	require.Equal(t, injectedAtomID(t, first), injectedAtomID(t, res))
	require.EqualValues(t, 2, kernel.scopes.Load())
}

// injectedAtomID finds the kernel-injected context atom among a result's
// candidates; its ID is a hash of the injected content.
func injectedAtomID(t *testing.T, res *CompilationResult) string {
	t.Helper()
	require.NotNil(t, res.Manifest)
	for _, entry := range res.Manifest.Selected {
		if strings.HasPrefix(entry.ID, "kernel/context/") {
			return entry.ID
		}
	}
	for _, entry := range res.Manifest.Dropped {
		if strings.HasPrefix(entry.ID, "kernel/context/") {
			return entry.ID
		}
	}
	t.Fatal("no kernel-injected context atom among the candidates")
	return ""
}
