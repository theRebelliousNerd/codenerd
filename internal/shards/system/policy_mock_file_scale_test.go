package system

import (
	"fmt"
	"testing"
	"time"

	"codenerd/internal/core"
)

// TestPolicyMockCoverage_IsLinearAtRepoScale pins the shape of the test-
// coverage derivation in codedom_core.mg.
//
// History: the first mock_file rule paired every test file with every source
// file repo-wide (~500K facts, "fact size limit reached ... 500423 > 500000",
// kernel left with zero world facts). The second bounded it per directory,
// but still materialised every (test, source) pair — ~31K facts on this
// codebase — and once the world shard began evaluating on every turn (item
// 55, 2026-09-04) that join alone cost 17 s of a 24.5 s evaluation. The
// only consumer needs "does this source file's directory hold a test", so
// the pairs are gone: dir_has_go_test/1 is one fact per directory and
// source_has_test_in_dir/1 one per source file, both linear in the scan.
func TestPolicyMockCoverage_IsLinearAtRepoScale(t *testing.T) {
	const (
		dirs          = 40
		testsPerDir   = 13 // 520 test files
		sourcesPerDir = 30 // 1200 source files
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
	// One directory with sources and no test: its sources must not be covered.
	for i := range sourcesPerDir {
		addFile(fmt.Sprintf("internal/untested/file%02d.go", i), "internal/untested", false)
	}

	start := time.Now()
	if err := kernel.AssertBatch(facts); err != nil {
		t.Fatalf("loading %d scan facts failed: %v", len(facts), err)
	}
	dirsWithTests, err := kernel.Query("dir_has_go_test")
	if err != nil {
		t.Fatalf("Query dir_has_go_test: %v", err)
	}
	covered, err := kernel.Query("source_has_test_in_dir")
	if err != nil {
		t.Fatalf("Query source_has_test_in_dir: %v", err)
	}
	elapsed := time.Since(start)

	if len(dirsWithTests) != dirs {
		t.Errorf("dir_has_go_test derived %d, want %d (one per directory holding a test)", len(dirsWithTests), dirs)
	}
	if want := dirs * sourcesPerDir; len(covered) != want {
		t.Errorf("source_has_test_in_dir derived %d, want %d (one per source file in a tested directory; "+
			"the untested directory contributes none)", len(covered), want)
	}
	for _, f := range covered {
		if len(f.Args) == 1 {
			if s, _ := f.Args[0].(string); len(s) > 17 && s[:17] == "internal/untested" {
				t.Fatalf("source in an untested directory reported covered: %s", s)
			}
		}
	}
	if elapsed > 20*time.Second {
		t.Errorf("load+derive took %v; the coverage derivation must stay linear in the scan", elapsed)
	}
}
