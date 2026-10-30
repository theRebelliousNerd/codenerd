package world

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"testing"

	"codenerd/internal/core"
)

func commitFile(t *testing.T, dir, rel, message string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	prev, _ := os.ReadFile(path)
	if err := os.WriteFile(path, append(prev, []byte("x\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"add", rel}, {"commit", "-q", "-m", message}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
}

func repoFactRows(res *IncrementalResult, predicate string) []core.Fact {
	var rows []core.Fact
	for _, f := range res.RepoFacts {
		if f.Predicate == predicate {
			rows = append(rows, f)
		}
	}
	return rows
}

func TestScanner_MeasuresRepositoryFactsOnlyWhenTheHistoryMoves(t *testing.T) {
	ctx := context.Background()
	repo, _ := setupGitRepo(t)
	s := NewScanner()

	first, err := s.ScanWorkspaceIncremental(ctx, repo, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if !first.RepoMeasured {
		t.Fatal("the first scan of a repository must measure it")
	}
	if got := repoFactRows(first, "current_user"); len(got) != 1 || got[0].Args[0] != "Test User" {
		t.Errorf("current_user = %v, want the repository's author name", got)
	}
	if got := repoFactRows(first, "git_history"); len(got) != 2 {
		t.Errorf("git_history rows = %d, want 2", len(got))
	}

	again, err := s.ScanWorkspaceIncremental(ctx, repo, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if again != nil && again.RepoMeasured {
		t.Error("an unmoved history must not be re-measured")
	}

	commitFile(t, repo, "test.go", "third")
	moved, err := s.ScanWorkspaceIncremental(ctx, repo, nil, IncrementalOptions{SkipWhenUnchanged: true})
	if err != nil {
		t.Fatal(err)
	}
	if moved == nil || !moved.RepoMeasured {
		t.Fatal("a new commit must re-measure the repository")
	}
	if got := repoFactRows(moved, "churn_rate"); len(got) != 1 || got[0].Args[1] != int64(3) {
		t.Errorf("churn_rate = %v, want test.go touched by 3 commits", got)
	}
}

func TestScanner_NoRepositoryMeasuresNothing(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "a.go"), []byte("package a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := NewScanner().ScanWorkspaceIncremental(context.Background(), dir, nil, IncrementalOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.RepoMeasured || len(res.RepoFacts) != 0 {
		t.Errorf("a directory that is not a repository has no repository facts: %+v", res.RepoFacts)
	}
}

// Paths join file_topology, which is relative to the workspace root; a
// workspace below the repository root sees only its own files. A commit
// message and a path may hold the characters the old parser split on.
func TestScanGitHistory_PathsAndMessagesAreExact(t *testing.T) {
	repo, _ := setupGitRepo(t)
	commitFile(t, repo, "sub/dir/my file.go", "fix a|b and c")
	commitFile(t, repo, "other/x.go", "elsewhere")

	facts, err := ScanGitHistory(context.Background(), filepath.Join(repo, "sub"), 10)
	if err != nil {
		t.Fatal(err)
	}
	var paths, messages []string
	for _, f := range facts {
		if f.Predicate == "git_history" {
			paths = append(paths, f.Args[0].(string))
			messages = append(messages, f.Args[4].(string))
		}
	}
	if !slices.Equal(paths, []string{"dir/my file.go"}) {
		t.Errorf("paths = %q, want only the workspace's own file, relative to it", paths)
	}
	if !slices.Equal(messages, []string{"fix a|b and c"}) {
		t.Errorf("messages = %q, want the whole subject", messages)
	}
}

func TestApplyIncrementalResult_ReplacesRepositoryFactsWholeAndOnlyWhenMeasured(t *testing.T) {
	facts := []core.Fact{{Predicate: "current_user", Args: []any{"Me"}}}

	t.Run("measured, even when no file changed", func(t *testing.T) {
		k := &mockKernel{}
		if err := ApplyIncrementalResult(k, &IncrementalResult{Unchanged: true, RepoMeasured: true, RepoFacts: facts}); err != nil {
			t.Fatal(err)
		}
		for _, pred := range GitPredicates {
			if !slices.Contains(k.retractedPredicates, pred) {
				t.Errorf("%s must be retracted before the new measurement loads: %v", pred, k.retractedPredicates)
			}
		}
		if len(k.loadedFacts) != 1 || k.loadedFacts[0].Predicate != "current_user" {
			t.Errorf("loaded = %v, want the measured rows", k.loadedFacts)
		}
	})

	t.Run("not measured leaves the kernel's rows alone", func(t *testing.T) {
		k := &mockKernel{}
		if err := ApplyIncrementalResult(k, &IncrementalResult{Unchanged: true}); err != nil {
			t.Fatal(err)
		}
		if k.retractCalled || k.loadFactsCalled {
			t.Errorf("an unmeasured result touched the kernel: retract=%v load=%v", k.retractedPredicates, k.loadedFacts)
		}
	})
}
