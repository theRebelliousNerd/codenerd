package campaign

import (
	"context"
	"fmt"
	"maps"
	"path"
	"slices"
	"strconv"
	"strings"
)

// The recurse branch's base. Fixes land on the base branch (normally main)
// from elsewhere while a long run is measuring and editing; a run that never
// takes them in keeps re-failing findings the base already fixed and drifts
// toward conflicts. The loop therefore records its base at start and merges
// it at the safe boundaries between attempts (recurse_absorb.go).
//
// Ruling, merge over rebase: a kept attempt is a commit the ledger names by
// hash and resume finds by its Recurse-Cycle trailer. A rebase rewrites those
// hashes, so the ledger would point at commits that no longer exist, and it
// would need a force-update of a branch the owner may have pushed. A merge
// keeps every kept commit as it is and aborts cleanly on a conflict. The merge
// is always a merge commit (--no-ff), so the branch's first-parent chain holds
// only the loop's own commits and its absorb merges: keptCycles reads that
// chain and never mistakes a cycle trailer arriving from the base for its own.

// Where a run's base branch came from, said in the ledger and the log.
const (
	baseFromFlag   = "--base-branch"
	baseFromLedger = "the ledger"
	baseFromStart  = "the branch the run started on"
	baseFromReflog = "the branch's creation record"
	baseFromRemote = "the remote's default branch"
	baseFromMain   = "the repository's main branch"
)

// Recurse merges carry this trailer naming the base commit they absorbed.
const recurseAbsorbTrailer = "Recurse-Absorb"

