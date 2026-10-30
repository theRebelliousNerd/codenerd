package world

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	"codenerd/internal/core"
	"codenerd/internal/logging"
	"codenerd/internal/types"
)

// RepoHistoryDepth is how many commits the world scan reads. churn_rate counts
// the commits inside this window that touched a file, so the depth is part of
// what the churn rules (policy/git_safety.mg, reviewer.mg) mean.
const RepoHistoryDepth = 100

// attachRepoFacts measures the repository and attaches its facts to a scan
// result when they changed since this scanner last measured them.
//
// The measurement key is HEAD plus the author identity: history only moves when
// HEAD does, and current_user only when the git config does. Equal key, no
// facts, and ApplyIncrementalResult leaves the kernel's rows alone. A workspace
// that is not a repository, or has no commit yet, measures nothing.
func (s *Scanner) attachRepoFacts(ctx context.Context, root string, res *IncrementalResult) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	head, ok := gitOutput(ctx, root, "rev-parse", "HEAD")
	if !ok {
		return ctx.Err()
	}
	user, haveUser := gitAuthorName(ctx, root)
	key := head + "\x00" + user

	s.repoMu.Lock()
	same := s.repoMemo == key
	s.repoMu.Unlock()
	if same {
		return nil
	}

	facts, err := ScanGitHistory(ctx, root, RepoHistoryDepth)
	if err != nil {
		// A failed history read is not a failed world scan: the world facts are
		// sound and the next scan measures again (the memo is not advanced).
		logging.WorldWarn("repository facts not refreshed: %v", err)
		return ctx.Err()
	}
	if haveUser {
		facts = append(facts, core.Fact{Predicate: "current_user", Args: []any{user}})
	}
	res.RepoMeasured = true
	res.RepoFacts = facts

	s.repoMu.Lock()
	s.repoMemo = key
	s.repoMu.Unlock()
	return nil
}

// gitAuthorName is the name git would record as author of a commit made now,
// from `git var GIT_AUTHOR_IDENT` (environment, then repository and user
// config). It is the spelling `git log --pretty=%an` reports, which is what
// git_history carries as Author. No resolvable identity is no fact: a rule must
// not guess who the user is.
func gitAuthorName(ctx context.Context, root string) (string, bool) {
	ident, ok := gitOutput(ctx, root, "var", "GIT_AUTHOR_IDENT")
	if !ok {
		return "", false
	}
	name, _, found := strings.Cut(ident, " <")
	name = strings.TrimSpace(name)
	return name, found && name != ""
}

func gitOutput(ctx context.Context, root string, args ...string) (string, bool) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	return strings.TrimSpace(string(out)), true
}

// applyRepoFacts replaces the kernel's git rows with the scan's measurement.
func applyRepoFacts(kernel types.Kernel, res *IncrementalResult) error {
	if !res.RepoMeasured {
		return nil
	}
	for _, pred := range GitPredicates {
		if err := kernel.Retract(pred); err != nil {
			return fmt.Errorf("retract %s: %w", pred, err)
		}
	}
	if len(res.RepoFacts) == 0 {
		return nil
	}
	return kernel.LoadFacts(toTypesFacts(res.RepoFacts))
}
