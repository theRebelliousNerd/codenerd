package campaign

import (
	"context"
	"fmt"
	"os"
	"path"
	"slices"
	"strings"

	"codenerd/internal/logging"
)

// The safe point: the one place a recurse run changes what it is running on.
// It is reached between visits, after the previous attempt was judged and
// committed or reverted and before the next one starts, so the tree is clean
// and nothing is in flight. At it the run
//
//  1. absorbs the base branch (this file), then
//  2. moves onto its own rebuilt binary when its commits changed what the
//     binary is built from (recurse_self.go).
//
// The loop never does either mid-attempt: an attempt's measurements, its
// in-flight record and its revert all assume the tree does not move under it.

// recurseBaseState is a run's base: where it came from, and the base commit
// whose merge last conflicted (a conflicted base is not retried until the
// base moves again).
type recurseBaseState struct {
	name, source string
	conflictTip  string
}

func (r *recurseRun) info(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logging.Get(logging.CategoryCampaign).Info("%s", msg)
	r.logf("recurse: INFO %s", msg)
}

func (r *recurseRun) warn(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	logging.Get(logging.CategoryCampaign).Warn("%s", msg)
	r.logf("recurse: WARN %s", msg)
}

// recordedBase is the base the ledger last named: a resumed run keeps the base
// it started with unless the flag says otherwise.
func recordedBase(history []recurseRecord) string {
	for i := len(history) - 1; i >= 0; i-- {
		if history[i].Step == stepBase {
			return history[i].Base
		}
	}
	return ""
}

// startBase fixes the run's base, journals it, and says where it came from. A
// mission is bounded and its evidence receipts are bound to one tree snapshot,
// so a mission run does not absorb: the tree it measured is the tree it keeps.
func (r *recurseRun) startBase(ctx context.Context, history []recurseRecord, startBranch string) error {
	if r.cfg.Mission != nil {
		r.info("base: not absorbed in a mission run (its evidence is bound to the tree it measured)")
		return nil
	}
	name, source, err := r.git.resolveBase(ctx, r.branch, r.cfg.BaseBranch, recordedBase(history), startBranch)
	if err != nil {
		return err
	}
	if name == "" {
		r.warn("base: no base branch found (none recorded, no start branch, no remote default, no main); the base is not absorbed. Name one with --base-branch")
		return nil
	}
	tip, err := r.git.commitOf(ctx, name)
	if err != nil {
		return err
	}
	r.base = &recurseBaseState{name: name, source: source}
	if err := r.journal.append(recurseRecord{Step: stepBase, Base: name, Commit: tip, Detail: "from " + source}); err != nil {
		return err
	}
	r.info("base: %s at %.9s (from %s); new base commits are merged into %s between attempts", name, tip, source, r.branch)
	return nil
}

// absorbBase merges the base's new commits, when it has any. It reports
// whether the tree changed. A conflict is settled for the base package by
// package (recurseGit.yieldToBase) and the merge completes. One that cannot be
// settled is aborted, the tree left exactly as it was, and recorded; the run
// goes on with the base it has and tries again once the base moves. A base
// that cannot be read is a warning, not the end of the run.
func (r *recurseRun) absorbBase(ctx context.Context, pass int) (bool, error) {
	if r.base == nil {
		return false, nil
	}
	tip, behind, err := r.git.behindBase(ctx, r.base.name)
	if err != nil {
		if ctx.Err() != nil {
			return false, ctx.Err()
		}
		r.warn("base %s could not be read; running on the base already absorbed: %v", r.base.name, err)
		return false, nil
	}
	if behind == 0 || tip == r.base.conflictTip {
		return false, nil
	}
	res, err := r.git.absorb(ctx, r.base.name, tip, behind)
	if err != nil {
		return false, err
	}
	switch res.Outcome {
	case absorbSkipped:
		r.warn("base %s is %d commit(s) ahead but %s; trying again at the next boundary", r.base.name, behind, res.Reason)
		return false, nil
	case absorbConflict:
		r.base.conflictTip = tip
		detail := fmt.Sprintf("merging %s at %.9s (%d commit(s)) failed and was aborted, the tree is unchanged", r.base.name, tip, behind)
		if len(res.Conflicts) > 0 {
			detail += "; conflicting files: " + strings.Join(res.Conflicts, ", ")
		} else {
			detail += ": " + oneLine(res.Reason)
		}
		r.warn("%s; running on the old base, retrying when %s moves", detail, r.base.name)
		return false, r.journal.append(recurseRecord{Step: stepAbsorbConflict, Pass: pass, Base: r.base.name, Commit: tip, Detail: detail})
	}
	return true, r.absorbed(ctx, pass, res)
}

