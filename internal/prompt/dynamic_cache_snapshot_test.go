package prompt_test

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/prompt"
	"codenerd/internal/system"

	"github.com/stretchr/testify/require"
)

type dynamicScopeProbe struct {
	adapter       *system.KernelAdapter
	opened        atomic.Int64
	closed        atomic.Int64
	liveQueries   atomic.Int64
	earlyClose    atomic.Int64
	doubleClose   atomic.Int64
	firstReleased atomic.Bool
	failure       atomic.Int32
	captured      chan int64
	snapshotted   chan int64
	acquireGate   <-chan struct{}
}

func (probe *dynamicScopeProbe) Query(predicate string) ([]prompt.Fact, error) {
	probe.liveQueries.Add(1)
	return probe.adapter.Query(predicate)
}

func (probe *dynamicScopeProbe) AssertBatch(facts []any) error {
	return probe.adapter.AssertBatch(facts)
}

func (probe *dynamicScopeProbe) NewCompilationScope() (prompt.KernelCompilationScope, error) {
	if probe.failure.Load() == 5 {
		return nil, errors.New("dynamic scope acquisition failure")
	}
	scope, err := probe.adapter.NewCompilationScope()
	if err != nil {
		return nil, err
	}
	sequence := probe.opened.Add(1)
	probe.captured <- sequence
	if probe.acquireGate != nil {
		<-probe.acquireGate
	}
	return &dynamicScopeReceipt{scope: scope, probe: probe, sequence: sequence}, nil
}

type dynamicScopeReceipt struct {
	scope    prompt.KernelCompilationScope
	probe    *dynamicScopeProbe
	sequence int64
	active   atomic.Int64
	isClosed atomic.Bool
}

func (scope *dynamicScopeReceipt) Query(predicate string) ([]prompt.Fact, error) {
	scope.active.Add(1)
	defer scope.active.Add(-1)
	if scope.isClosed.Load() {
		return nil, errors.New("dynamic scope queried after release")
	}
	failure := scope.probe.failure.Load()
	if (failure == 1 && predicate == "injectable_context") || (failure == 2 && predicate == "specialist_knowledge") {
		return nil, fmt.Errorf("dynamic query failure: %s", predicate)
	}
	return scope.scope.Query(predicate)
}

func (scope *dynamicScopeReceipt) QueryAll() (map[string][]prompt.Fact, error) {
	scope.active.Add(1)
	defer scope.active.Add(-1)
	if scope.isClosed.Load() {
		return nil, errors.New("dynamic snapshot queried after release")
	}
	if scope.probe.failure.Load() == 3 {
		return nil, errors.New("dynamic complete snapshot failure")
	}
	snapshot, ok := scope.scope.(prompt.KernelFactSnapshotQuerier)
	if !ok {
		return nil, errors.New("production KernelAdapter scope does not expose QueryAll")
	}
	facts, err := snapshot.QueryAll()
	if err == nil {
		scope.probe.snapshotted <- scope.sequence
	}
	return facts, err
}

func (scope *dynamicScopeReceipt) AssertBatch(facts []any) error {
	scope.active.Add(1)
	defer scope.active.Add(-1)
	if scope.isClosed.Load() {
		return errors.New("dynamic scope asserted after release")
	}
	if scope.probe.failure.Load() == 4 {
		return errors.New("dynamic scope assertion failure")
	}
	return scope.scope.AssertBatch(facts)
}

func (scope *dynamicScopeReceipt) Close() error {
	if scope.isClosed.Swap(true) {
		scope.probe.doubleClose.Add(1)
		return errors.New("dynamic scope released twice")
	}
	if scope.active.Load() != 0 {
		scope.probe.earlyClose.Add(1)
	}
	err := scope.scope.Close()
	if scope.sequence == 1 {
		scope.probe.firstReleased.Store(true)
	}
	scope.probe.closed.Add(1)
	return err
}

