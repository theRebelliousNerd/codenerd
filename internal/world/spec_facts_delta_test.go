package world

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"codenerd/internal/core"
)

func specRowsByPredicate(t *testing.T, k *core.RealKernel) map[string][]string {
	t.Helper()
	out := make(map[string][]string)
	for _, pred := range SpecPredicates {
		out[pred] = factRows(t, k, pred)
	}
	return out
}

// A measurement retaken after one edit reaches the kernel as the rows that
// changed. Replacing every spec predicate told the kernel the whole pairing
// had changed -- 151k changed rows for 2 on this repository, a second or more
// of evaluation after every edit -- and the kernel must end with exactly the
// rows a whole replacement leaves.
func TestSpecFacts_ARetakenMeasurementAppliesAsItsDifference(t *testing.T) {
	ctx := context.Background()
	old := time.Now().Add(-48 * time.Hour).Truncate(time.Second)
	recent := time.Now().Add(-time.Hour).Truncate(time.Second)
	files := map[string]string{
		"go.mod":                    "module example.com/ws\n\ngo 1.26\n",
		"internal/core/foo.go":      specGoFile,
		"internal/core/foo_test.go": "package core\n\nimport \"testing\"\n\nfunc TestEval(t *testing.T) { var k Kernel; k.Eval(); Func() }\n",
		"Docs/guide.md":             specGuide,
		"Docs/specs/kernel.mg":      specRequirements,
	}
	root := specWorkspace(t, files, old, recent)
	kernel, err := core.NewRealKernelWithWorkspace(root)
	if err != nil {
		t.Fatal(err)
	}
	scanner := NewScanner()
	first, err := scanner.Refresh(ctx, root, nil, kernel, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil || !first.SpecMeasured {
		t.Fatalf("first scan: measured=%v err=%v", first != nil && first.SpecMeasured, err)
	}
	total := 0
	for _, rows := range specRowsByPredicate(t, kernel) {
		total += len(rows)
	}
	if total < 10 {
		t.Fatalf("the fixture measured only %d spec rows", total)
	}

	// One new exported function: the pairing is measured again, whole.
	path := filepath.Join(root, "internal", "core", "foo.go")
	if err := os.WriteFile(path, []byte(specGoFile+"\n// Added is new.\nfunc Added() {}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second, err := scanner.ScanWorkspaceIncremental(ctx, root, nil, IncrementalOptions{})
	if err != nil || !second.SpecMeasured {
		t.Fatalf("second scan: measured=%v err=%v", second != nil && second.SpecMeasured, err)
	}

	// The reference: a kernel given the same measurement by whole replacement.
	reference, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if err := reference.LoadFacts(toTypesFacts(second.SpecFacts)); err != nil {
		t.Fatal(err)
	}

	before := kernel.FactCount()
	if err := applySpecFacts(kernel, second); err != nil {
		t.Fatal(err)
	}
	if _, err := kernel.Query(SpecPredicates[0]); err != nil {
		t.Fatal(err)
	}
	got, want := specRowsByPredicate(t, kernel), specRowsByPredicate(t, reference)
	for _, pred := range SpecPredicates {
		if !slices.Equal(got[pred], want[pred]) {
			t.Fatalf("%s: the kernel holds %d rows, a whole replacement leaves %d\n got %v\nwant %v", pred, len(got[pred]), len(want[pred]), got[pred], want[pred])
		}
	}
	changed := kernel.LastEvaluation().ChangedRows
	if changed == 0 || changed >= total {
		t.Fatalf("the kernel saw %d changed rows for one added function (the pairing has %d rows, %d facts before): the measurement was not applied as its difference", changed, total, before)
	}

	// The same measurement again changes nothing.
	if err := applySpecFacts(kernel, second); err != nil {
		t.Fatal(err)
	}
	if kernel.IsDirty() {
		t.Fatal("applying an unchanged measurement dirtied the kernel")
	}
}
