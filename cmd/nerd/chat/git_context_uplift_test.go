package chat

import (
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

// populateGitContext must read the /name attributes the sysfacts writer
// asserts (slash-prefixed) AND tolerate legacy bare-string facts still
// sitting in older stores. Either shape failing silently empties the
// Chesterton's Fence context with no error anywhere.

func TestPopulateGitContext_RoundTrip(t *testing.T) {
	for _, attr := range []string{"/branch", "branch"} {
		t.Run(attr, func(t *testing.T) {
			k, err := core.NewRealKernel()
			if err != nil {
				t.Fatalf("NewRealKernel: %v", err)
			}
			asserts := []core.Fact{
				{Predicate: "git_state", Args: []any{attr, "main"}},
				{Predicate: "git_state", Args: []any{"/modified_files", "a.go\nb.go"}},
				{Predicate: "git_state", Args: []any{"/recent_commits", "fix x"}},
				{Predicate: "git_state", Args: []any{"/unstaged_count", "3"}},
			}
			// Mirror the legacy shape across all four when testing it.
			if attr == "branch" {
				for i := range asserts {
					if s, ok := asserts[i].Args[0].(string); ok && len(s) > 0 && s[0] == '/' {
						asserts[i].Args[0] = s[1:]
					}
				}
			}
			if err := k.AssertBatch(asserts); err != nil {
				t.Fatalf("AssertBatch: %v", err)
			}
			m := NewTestModel()
			m.kernel = k
			ctx := &types.SessionContext{ExtraContext: make(map[string]string)}
			m.populateGitContext(ctx)
			if ctx.GitBranch != "main" {
				t.Errorf("GitBranch = %q, want main", ctx.GitBranch)
			}
			if len(ctx.GitModifiedFiles) != 2 {
				t.Errorf("GitModifiedFiles = %v, want 2 files", ctx.GitModifiedFiles)
			}
			if len(ctx.GitRecentCommits) != 1 {
				t.Errorf("GitRecentCommits = %v, want 1 commit", ctx.GitRecentCommits)
			}
			if ctx.GitUnstagedCount != 3 {
				t.Errorf("GitUnstagedCount = %d, want 3", ctx.GitUnstagedCount)
			}
			if ctx.ExtraContext["git_branch"] != "main" {
				t.Errorf("ExtraContext[git_branch] = %q, want main", ctx.ExtraContext["git_branch"])
			}
		})
	}
}