func newDynamicSnapshotCompiler(test *testing.T, options ...prompt.CompilerOption) (*prompt.JITPromptCompiler, *core.RealKernel, *dynamicScopeProbe) {
	test.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(test.TempDir())
	require.NoError(test, err)
	corpus, err := prompt.LoadEmbeddedCorpus()
	require.NoError(test, err)
	probe := &dynamicScopeProbe{
		adapter:     system.NewKernelAdapter(kernel),
		captured:    make(chan int64, 32),
		snapshotted: make(chan int64, 32),
	}
	options = append(options, prompt.WithKernel(probe), prompt.WithEmbeddedCorpus(corpus))
	compiler, err := prompt.NewJITPromptCompiler(options...)
	require.NoError(test, err)
	test.Cleanup(func() {
		require.NoError(test, compiler.Close())
		require.Equal(test, probe.opened.Load(), probe.closed.Load(), "private scopes leaked")
		require.Zero(test, probe.earlyClose.Load(), "scope released during a query")
		require.Zero(test, probe.doubleClose.Load(), "scope released twice")
		require.Zero(test, probe.liveQueries.Load(), "compiler read the live caller kernel")
	})
	return compiler, kernel, probe
}

func dynamicCoderContext(test *testing.T) *prompt.CompilationContext {
	test.Helper()
	caller := prompt.NewCompilationContextWithBudget(1048576)
	caller.ShardID = "coder"
	caller.ShardType = "/coder"
	caller.IntentVerb = "/fix"
	caller.IntentTarget = "internal/prompt/compiler.go"
	caller.Language = "/go"
	caller.Provider = "/meta"
	caller.Model = "/muse_spark"
	caller.AvailableTools = taxonomyTools(test, "/fix")
	require.NotEmpty(test, caller.AvailableTools, "production tool derivation was vacuous")
	caller.AvailableSpecialists = "No additional specialists registered."
	return caller
}

func compileDynamicSnapshot(test *testing.T, compiler *prompt.JITPromptCompiler, caller *prompt.CompilationContext) *prompt.CompilationResult {
	test.Helper()
	result, err := compiler.Compile(test.Context(), caller)
	require.NoError(test, err)
	require.NotNil(test, result)
	require.NotEmpty(test, result.IncludedAtoms, "production selection was vacuous")
	require.NotNil(test, result.Manifest)
	require.NotNil(test, result.Stats)
	return result
}

func replaceDynamicFact(test *testing.T, kernel *core.RealKernel, predicate string, args ...any) {
	test.Helper()
	require.NoError(test, kernel.Retract(predicate))
	if len(args) != 0 {
		require.NoError(test, kernel.Assert(core.Fact{Predicate: predicate, Args: args}))
	}
}

func TestDynamicCacheSnapshot_ReplacementRetractionAndReuse(test *testing.T) {
	compiler, kernel, probe := newDynamicSnapshotCompiler(test)
	caller := dynamicCoderContext(test)
	callerHash := caller.Hash()
	replaceDynamicFact(test, kernel, "injectable_context", "coder", "snapshot evidence A: revision 17")
	first := compileDynamicSnapshot(test, compiler, caller)
	require.Contains(test, first.Prompt, "snapshot evidence A: revision 17")
	require.False(test, first.Stats.CacheHit)
	originalPrompt := first.Prompt
	first.Prompt = "caller-owned mutation"
	first.Stats.CacheHit = true
	unchanged := compileDynamicSnapshot(test, compiler, caller)
	require.True(test, unchanged.Stats.CacheHit)
	require.Equal(test, originalPrompt, unchanged.Prompt)
	require.NotSame(test, first, unchanged)
	require.NotSame(test, first.Stats, unchanged.Stats)
	require.Equal(test, int64(1), compiler.GetStats().TotalCompilations)
	require.Equal(test, probe.opened.Load(), probe.closed.Load(), "cache hit leaked its new scope")

	replaceDynamicFact(test, kernel, "injectable_context", "coder", "snapshot evidence B: revision 18")
	second := compileDynamicSnapshot(test, compiler, caller)
	require.Contains(test, second.Prompt, "snapshot evidence B: revision 18")
	require.NotContains(test, second.Prompt, "snapshot evidence A: revision 17")
	require.False(test, second.Stats.CacheHit)
	require.NotEqual(test, first.Manifest.ContextHash, second.Manifest.ContextHash)

	replaceDynamicFact(test, kernel, "injectable_context")
	retracted := compileDynamicSnapshot(test, compiler, caller)
	require.NotContains(test, retracted.Prompt, "snapshot evidence B: revision 18")
	require.NotContains(test, retracted.Prompt, "snapshot evidence A: revision 17")
	require.False(test, retracted.Stats.CacheHit)
	require.NotEqual(test, second.Manifest.ContextHash, retracted.Manifest.ContextHash)
	require.True(test, compileDynamicSnapshot(test, compiler, caller).Stats.CacheHit)
	require.Equal(test, int64(3), compiler.GetStats().TotalCompilations)
	require.Equal(test, callerHash, caller.Hash(), "compiler changed the caller context")
	require.Nil(test, caller.Kernel)
	for _, predicate := range []string{"compile_context", "compile_shard", "available_tool", "atom", "vector_hit"} {
		facts, err := kernel.Query(predicate)
		require.NoError(test, err)
		require.Empty(test, facts, "private selector assertions escaped into %s", predicate)
	}
}

