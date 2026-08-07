package system

import (
	"fmt"
	"testing"

	"codenerd/internal/core"
)

// TestPolicyMockFile_SurvivesRepoScaleScan is the regression for the world-scan
// abort that left the kernel with zero facts:
//
//	Failed to load scan facts: failed to evaluate program:
//	fact size limit reached "mock_file(TestFile,SourceFile)" 500423 > 500000
//
// The cause was premise ORDER, not the rule's logic. Mangle evaluates premises
// strictly left-to-right and checks the fact limit against the intermediate
// solution set (engine/seminaivebottomup.go:645). With the two file_topology
// scans adjacent, every test file paired with every source file repo-wide
// before the file_dir premises could bound it.
//
// This fixture mirrors codeNERD's own shape closely enough to reproduce that:
// ~500 test files and ~940 source files spread over 40 directories. Misordered,
// the intermediate join is ~470k and evaluation aborts; correctly ordered, the
// peak stays at per-directory scale and the final relation is small.
func TestPolicyMockFile_SurvivesRepoScaleScan(t *testing.T) {
	// Sized so the MISORDERED join (all tests x all sources, repo-wide) exceeds
	// the engine's 500,000 intermediate-solution limit: 520 x 1200 = 624,000.
	// The correctly ordered join never leaves per-directory scale
	// (40 dirs x 13 tests x 43 files-in-dir = ~22k), so it stays far under.
	const (
		dirs           = 40
		testsPerDir    = 13 // 520 test files
		sourcesPerDir  = 30 // 1200 source files
		wantPerDirPair = testsPerDir * sourcesPerDir
	)

	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}

	facts := make([]core.Fact, 0, dirs*(testsPerDir+sourcesPerDir)*2)
	addFile := func(path, dir string, isTest bool) {
		flag := "/false"
		if isTest {
			flag = "/true"
		}
		facts = append(facts,
			core.Fact{
				Predicate: "file_topology",
				// file_topology(Path, Hash, Lang, ModTime, IsTest)
				Args: []any{path, "hash-" + path, core.MangleAtom("/go"), int64(1), core.MangleAtom(flag)},
			},
			core.Fact{Predicate: "file_dir", Args: []any{path, dir}},
		)
	}

	for d := range dirs {
		dir := fmt.Sprintf("internal/pkg%02d", d)
		for i := range testsPerDir {
			addFile(fmt.Sprintf("%s/file%02d_test.go", dir, i), dir, true)
		}
		for i := range sourcesPerDir {
			addFile(fmt.Sprintf("%s/file%02d.go", dir, i), dir, false)
		}
	}

	// A single batch, the way a world scan loads: this is where the abort hit.
	if err := kernel.AssertBatch(facts); err != nil {
		t.Fatalf("loading %d scan facts failed (this is the regression): %v", len(facts), err)
	}

	derived, err := kernel.Query("mock_file")
	if err != nil {
		t.Fatalf("Query mock_file: %v", err)
	}

	// Every pair must be same-directory, and the total must be the per-directory
	// product -- not the repo-wide one. If a future edit drops the file_dir join
	// entirely the count balloons to dirs^2 times this, which this pins.
	want := dirs * wantPerDirPair
	if len(derived) != want {
		t.Errorf("mock_file derived %d pairs, want %d (per-directory product); "+
			"a much larger number means the file_dir join was lost", len(derived), want)
	}
}
