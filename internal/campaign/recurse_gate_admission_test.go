package campaign

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
)

// A cycle whose after-measurement does not cover the gates the baseline
// measured cannot be vouched for by the pair: it is reverted and the journal
// names the gate it could not compare (audit
// gate-comparison-domain-admission). The attempt itself is a clean fix.
func TestRecurseCycles_AGateMissingFromTheAfterMeasureRevertsTheCycle(t *testing.T) {
	t.Parallel()
	root := recurseFixture(t, nil)
	k, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	var calls int
	measure := func(ctx context.Context, _ []string) gates.Snapshot {
		calls++
		if calls == 1 {
			return gates.MeasureErrorFree(ctx, root, gates.MeasureOptions{Only: []string{"gofmt"}})
		}
		return gates.MeasureErrorFree(ctx, root, gates.MeasureOptions{Only: []string{"vet"}})
	}
	var progress strings.Builder
	res, err := RunRecurseCycles(context.Background(), RecurseCycleConfig{
		Workspace: root, Kernel: k, Passes: 1, Progress: &progress, MeasureGates: measure,
		Execute: ownWrites(root, onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
			write(t, root, "store/store.go", fixedStore)
			return nil
		})),
	})
	t.Logf("progress:\n%s", progress.String())
	if err != nil {
		t.Fatal(err)
	}
	if res.Kept != 0 {
		t.Fatalf("a cycle whose after-measure dropped gofmt was kept: %+v", res)
	}
	r := ratchets(journalOf(t, root))
	if len(r) != 1 || r[0].Outcome != outcomeReverted || !strings.Contains(r[0].Detail, "gofmt (missing_after)") {
		t.Fatalf("journal ratchets = %+v, want the attempt reverted naming gofmt missing_after", r)
	}
}