func TestDynamicCacheSnapshot_SpecialistKnowledgeAndOtherFacts(test *testing.T) {
	compiler, kernel, _ := newDynamicSnapshotCompiler(test)
	caller := dynamicCoderContext(test)
	replaceDynamicFact(test, kernel, "specialist_knowledge", "coder", core.MangleAtom("/snapshot_topic"), "snapshot expertise A: lock readers")
	first := compileDynamicSnapshot(test, compiler, caller)
	require.Contains(test, first.Prompt, "snapshot expertise A: lock readers")
	replaceDynamicFact(test, kernel, "specialist_knowledge", "coder", core.MangleAtom("/snapshot_topic"), "snapshot expertise B: retain scopes")
	second := compileDynamicSnapshot(test, compiler, caller)
	require.Contains(test, second.Prompt, "snapshot expertise B: retain scopes")
	require.NotContains(test, second.Prompt, "snapshot expertise A: lock readers")
	require.False(test, second.Stats.CacheHit)
	require.NotEqual(test, first.Manifest.ContextHash, second.Manifest.ContextHash)
	replaceDynamicFact(test, kernel, "specialist_knowledge")
	retracted := compileDynamicSnapshot(test, compiler, caller)
	require.NotContains(test, retracted.Prompt, "snapshot expertise B: retain scopes")
	require.False(test, retracted.Stats.CacheHit)

	const mission = "identity/coder/mission"
	require.Contains(test, dynamicIncludedIDs(retracted), mission)
	replaceDynamicFact(test, kernel, "blocked_by_context", mission)
	blocked := compileDynamicSnapshot(test, compiler, caller)
	require.False(test, blocked.Stats.CacheHit)
	require.NotEqual(test, retracted.Manifest.ContextHash, blocked.Manifest.ContextHash)
	require.NotContains(test, dynamicIncludedIDs(blocked), mission, "non-injection kernel input did not affect production selection")
	replaceDynamicFact(test, kernel, "blocked_by_context")
	restored := compileDynamicSnapshot(test, compiler, caller)
	require.True(test, restored.Stats.CacheHit)
	require.Equal(test, retracted.Manifest.ContextHash, restored.Manifest.ContextHash)
	require.Contains(test, dynamicIncludedIDs(restored), mission)
}

func dynamicIncludedIDs(result *prompt.CompilationResult) []string {
	ids := make([]string, 0, len(result.IncludedAtoms))
	for _, atom := range result.IncludedAtoms {
		ids = append(ids, atom.ID)
	}
	return ids
}

type dynamicCompileOutcome struct {
	result *prompt.CompilationResult
	err    error
}

func startDynamicCompile(ctx context.Context, compiler *prompt.JITPromptCompiler, caller *prompt.CompilationContext) <-chan dynamicCompileOutcome {
	completed := make(chan dynamicCompileOutcome, 1)
	go func() {
		result, err := compiler.Compile(ctx, caller)
		completed <- dynamicCompileOutcome{result: result, err: err}
	}()
	return completed
}

func awaitDynamicOutcome(test *testing.T, completed <-chan dynamicCompileOutcome) dynamicCompileOutcome {
	test.Helper()
	select {
	case outcome := <-completed:
		return outcome
	case <-time.After(30 * time.Second):
		test.Fatal("dynamic compilation did not finish")
		return dynamicCompileOutcome{}
	}
}

func awaitDynamicScope(test *testing.T, captured <-chan int64) int64 {
	test.Helper()
	select {
	case sequence := <-captured:
		return sequence
	case <-time.After(30 * time.Second):
		test.Fatal("dynamic compilation did not reach its scope barrier")
		return 0
	}
}