// absorbed is the run after a completed merge: what the tree was derived from
// is stale, and the nodes whose files the merge changed are changed nodes.
func (r *recurseRun) absorbed(ctx context.Context, pass int, res absorbResult) error {
	r.invalidateTree()
	touched := r.touchedNodes(ctx, res.Changed)
	r.cycle++
	detail := fmt.Sprintf("merged %d commit(s) of %s into %s (%.9s -> %.9s), %d file(s) changed", res.Commits, r.base.name, r.branch, res.Before, res.After, len(res.Changed))
	if len(res.Yielded) > 0 {
		detail += fmt.Sprintf("; the merge conflicted in %s, so the base's version of %s was kept and the run's own changes there were replaced",
			strings.Join(res.Conflicts, ", "), strings.Join(res.Yielded, ", "))
	}
	// The ledger first: it is what resume replays.
	rec := recurseRecord{Step: stepAbsorb, Pass: pass, Cycle: r.cycle, Base: r.base.name, Commit: res.BaseTip, Nodes: touched, Detail: detail}
	if err := r.journal.append(rec); err != nil {
		return err
	}
	for _, id := range touched {
		if err := r.policy.nodeKept(id, r.cycle); err != nil {
			return err
		}
	}
	r.noteTreeChange(res.Changed)
	r.info("absorbed base: %s; derived state (graph, gate measurements, baselines) is re-measured, stalls lifted on %d node(s)", detail, len(touched))
	return nil
}

// invalidateTree drops what was derived from the tree as it stood: the gate
// measurements, the error-free baseline and the finding groups cut from it.
// The sweep graph and the gate set are derived again by the pass
// (deriveSweep); the next cycles measure the new tree.
func (r *recurseRun) invalidateTree() {
	r.state = map[string]gateRun{}
	_ = os.Remove(workspaceStatePath(r.root))
	r.gateBaseline = nil
	r.efGroups, r.efOpen = nil, nil
	r.efDirty = true
	r.cyclesSinceFull = 0
	r.notedUnattributed = nil
}

// touchedNodes names the sweep nodes whose files changed. Their "failed
// twice" marks are lifted by recording each as a kept change
// (recurse_node_kept), the same fact a kept commit asserts: the finding may be
// fixable on the new code. A change to go.mod or go.sum touches every node.
func (r *recurseRun) touchedNodes(ctx context.Context, changed []string) []string {
	nodes, err := deriveRecurseDAG(ctx, r.root)
	if err != nil {
		r.warn("the sweep graph could not be derived after the merge (%v); stalls are lifted on the graph the run had", err)
		nodes = r.allNodes
	}
	set := map[string]bool{}
	for _, p := range changed {
		if base := path.Base(p); base == "go.mod" || base == "go.sum" {
			for _, n := range nodes {
				set[n.ID] = true
			}
			continue
		}
		if id := attributeNodeInRoot(r.root, p, nodes); id != RecurseUnattributedNodeID {
			set[id] = true
		}
	}
	return slices.Sorted(func(yield func(string) bool) {
		for id := range set {
			if !yield(id) {
				return
			}
		}
	})
}

// safePoint is called between visits. It reports whether the tree changed
// under the run, which makes the pass derive its graph and gates again.
func (r *recurseRun) safePoint(ctx context.Context, pass int) (bool, error) {
	fresh, err := r.absorbBase(ctx, pass)
	if err != nil {
		return false, err
	}
	return fresh, r.selfUpdate(ctx, pass)
}
