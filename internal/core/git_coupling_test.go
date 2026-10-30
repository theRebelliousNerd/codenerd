package core

import (
	"fmt"
	"testing"

	"codenerd/internal/types"
)

// git_history is a measurement of the repository, so one sweeping commit (a
// reformat, a vendored import, an initial import) puts every file it touched
// under one hash. co_committed_files pairs the files of a hash, so a commit of
// N files cost N*N rows: a thousand-file commit blew the evaluation's fact
// limit and took every rule of the kernel with it. A commit that large is not
// evidence that two files change together, and must cost the kernel a count,
// not a cross product.
func TestCoCommitCouplingIgnoresSweepingCommits(t *testing.T) {
	k := setupMockKernel(t)
	author := "alice"
	var facts []types.Fact
	row := func(file, hash string) {
		facts = append(facts, types.Fact{Predicate: "git_history", Args: []any{file, hash, author, int64(1000), "msg"}})
	}
	// Three small commits touched the same pair together: genuine coupling.
	for i := 0; i < 3; i++ {
		row("a.go", fmt.Sprintf("small%d", i))
		row("b.go", fmt.Sprintf("small%d", i))
	}
	// One sweeping commit touched two thousand files, including a and c.
	for i := 0; i < 2000; i++ {
		row(fmt.Sprintf("vendor/f%d.go", i), "sweep")
	}
	row("a.go", "sweep")
	row("c.go", "sweep")
	if err := k.AssertBatch(facts); err != nil {
		t.Fatalf("assert: %v", err)
	}

	coupled := mustQuery(t, k, "hidden_coupling")
	if !hasPair(coupled, "a.go", "b.go") {
		t.Errorf("a.go and b.go changed together in three small commits: %v", coupled)
	}
	if hasPair(coupled, "a.go", "c.go") {
		t.Errorf("a sweeping commit must not couple the files it touched: %v", coupled)
	}
}

func hasPair(rows []types.Fact, a, b string) bool {
	for _, f := range rows {
		if len(f.Args) >= 2 && types.ExtractString(f.Args[0]) == a && types.ExtractString(f.Args[1]) == b {
			return true
		}
	}
	return false
}