func TestDynamicCacheSnapshot_ConcurrentIsolatedSnapshots(test *testing.T) {
	compiler, kernel, probe := newDynamicSnapshotCompiler(test)
	gate := make(chan struct{})
	var release sync.Once
	test.Cleanup(func() { release.Do(func() { close(gate) }) })
	probe.acquireGate = gate
	caller := dynamicCoderContext(test)
	replaceDynamicFact(test, kernel, "injectable_context", "coder", "isolated snapshot A")
	first := startDynamicCompile(test.Context(), compiler, caller)
	require.Equal(test, int64(1), awaitDynamicScope(test, probe.captured))
	replaceDynamicFact(test, kernel, "injectable_context", "coder", "isolated snapshot B")
	second := startDynamicCompile(test.Context(), compiler, caller)
	require.Equal(test, int64(2), awaitDynamicScope(test, probe.captured))
	replaceDynamicFact(test, kernel, "injectable_context", "coder", "live snapshot C")
	release.Do(func() { close(gate) })
	firstResult := awaitDynamicOutcome(test, first)
	secondResult := awaitDynamicOutcome(test, second)
	require.NoError(test, firstResult.err)
	require.NoError(test, secondResult.err)
	require.Contains(test, firstResult.result.Prompt, "isolated snapshot A")
	require.NotContains(test, firstResult.result.Prompt, "isolated snapshot B")
	require.NotContains(test, firstResult.result.Prompt, "live snapshot C")
	require.Contains(test, secondResult.result.Prompt, "isolated snapshot B")
	require.NotContains(test, secondResult.result.Prompt, "isolated snapshot A")
	require.NotContains(test, secondResult.result.Prompt, "live snapshot C")
	require.NotEqual(test, firstResult.result.Manifest.ContextHash, secondResult.result.Manifest.ContextHash)
	require.Equal(test, int64(2), compiler.GetStats().TotalCompilations)
}

func TestDynamicCacheSnapshot_FailureCannotReuseSuccess(test *testing.T) {
	compiler, kernel, probe := newDynamicSnapshotCompiler(test)
	caller := dynamicCoderContext(test)
	replaceDynamicFact(test, kernel, "injectable_context", "coder", "cached success evidence")
	warm := compileDynamicSnapshot(test, compiler, caller)
	for failure := int32(1); failure <= 5; failure++ {
		probe.failure.Store(failure)
		result, err := compiler.Compile(test.Context(), caller)
		require.Error(test, err, "failure mode %d returned a stale success", failure)
		require.Nil(test, result)
		require.Equal(test, probe.opened.Load(), probe.closed.Load(), "failure mode %d leaked a scope", failure)
	}
	probe.failure.Store(0)
	recovered := compileDynamicSnapshot(test, compiler, caller)
	require.True(test, recovered.Stats.CacheHit)
	require.Equal(test, warm.Prompt, recovered.Prompt)
	require.Equal(test, int64(1), compiler.GetStats().TotalCompilations)
}

func TestDynamicCacheSnapshot_CanceledBeforeAndDuringCapture(test *testing.T) {
	compiler, _, probe := newDynamicSnapshotCompiler(test)
	caller := dynamicCoderContext(test)
	alreadyCanceled, cancel := context.WithCancel(test.Context())
	cancel()
	result, err := compiler.Compile(alreadyCanceled, caller)
	require.ErrorIs(test, err, context.Canceled)
	require.Nil(test, result)
	require.Zero(test, probe.opened.Load())

	gate := make(chan struct{})
	var release sync.Once
	test.Cleanup(func() { release.Do(func() { close(gate) }) })
	probe.acquireGate = gate
	capturing, cancelCapture := context.WithCancel(test.Context())
	defer cancelCapture()
	completed := startDynamicCompile(capturing, compiler, caller)
	awaitDynamicScope(test, probe.captured)
	cancelCapture()
	release.Do(func() { close(gate) })
	outcome := awaitDynamicOutcome(test, completed)
	require.ErrorIs(test, outcome.err, context.Canceled)
	require.Nil(test, outcome.result)
	require.Equal(test, int64(1), probe.closed.Load())
	require.Zero(test, compiler.GetStats().TotalCompilations)
}

type dynamicSearchBarrier struct {
	entered chan struct{}
	release chan struct{}
	once    sync.Once
	queries chan string
}

