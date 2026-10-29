package system

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/prompt"

	"github.com/stretchr/testify/require"
)

const factorySnapshotEvidence = "adapter_snapshot_evidence"

func newFactoryAdapterSnapshotKernel(test *testing.T) (*core.RealKernel, *KernelAdapter) {
	test.Helper()
	kernel, err := core.NewRealKernelWithWorkspace(test.TempDir())
	require.NoError(test, err)
	kernel.AppendPolicy(`
Decl adapter_snapshot_evidence(Name, Revision).
Decl adapter_snapshot_derived(Name, Revision).
adapter_snapshot_derived(Name, Revision) :- adapter_snapshot_evidence(Name, Revision).
`)
	require.NoError(test, kernel.Assert(core.Fact{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}))
	return kernel, NewKernelAdapter(kernel)
}

func fingerprint(test *testing.T, f prompt.KernelContentFingerprinter) string {
	test.Helper()
	fp, err := f.ContentFingerprint()
	require.NoError(test, err)
	require.NotEmpty(test, fp)
	return fp
}

// The adapter names the live kernel's content; a compilation scope names the
// content it cloned, keeps naming it while the live kernel moves on, and
// changes only with its own private writes. A closed scope names nothing,
// and a fresh scope names the live content again.
func TestKernelAdapterFingerprint_FollowsLiveAndScopeContent(test *testing.T) {
	kernel, adapter := newFactoryAdapterSnapshotKernel(test)
	kernelFP, err := kernel.ContentFingerprint()
	require.NoError(test, err)
	original := fingerprint(test, adapter)
	require.Equal(test, kernelFP, original)

	scope, err := adapter.NewCompilationScope()
	require.NoError(test, err)
	test.Cleanup(func() { require.NoError(test, scope.Close()) })
	scoped, ok := scope.(prompt.KernelContentFingerprinter)
	require.True(test, ok, "production compilation scope does not name its content")
	require.Equal(test, original, fingerprint(test, scoped))

	require.NoError(test, kernel.Retract(factorySnapshotEvidence))
	require.NoError(test, kernel.Assert(core.Fact{Predicate: factorySnapshotEvidence, Args: []any{"evidence B", int64(18)}}))
	live := fingerprint(test, adapter)
	require.NotEqual(test, original, live, "a changed live kernel kept its fingerprint")
	require.Equal(test, original, fingerprint(test, scoped), "the scope's fingerprint followed the live kernel")

	require.NoError(test, scope.AssertBatch([]any{`adapter_snapshot_evidence("private evidence", 19).`}))
	private := fingerprint(test, scoped)
	require.NotEqual(test, original, private, "a private scope write kept the scope's fingerprint")
	require.Equal(test, live, fingerprint(test, adapter), "a private scope write reached the live kernel")

	require.NoError(test, scope.Close())
	_, err = scoped.ContentFingerprint()
	require.ErrorContains(test, err, "nil adapter")

	fresh, err := adapter.NewCompilationScope()
	require.NoError(test, err)
	test.Cleanup(func() { require.NoError(test, fresh.Close()) })
	require.Equal(test, live, fingerprint(test, fresh.(prompt.KernelContentFingerprinter)))

	// Putting the original row back restores the original content, and so
	// the original fingerprint, whatever route led there.
	require.NoError(test, kernel.Retract(factorySnapshotEvidence))
	require.NoError(test, kernel.Assert(core.Fact{Predicate: factorySnapshotEvidence, Args: []any{"evidence A", int64(17)}}))
	require.Equal(test, original, fingerprint(test, adapter))
}

type factorySnapshotOpaqueBackend struct{ core.Kernel }

// Only a RealKernel can name its content; anything else is an error, never an
// identity that could match a different state.
func TestKernelAdapterFingerprint_NilAndUnsupportedKernels(test *testing.T) {
	var nilAdapter *KernelAdapter
	var nilRealKernel *core.RealKernel
	kernel, _ := newFactoryAdapterSnapshotKernel(test)
	for name, adapter := range map[string]*KernelAdapter{
		"nil adapter":           nilAdapter,
		"nil kernel":            NewKernelAdapter(nil),
		"typed nil real kernel": NewKernelAdapter(nilRealKernel),
		"opaque kernel":         NewKernelAdapter(&factorySnapshotOpaqueBackend{Kernel: kernel}),
	} {
		test.Run(name, func(test *testing.T) {
			fp, err := adapter.ContentFingerprint()
			require.Error(test, err)
			require.Empty(test, fp)
		})
	}
}