// currentBranch is the checked-out branch, "" when HEAD is detached.
func (g recurseGit) currentBranch(ctx context.Context) (string, error) {
	out, err := g.run(ctx, nil, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	name := strings.TrimSpace(out)
	if name == "HEAD" {
		return "", nil
	}
	return name, nil
}

// commitOf resolves rev to a commit hash.
func (g recurseGit) commitOf(ctx context.Context, rev string) (string, error) {
	out, err := g.run(ctx, nil, "rev-parse", "--verify", "--quiet", rev+"^{commit}")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(out), nil
}

// createdFrom reads the branch a branch was created from out of its reflog
// ("branch: Created from main"), the one record git keeps of a worktree's
// start point. "" when the log names HEAD, a commit or anything else that is
// not a local branch now, or has no such entry. Git says whether the name is
// a branch: the string's shape cannot (a branch may be named like a hash).
func (g recurseGit) createdFrom(ctx context.Context, branch string) string {
	out, err := g.run(ctx, nil, "reflog", "show", "--format=%gs", "refs/heads/"+branch)
	if err != nil {
		return ""
	}
	lines := strings.Split(strings.TrimSpace(out), "\n")
	// Newest first: the creation entry is the last.
	from, ok := strings.CutPrefix(strings.TrimSpace(lines[len(lines)-1]), "branch: Created from ")
	if !ok {
		return ""
	}
	from = strings.TrimPrefix(strings.TrimSpace(from), "refs/heads/")
	if from == "" || from == "HEAD" {
		return ""
	}
	if _, err := g.run(ctx, nil, "show-ref", "--verify", "--quiet", "refs/heads/"+from); err != nil {
		return ""
	}
	return from
}

// resolveBase names the branch the run absorbs. An explicit flag wins and
// must resolve to a commit; otherwise the first of these that does: the base
// the ledger recorded, the branch the run started on (the checkout recurse
// was launched from), the branch the recurse branch was created from, the
// remote's default branch, and a local main or master. It returns "" when
// none exists: absorbing is then off, and the caller says so.
func (g recurseGit) resolveBase(ctx context.Context, recurseBranch, flag, recorded, startBranch string) (name, source string, err error) {
	if flag = strings.TrimSpace(flag); flag != "" {
		if flag == recurseBranch {
			return "", "", fmt.Errorf("recurse: the base branch cannot be the recurse branch %s itself", recurseBranch)
		}
		if _, err := g.commitOf(ctx, flag); err != nil {
			return "", "", fmt.Errorf("recurse: --base-branch %q names no commit in this repository", flag)
		}
		return flag, baseFromFlag, nil
	}
	remote := ""
	if out, err := g.run(ctx, nil, "symbolic-ref", "--quiet", "--short", "refs/remotes/origin/HEAD"); err == nil {
		remote = strings.TrimPrefix(strings.TrimSpace(out), "origin/")
	}
	for _, c := range []struct{ name, source string }{
		{recorded, baseFromLedger},
		{startBranch, baseFromStart},
		{g.createdFrom(ctx, recurseBranch), baseFromReflog},
		{remote, baseFromRemote},
		{"main", baseFromMain},
		{"master", baseFromMain},
	} {
		c.name = strings.TrimSpace(c.name)
		if c.name == "" || c.name == recurseBranch {
			continue
		}
		if _, err := g.commitOf(ctx, c.name); err == nil {
			return c.name, c.source, nil
		}
	}
	return "", "", nil
}

// behindBase returns the base's tip and whether it has commits the checked-out
// branch does not.
func (g recurseGit) behindBase(ctx context.Context, base string) (tip string, behind int, err error) {
	if tip, err = g.commitOf(ctx, base); err != nil {
		return "", 0, fmt.Errorf("recurse: base %s: %w", base, err)
	}
	out, err := g.run(ctx, nil, "rev-list", "--count", "HEAD.."+tip)
	if err != nil {
		return tip, 0, err
	}
	behind, err = strconv.Atoi(strings.TrimSpace(out))
	return tip, behind, err
}

// absorbOutcome is what one attempt to take in the base did.
type absorbOutcome int

const (
	absorbMerged absorbOutcome = iota
	// absorbConflict: the merge could not complete and was undone.
	absorbConflict
	// absorbSkipped: the tree was not in a state a merge may start from.
	absorbSkipped
)

// absorbResult describes an absorb for the ledger and the log.
type absorbResult struct {
	Outcome absorbOutcome
	// Before and After are HEAD around the merge; BaseTip the commit merged.
	Before, After, BaseTip string
	// Commits is how many base commits the merge brought in.
	Commits int
	// Changed lists the files the merge changed in the tree (merged only).
	Changed []string
	// Conflicts lists the files that conflicted (conflict only).
	Conflicts []string
	// Reason says why a conflict or a skip happened.
	Reason string
	// Yielded lists the directories (packages) where the merge conflicted and
	// the base's version was kept whole (merged only): the run's own changes
	// there were replaced, and those nodes are stabilized again on the new code.
	Yielded []string
}

// absorb merges the base's commit tip into the checked-out branch. It starts
// only from a tree with no uncommitted tracked change (a merge abort cannot
// restore one) and leaves the tree exactly as it found it unless the merge
// completed: a conflict, or a merge git refuses to start, is aborted and
// verified undone. A tree that cannot be put back is an error, not a result:
// the loop must not run on a half-merged checkout.
func (g recurseGit) absorb(ctx context.Context, base, tip string, commits int) (absorbResult, error) {
	res := absorbResult{BaseTip: tip, Commits: commits}
	before, err := g.commitOf(ctx, "HEAD")
	if err != nil {
		return res, err
	}
	res.Before = before
	dirty, err := g.run(ctx, nil, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return res, err
	}
	if strings.TrimSpace(dirty) != "" {
		res.Outcome, res.Reason = absorbSkipped, "the checkout has uncommitted tracked changes"
		return res, nil
	}
	args := append(g.identityArgs(ctx), "merge", "--no-ff", "--no-edit",
		"-m", fmt.Sprintf("recurse: absorb %s at %.9s", base, tip),
		"-m", fmt.Sprintf("%s: %s", recurseAbsorbTrailer, tip), tip)
	if _, mergeErr := g.run(ctx, nil, args...); mergeErr != nil {
		res.Outcome, res.Reason = absorbConflict, mergeErr.Error()
		if _, err := g.run(ctx, nil, "rev-parse", "-q", "--verify", "MERGE_HEAD"); err == nil {
			if files, err := g.run(ctx, nil, "diff", "--name-only", "--diff-filter=U"); err == nil {
				res.Conflicts = strings.Fields(files)
			}
			if yielded, why := g.yieldToBase(ctx, base, tip, res.Conflicts); len(yielded) > 0 {
				res.Yielded, res.Reason = yielded, ""
				return g.finishMerge(ctx, res)
			} else if why != "" {
				res.Reason = why + "; " + res.Reason
			}
			if _, err := g.run(ctx, nil, "merge", "--abort"); err != nil {
				return res, fmt.Errorf("recurse: the merge of %s conflicted and could not be aborted; the checkout needs the owner: %w", base, err)
			}
		}
		if err := g.requireUnchanged(ctx, before); err != nil {
			return res, err
		}
		return res, nil
	}
	return g.finishMerge(ctx, res)
}

// finishMerge records a completed merge: the new HEAD and the files it changed.
func (g recurseGit) finishMerge(ctx context.Context, res absorbResult) (absorbResult, error) {
	before := res.Before
	after, err := g.commitOf(ctx, "HEAD")
	if err != nil {
		return res, err
	}
	res.After, res.Outcome = after, absorbMerged
	out, err := g.run(ctx, nil, "diff", "--name-only", "--relative", "-z", before, after, "--", ".")
	if err != nil {
		return res, err
	}
	for _, p := range strings.Split(out, "\x00") {
		if p != "" {
			res.Changed = append(res.Changed, p)
		}
	}
	return res, nil
}

// yieldToBase settles a conflicted merge in progress by keeping the base's
// version of every package (directory) that conflicted: each file the run
// changed there since the merge base is put back to the base's tip, and one
// the base does not have is removed, so the run's tests in that package never
// compile against code it no longer has. The run's changes elsewhere stay. It
// commits the merge and returns the packages, or returns none and why, with
// the merge still in progress for the caller to abort. A forever run that only
// aborted would never absorb again once the base held a change to a package
// the run had also changed: the conflicting commit stays in the base's history.
func (g recurseGit) yieldToBase(ctx context.Context, base, tip string, conflicts []string) ([]string, string) {
	if len(conflicts) == 0 {
		return nil, ""
	}
	mergeBase, err := g.run(ctx, nil, "merge-base", "HEAD", tip)
	if err != nil {
		return nil, "no merge base: " + oneLine(err.Error())
	}
	ours, err := g.run(ctx, nil, "diff", "--name-only", "-z", strings.TrimSpace(mergeBase), "HEAD")
	if err != nil {
		return nil, "the run's own changes could not be listed: " + oneLine(err.Error())
	}
	dirs := map[string]bool{}
	for _, c := range conflicts {
		dirs[path.Dir(c)] = true
	}
	paths := map[string]bool{}
	for _, c := range conflicts {
		paths[c] = true
	}
	for _, p := range strings.Split(ours, "\x00") {
		if p != "" && dirs[path.Dir(p)] {
			paths[p] = true
		}
	}
	for _, p := range slices.Sorted(maps.Keys(paths)) {
		spec := ":(top)" + p
		if _, err := g.run(ctx, nil, "cat-file", "-e", tip+":"+p); err == nil {
			if _, err := g.run(ctx, nil, "checkout", tip, "--", spec); err != nil {
				return nil, "the base's " + p + " could not be checked out: " + oneLine(err.Error())
			}
			continue
		}
		if _, err := g.run(ctx, nil, "rm", "-q", "-f", "--ignore-unmatch", "--", spec); err != nil {
			return nil, p + " could not be removed: " + oneLine(err.Error())
		}
	}
	if left, err := g.run(ctx, nil, "diff", "--name-only", "--diff-filter=U"); err != nil || strings.TrimSpace(left) != "" {
		return nil, "files still unmerged: " + oneLine(left)
	}
	yielded := slices.Sorted(maps.Keys(dirs))
	args := append(g.identityArgs(ctx), "commit", "--no-verify",
		"-m", fmt.Sprintf("recurse: absorb %s at %.9s", base, tip),
		"-m", "The base's version was kept in "+strings.Join(yielded, ", ")+" (the merge conflicted there); the run stabilizes those packages again.",
		"-m", fmt.Sprintf("%s: %s", recurseAbsorbTrailer, tip))
	if _, err := g.run(ctx, nil, args...); err != nil {
		return nil, "the merge could not be committed: " + oneLine(err.Error())
	}
	return yielded, ""
}

// requireUnchanged errors unless HEAD is head and no tracked file differs.
func (g recurseGit) requireUnchanged(ctx context.Context, head string) error {
	now, err := g.commitOf(ctx, "HEAD")
	if err != nil {
		return err
	}
	dirty, err := g.run(ctx, nil, "status", "--porcelain", "--untracked-files=no")
	if err != nil {
		return err
	}
	if now != head || strings.TrimSpace(dirty) != "" {
		return fmt.Errorf("recurse: an aborted merge left the checkout changed (HEAD %.9s, was %.9s):\n%s", now, head, dirty)
	}
	return nil
}
