package system

import (
	"errors"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"

	"github.com/stretchr/testify/require"
)

const factorySnapshotEvidence = "adapter_snapshot_evidence"
const factorySnapshotDerived = "adapter_snapshot_derived"
const factorySnapshotEmpty = "adapter_snapshot_empty"

func newFactoryAdapterSnapshotKernel(test *testing.T) (*core.RealKernel, *KernelAdapter) {
	test.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(test.TempDir())
	require.NoError(test, err)
	kernel.AppendPolicy(`
Decl adapter_snapshot_evidence(Name, Revision).
Decl adapter_snapshot_derived(Name, Revision).
Decl adapter_snapshot_empty(Value).
adapter_snapshot_derived(Name, Revision) :- adapter_snapshot_evidence(Name, Revision).
`)
	require.NoError(test, kernel.Assert(core.Fact{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}))
	return kernel, NewKernelAdapter(kernel)
}

func TestKernelAdapterSnapshot_QueryAllPreservesRealFacts(test *testing.T) {
	kernel, adapter := newFactoryAdapterSnapshotKernel(test)
	backendFacts, err := kernel.QueryAll()
	require.NoError(test, err)
	snapshot, err := adapter.QueryAll()
	require.NoError(test, err)
	require.Len(test, snapshot, len(backendFacts))
	for predicate, rows := range backendFacts {
		converted, present := snapshot[predicate]
		require.True(test, present, "predicate %s disappeared", predicate)
		expected := make([]prompt.Fact, 0, len(rows))
		for _, fact := range rows {
			expected = append(expected, prompt.Fact{Predicate: fact.Predicate, Args: fact.Args})
		}
		require.ElementsMatch(test, expected, converted, "predicate %s changed", predicate)
	}
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}}, snapshot[factorySnapshotEvidence])
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotDerived, Args: []any{"evidence A", int64(17)}}}, snapshot[factorySnapshotDerived])
	empty, present := snapshot[factorySnapshotEmpty]
	require.True(test, present, "empty declaration was lost")
	require.Empty(test, empty)
}

type factorySnapshotRetainedBackend struct {
	core.Kernel
	facts   map[string][]core.Fact
	err     error
	queries int
}

func (backend *factorySnapshotRetainedBackend) QueryAll() (map[string][]core.Fact, error) {
	backend.queries++
	return backend.facts, backend.err
}

func TestKernelAdapterSnapshot_ReturnedContainersArePrivate(test *testing.T) {
	kernel, _ := newFactoryAdapterSnapshotKernel(test)
	backendFacts, err := kernel.QueryAll()
	require.NoError(test, err)
	backend := &factorySnapshotRetainedBackend{Kernel: kernel, facts: backendFacts}
	adapter := NewKernelAdapter(backend)
	first, err := adapter.QueryAll()
	require.NoError(test, err)
	require.Equal(test, 1, backend.queries)
	first[factorySnapshotEvidence][0].Args[0] = "caller argument mutation"
	first[factorySnapshotEvidence][0].Predicate = "caller predicate mutation"
	first[factorySnapshotEvidence] = append(first[factorySnapshotEvidence], prompt.Fact{Predicate: "caller row mutation", Args: []any{"extra"}})
	delete(first, factorySnapshotDerived)
	first["caller map mutation"] = []prompt.Fact{{Predicate: "caller map mutation"}}
	require.Equal(test, []core.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}}, backend.facts[factorySnapshotEvidence])
	require.Contains(test, backend.facts, factorySnapshotDerived)
	require.NotContains(test, backend.facts, "caller map mutation")

	retained, err := adapter.QueryAll()
	require.NoError(test, err)
	require.Equal(test, 2, backend.queries)
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}}, retained[factorySnapshotEvidence])
	backend.facts[factorySnapshotEvidence][0].Args[0] = "backend argument mutation"
	backend.facts[factorySnapshotEvidence][0].Predicate = "backend predicate mutation"
	backend.facts[factorySnapshotEvidence] = append(backend.facts[factorySnapshotEvidence], core.Fact{Predicate: "backend row mutation"})
	delete(backend.facts, factorySnapshotDerived)
	backend.facts["backend map mutation"] = []core.Fact{{Predicate: "backend map mutation"}}
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}}, retained[factorySnapshotEvidence])
	require.Contains(test, retained, factorySnapshotDerived)
	require.NotContains(test, retained, "backend map mutation")
	fresh, err := kernel.Query(factorySnapshotEvidence)
	require.NoError(test, err)
	require.Equal(test, []core.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}}, fresh)
}

