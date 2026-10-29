package campaign

import (
	"context"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/gates"
)

// Rung R7: with the error-free gates measured around each cycle, an attempt
// that fixes its finding but leaves another gate worse -- here a file gofmt
// rejects, which none of the workspace's own gates measure -- is reverted, and
// the journal names the gate. Without the measurement the same attempt is kept.
func TestRecurseCycles_AFixThatWorsensAnErrorFreeGateIsReverted(t *testing.T) {
	t.Parallel()
	fixAndLeaveUnformatted := func(root string) attemptFn {
		return onlyFixes(func(ctx context.Context, a RecurseAttempt) error {
			write(t, root, "store/store.go", fixedStore)
			write(t, root, "web/ugly.go", "package web\n\nfunc  Ugly()  {}\n")
			return nil
		})
	}
	run := func(root string, measure func(context.Context, []string) gates.Snapshot) *RecurseCycleResult {
		k, err := core.NewRealKernelWithWorkspace(t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		var progress strings.Builder
		res, err := RunRecurseCycles(context.Background(), RecurseCycleConfig{
			Workspace: root, Kernel: k, Execute: ownWrites(root, fixAndLeaveUnformatted(root)), Passes: 1, Progress: &progress,
			MeasureGates: measure,
		})
		t.Logf("progress:\n%s", progress.String())
		if err != nil {
			t.Fatal(err)
		}
		return res
	}

	measured := recurseFixture(t, nil)
	var snapshots int
	res := run(measured, func(ctx context.Context, _ []string) gates.Snapshot {
		snapshots++
		return gates.MeasureErrorFree(ctx, measured, gates.MeasureOptions{Only: []string{"gofmt"}})
	})
	if res.Kept != 0 {
		t.Fatalf("a fix that left an unformatted file was kept: %+v", res)
	}
	if snapshots != 2 {
		t.Fatalf("the gates were measured %d time(s), want 2 (before the attempt, after it)", snapshots)
	}
	r := ratchets(journalOf(t, measured))
	if len(r) != 1 || r[0].Outcome != outcomeReverted || !strings.Contains(r[0].Detail, "error-free gates worse: gofmt") {
		t.Fatalf("journal ratchets = %+v, want the attempt reverted naming gofmt", r)
	}
	if mustRead(t, measured, "store/store.go") != recurseModule["store/store.go"] {
		t.Fatal("the reverted attempt's fix is still in the tree")
	}

	unmeasured := recurseFixture(t, nil)
	if res := run(unmeasured, nil); res.Kept != 1 {
		t.Fatalf("without the error-free gates the same attempt should be kept: %+v", res)
	}
}
