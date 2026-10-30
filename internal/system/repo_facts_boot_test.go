package system

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/types"
)

// The Chesterton rules read git_history, churn_rate and current_user. Only the
// campaign report ever produced the first two, and nothing the third, so on the
// production kernel no change by another author and no high-churn file ever drew
// a warning. The world scan now asserts them from the repository at boot, homed
// in the shard that evaluates the rules, and Mangle judges "recent" from the
// commit's unix time against current_time.
func TestChestertonWarningsFireFromARealRepositoryOnTheShardedKernel(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping full Cortex boot in -short mode")
	}
	ws := t.TempDir()
	now := time.Now()
	runGit(t, ws, nil, "init", "-q")
	runGit(t, ws, nil, "config", "user.name", "Me Tester")
	runGit(t, ws, nil, "config", "user.email", "me@example.test")

	commit := func(file, author string, at time.Time) {
		t.Helper()
		path := filepath.Join(ws, file)
		prev, _ := os.ReadFile(path)
		if err := os.WriteFile(path, append(prev, []byte("package x\n")...), 0o644); err != nil {
			t.Fatal(err)
		}
		runGit(t, ws, nil, "add", file)
		date := fmt.Sprintf("%d +0000", at.Unix())
		runGit(t, ws, []string{
			"GIT_AUTHOR_NAME=" + author, "GIT_AUTHOR_EMAIL=a@example.test",
			"GIT_COMMITTER_NAME=" + author, "GIT_COMMITTER_EMAIL=a@example.test",
			"GIT_AUTHOR_DATE=" + date, "GIT_COMMITTER_DATE=" + date,
		}, "commit", "-q", "-m", "change "+file)
	}
	commit("theirs.go", "Bob Other", now.Add(-time.Hour))      // recent, by another author
	commit("stale.go", "Bob Other", now.Add(-30*24*time.Hour)) // another author, long ago
	commit("mine.go", "Me Tester", now.Add(-time.Hour))        // recent, by the user
	for i := 0; i < 7; i++ {                                   // churn above the threshold of 5
		commit("hot.go", "Me Tester", now.Add(-time.Duration(i+2)*24*time.Hour))
	}

	ctx, cancel := context.WithTimeout(context.Background(), 90*time.Second)
	defer cancel()
	cortex, err := BootCortex(ctx, ws, "", nil)
	if err != nil {
		t.Fatalf("BootCortex: %v", err)
	}
	defer requireWorkspaceReleased(t, cortex, ws)

	del, refactor := types.MangleAtom("/delete"), types.MangleAtom("/refactor")
	mutation, current := types.MangleAtom("/mutation"), types.MangleAtom("/current_intent")
	var intents []types.Fact
	for _, file := range []string{"theirs.go", "stale.go", "mine.go"} {
		intents = append(intents, types.Fact{Predicate: "user_intent", Args: []any{current, mutation, del, file, ""}})
	}
	intents = append(intents,
		types.Fact{Predicate: "user_intent", Args: []any{current, mutation, refactor, "hot.go", ""}},
		types.Fact{Predicate: "user_intent", Args: []any{current, mutation, refactor, "mine.go", ""}},
	)
	if err := cortex.Kernel.AssertBatch(intents); err != nil {
		t.Fatalf("assert intents: %v", err)
	}

	users, err := cortex.Kernel.Query("current_user")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(users, "Me Tester") {
		t.Errorf("the world scan must assert the repository author as current_user: %v", users)
	}

	warnings, err := cortex.Kernel.Query("chesterton_fence_warning")
	if err != nil {
		t.Fatal(err)
	}
	if !factWith(warnings, "theirs.go", "recent_change_by_other") {
		t.Errorf("deleting a file another author changed an hour ago must warn: %v", warnings)
	}
	if !factWith(warnings, "hot.go", "high_churn_file") {
		t.Errorf("refactoring a file touched by seven commits must warn: %v", warnings)
	}
	if factWith(warnings, "stale.go", "recent_change_by_other") {
		t.Errorf("another author's change from 30 days ago is not recent: %v", warnings)
	}
	if factWith(warnings, "mine.go", "recent_change_by_other") {
		t.Errorf("the user's own recent change must not warn the user: %v", warnings)
	}
	if factWith(warnings, "mine.go", "high_churn_file") {
		t.Errorf("a single-commit file is not high churn: %v", warnings)
	}
}

func runGit(t *testing.T, dir string, env []string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(), env...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