func TestKernelAdapterSnapshot_ScopeRetainsCloneAndRejectsClosedQuery(test *testing.T) {
	kernel, adapter := newFactoryAdapterSnapshotKernel(test)
	scope, err := adapter.NewCompilationScope()
	require.NoError(test, err)
	test.Cleanup(func() { require.NoError(test, scope.Close()) })
	snapshotter, ok := scope.(prompt.KernelFactSnapshotQuerier)
	require.True(test, ok, "production compilation scope omitted QueryAll")
	original, err := snapshotter.QueryAll()
	require.NoError(test, err)
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}}, original[factorySnapshotEvidence])

	require.NoError(test, kernel.Retract(factorySnapshotEvidence))
	require.NoError(test, kernel.Assert(core.Fact{Predicate: factorySnapshotEvidence, Args: []any{"evidence B", int64(18)}}))
	live, err := adapter.QueryAll()
	require.NoError(test, err)
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence B", int64(18)}}}, live[factorySnapshotEvidence])
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotDerived, Args: []any{"evidence B", int64(18)}}}, live[factorySnapshotDerived])
	retained, err := snapshotter.QueryAll()
	require.NoError(test, err)
	require.Equal(test, original[factorySnapshotEvidence], retained[factorySnapshotEvidence])
	require.Equal(test, original[factorySnapshotDerived], retained[factorySnapshotDerived])

	require.NoError(test, scope.AssertBatch([]any{`adapter_snapshot_evidence("private evidence", 19).`}))
	private, err := snapshotter.QueryAll()
	require.NoError(test, err)
	require.ElementsMatch(test, []prompt.Fact{
		{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}},
		{Predicate: factorySnapshotEvidence, Args: []any{"private evidence", int64(19)}},
	}, private[factorySnapshotEvidence])
	require.Len(test, original[factorySnapshotEvidence], 1)
	live, err = adapter.QueryAll()
	require.NoError(test, err)
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence B", int64(18)}}}, live[factorySnapshotEvidence])
	require.NoError(test, scope.Close())
	closedFacts, err := snapshotter.QueryAll()
	require.ErrorContains(test, err, "nil adapter")
	require.Nil(test, closedFacts)
	require.Len(test, private[factorySnapshotEvidence], 2, "scope close invalidated detached returned evidence")
	live, err = adapter.QueryAll()
	require.NoError(test, err)
	require.Equal(test, []prompt.Fact{{Predicate: factorySnapshotEvidence, Args: []any{"evidence B", int64(18)}}}, live[factorySnapshotEvidence])

	freshScope, err := adapter.NewCompilationScope()
	require.NoError(test, err)
	test.Cleanup(func() { require.NoError(test, freshScope.Close()) })
	freshSnapshotter, ok := freshScope.(prompt.KernelFactSnapshotQuerier)
	require.True(test, ok)
	fresh, err := freshSnapshotter.QueryAll()
	require.NoError(test, err)
	require.Equal(test, live[factorySnapshotEvidence], fresh[factorySnapshotEvidence])
}

func TestKernelAdapterSnapshot_NilAndBackendErrors(test *testing.T) {
	var nilAdapter *KernelAdapter
	var nilRealKernel *core.RealKernel
	var nilRetainedBackend *factorySnapshotRetainedBackend
	for name, adapter := range map[string]*KernelAdapter{
		"nil adapter":           nilAdapter,
		"nil kernel":            NewKernelAdapter(nil),
		"typed nil real kernel": NewKernelAdapter(nilRealKernel),
		"typed nil backend":     NewKernelAdapter(nilRetainedBackend),
	} {
		test.Run(name, func(test *testing.T) {
			facts, err := adapter.QueryAll()
			require.ErrorContains(test, err, "nil")
			require.Nil(test, facts)
		})
	}
	kernel, _ := newFactoryAdapterSnapshotKernel(test)
	backendError := errors.New("snapshot backend refused evaluation")
	backend := &factorySnapshotRetainedBackend{
		Kernel: kernel,
		facts:  map[string][]core.Fact{factorySnapshotEvidence: {{Predicate: factorySnapshotEvidence, Args: []any{"partial evidence", int64(99)}}}},
		err:    backendError,
	}
	facts, err := NewKernelAdapter(backend).QueryAll()
	require.ErrorIs(test, err, backendError)
	require.ErrorContains(test, err, "query prompt kernel snapshot")
	require.Nil(test, facts, "backend failure exposed a partial success")
	require.Equal(test, 1, backend.queries)
}
