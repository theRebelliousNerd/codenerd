package campaign

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"codenerd/internal/logging"
)

// runTaskMicroCheckpoint enforces a minimal per-task verification gate: a
// mutating task left at least one of its paths on disk.
//
// It no longer builds the tree (ladder L4). The build it ran -- `go build
// ./...` through the tactile executor, a 20-second limit, the executor's
// environment allowlist -- predates the turn's own gates: the turn a mutating
// task runs has already built ./... under the session's build gate
// (verifyBuild: the project's build environment, its own bound), and a task
// completes only on a /done turn, which owes that gate green. The second
// build added nothing when tasks ran one at a time, failed a good change when
// it took longer than 20 seconds, and with tasks side by side built a tree
// holding a sibling's half-finished edits.
func (o *Orchestrator) runTaskMicroCheckpoint(ctx context.Context, task *Task) error {
	if task == nil || !isMutatingTaskType(task.Type) {
		return nil
	}

	writeSet := o.resolveTaskWriteSet(task)
	if len(writeSet) == 0 {
		// Mutating tasks may omit write_set (see acquireWriteSetLease soft path).
		// Skip micro-checkpoint rather than hard-fail the whole campaign task.
		logging.Get(logging.CategoryCampaign).Warn(
			"micro-checkpoint: task %s has empty write_set; skipping file gate", task.ID,
		)
		return nil
	}
	// Ladder C2: what the attempt wrote is where its change landed -- a
	// modification whose planned target was a guess changes existing code
	// elsewhere -- so those paths are checked with the declared ones.
	var removed []string
	for _, w := range o.attemptWrites(task) {
		writeSet = append(writeSet, w.Path)
		// A removal the attempt made and that still stands is the change
		// landing, not its absence: nothing of a deleted path survives to be
		// statted (campaign-deletion-microcheckpoint #1).
		if w.Before.Known && w.Before.Exists && w.After.Known && !w.After.Exists {
			if _, err := os.Stat(w.Path); os.IsNotExist(err) {
				removed = append(removed, w.Path)
			}
		}
	}

	// The change landed where the task declared it or where the attempt wrote
	// it: a path in either list that is on disk. A file of the same name
	// elsewhere in the workspace is not evidence -- a task whose write set is
	// docs/features/README.md and that wrote nothing used to pass because some
	// other README.md existed (sweep finding F12). What a planner guessed wrong
	// is covered by the attempt's own writes, above.
	for _, p := range writeSet {
		if _, err := os.Stat(p); err == nil {
			return nil
		}
		if strings.ContainsAny(p, "*?[") {
			if matches, err := filepath.Glob(p); err == nil && len(matches) > 0 {
				return nil
			}
		}
		logging.Get(logging.CategoryCampaign).Warn("micro-checkpoint: planned path %s is not on disk", p)
	}
	if task.Type == TaskTypeTestWrite {
		if owned := ownedDirtyTestFiles(o.workspace, o.resolveTaskWriteSet(task), dirtyWorkspaceTestFiles(o.workspace), o.attemptStarted(task)); len(owned) > 0 {
			logging.Campaign("micro-checkpoint: task %s (%s) satisfied by test files changed this attempt beside its write set: %s", task.ID, task.Type, strings.Join(owned, ", "))
			return nil
		}
	}
	if len(removed) > 0 {
		logging.Campaign("micro-checkpoint: task %s (%s) removed %s; the removal is its change", task.ID, task.Type, strings.Join(removed, ", "))
		return nil
	}
	// What was checked is existence, not authorship: none of the declared or
	// attempt-written paths is on disk, and the attempt recorded no removal
	// (campaign-deletion-microcheckpoint #3).
	// The phrase "none of the planned write_set paths exist" is read by
	// resume classification (isNonTerminalResumeAttempt); keep it.
	return fmt.Errorf("micro-checkpoint: none of the planned write_set paths exist (declared or attempt-written) and the attempt recorded no removal: %v", writeSet)
}

// ownedDirtyTestFiles keeps the dirty test files a test_write attempt can be
// credited with: in a directory the task's write set names (a declared
// directory, or the directory of a declared file), and modified since the
// attempt began. Any dirty test file in the workspace used to satisfy the
// gate, including another agent's or a human's work that predated the
// campaign (campaign-testwrite-checkpoint-ownership). With no attempt start
// known nothing is credited.
func ownedDirtyTestFiles(workspace string, writeSet, dirty []string, since time.Time) []string {
	if since.IsZero() || len(writeSet) == 0 {
		return nil
	}
	dirs := map[string]bool{}
	for _, p := range writeSet {
		abs := p
		if !filepath.IsAbs(abs) {
			abs = filepath.Join(workspace, filepath.FromSlash(p))
		}
		abs = filepath.Clean(abs)
		if info, err := os.Stat(abs); err == nil && info.IsDir() {
			dirs[abs] = true
		} else {
			dirs[filepath.Dir(abs)] = true
		}
	}
	var out []string
	for _, rel := range dirty {
		abs := filepath.Join(workspace, filepath.FromSlash(rel))
		if !dirs[filepath.Dir(abs)] {
			continue
		}
		info, err := os.Stat(abs)
		if err != nil || info.ModTime().Before(since) {
			continue
		}
		out = append(out, rel)
	}
	return out
}

func dirtyWorkspaceTestFiles(workspace string) []string {
	ws := strings.TrimSpace(workspace)
	if ws == "" {
		return nil
	}
	cmd := exec.Command("git", "-C", ws, "status", "--porcelain", "--untracked-files=all")
	out, err := cmd.Output()
	if err != nil {
		return nil
	}
	var found []string
	for _, line := range strings.Split(string(out), "\n") {
		if len(line) < 4 {
			continue
		}
		rel := strings.TrimSpace(line[3:])
		if idx := strings.Index(rel, " -> "); idx >= 0 {
			rel = strings.TrimSpace(rel[idx+4:])
		}
		rel = strings.Trim(rel, `"`)
		if !strings.HasSuffix(strings.ToLower(rel), "_test.go") {
			continue
		}
		if _, err := os.Stat(filepath.Join(ws, filepath.FromSlash(rel))); err == nil {
			found = append(found, rel)
		}
	}
	return found
}