func (search *dynamicSearchBarrier) Search(ctx context.Context, query string, _ int) ([]prompt.SearchResult, error) {
	search.queries <- query
	search.once.Do(func() { close(search.entered) })
	select {
	case <-search.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (search *dynamicSearchBarrier) EmbedQuery(context.Context, string) ([]float32, error) {
	return []float32{1}, nil
}

func TestDynamicCacheSnapshot_CanceledCallerDoesNotReleaseSharedScope(test *testing.T) {
	for _, cancelLeader := range []bool{true, false} {
		test.Run(fmt.Sprintf("cancel_leader_%t", cancelLeader), func(test *testing.T) {
			search := &dynamicSearchBarrier{entered: make(chan struct{}), release: make(chan struct{}), queries: make(chan string, 4)}
			config := prompt.DefaultCompilerConfig()
			config.VectorSearchTimeout = time.Minute
			compiler, kernel, probe := newDynamicSnapshotCompiler(test, prompt.WithVectorSearcher(search), prompt.WithConfig(config))
			var release sync.Once
			test.Cleanup(func() { release.Do(func() { close(search.release) }) })
			replaceDynamicFact(test, kernel, "injectable_context", "coder", "shared snapshot evidence")
			caller := dynamicCoderContext(test)
			caller.SemanticQuery = "original shared search"
			followerCaller := caller.Clone()
			leaderContext, cancelFirst := context.WithCancel(test.Context())
			defer cancelFirst()
			followerContext, cancelSecond := context.WithCancel(test.Context())
			defer cancelSecond()
			leader := startDynamicCompile(leaderContext, compiler, caller)
			select {
			case <-search.entered:
			case <-time.After(30 * time.Second):
				test.Fatal("production selection did not enter semantic search")
			}
			require.Equal(test, int64(1), awaitDynamicScope(test, probe.snapshotted))
			caller.SemanticQuery = "caller mutation after capture"
			caller.AvailableTools[0] = "caller_mutation"
			follower := startDynamicCompile(followerContext, compiler, followerCaller)
			require.Equal(test, int64(2), awaitDynamicScope(test, probe.snapshotted))
			canceled, surviving := follower, leader
			if cancelLeader {
				cancelFirst()
				canceled, surviving = leader, follower
			} else {
				cancelSecond()
			}
			canceledOutcome := awaitDynamicOutcome(test, canceled)
			require.ErrorIs(test, canceledOutcome.err, context.Canceled)
			require.Nil(test, canceledOutcome.result)
			require.False(test, probe.firstReleased.Load(), "caller cancellation released the scope used by shared compilation")
			release.Do(func() { close(search.release) })
			survivor := awaitDynamicOutcome(test, surviving)
			require.NoError(test, survivor.err)
			require.Contains(test, survivor.result.Prompt, "shared snapshot evidence")
			require.NotContains(test, survivor.result.Prompt, "caller_mutation")
			require.Equal(test, "original shared search", <-search.queries)
			require.Equal(test, int64(1), compiler.GetStats().TotalCompilations)
			reused := compileDynamicSnapshot(test, compiler, followerCaller)
			require.True(test, reused.Stats.CacheHit)
			require.Contains(test, reused.Prompt, "shared snapshot evidence")
		})
	}
}

func TestDynamicCacheSnapshot_CloseJoinsCanceledWork(test *testing.T) {
	search := &dynamicSearchBarrier{entered: make(chan struct{}), release: make(chan struct{}), queries: make(chan string, 4)}
	config := prompt.DefaultCompilerConfig()
	config.VectorSearchTimeout = time.Minute
	compiler, _, probe := newDynamicSnapshotCompiler(test, prompt.WithVectorSearcher(search), prompt.WithConfig(config))
	var release sync.Once
	test.Cleanup(func() { release.Do(func() { close(search.release) }) })
	caller := dynamicCoderContext(test)
	caller.SemanticQuery = "shutdown snapshot search"
	ctx, cancel := context.WithCancel(test.Context())
	defer cancel()
	completed := startDynamicCompile(ctx, compiler, caller)
	select {
	case <-search.entered:
	case <-time.After(30 * time.Second):
		test.Fatal("production selection did not reach the shutdown barrier")
	}
	cancel()
	outcome := awaitDynamicOutcome(test, completed)
	require.ErrorIs(test, outcome.err, context.Canceled)
	require.False(test, probe.firstReleased.Load())
	closed := make(chan error, 1)
	go func() { closed <- compiler.Close() }()
	select {
	case err := <-closed:
		require.NoError(test, err)
	case <-time.After(30 * time.Second):
		test.Fatal("compiler shutdown did not cancel and join private snapshot work")
	}
	require.Equal(test, int64(1), probe.opened.Load())
	require.Equal(test, probe.opened.Load(), probe.closed.Load())
	result, err := compiler.Compile(test.Context(), caller)
	require.Error(test, err)
	require.Nil(test, result)
}
