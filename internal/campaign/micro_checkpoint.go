package campaign

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"codenerd/internal/logging"
)

// findWorkspaceFileByBase walks workspace for a file with the given basename
// (skips .nerd, node_modules, target). Used when planner paths don't match layout.
func findWorkspaceFileByBase(workspace, base string) string {
	if workspace == "" || base == "" || base == "." || base == string(filepath.Separator) {
		return ""
	}
	var found string
	_ = filepath.WalkDir(workspace, func(path string, d os.DirEntry, err error) error {
		if err != nil || found != "" {
			return nil
		}
		if d.IsDir() {
			name := d.Name()
			if name == ".nerd" || name == "node_modules" || name == "target" || name == ".git" || name == "dist" {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.EqualFold(d.Name(), base) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	return found
}

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
	for _, w := range o.attemptWrites(task) {
		writeSet = append(writeSet, w.Path)
	}

	// File existence sanity for create/modify tasks (fail fast before expensive checks).
	// Planner paths are often wrong (e.g. cmd/server/main.go when code is backend/main.go).
	// Accept any write_set path that exists OR a same-basename file under the workspace
	// so checkpoints do not hard-fail layout mismatches after successful nearby writes.
	for _, p := range writeSet {
		info, err := os.Stat(p)
		if err == nil {
			if info.IsDir() {
				continue
			}
			continue
		}
		if alt := findWorkspaceFileByBase(o.workspace, filepath.Base(p)); alt != "" {
			logging.Get(logging.CategoryCampaign).Warn(
				"micro-checkpoint: planned path %s missing; found alternate %s", p, alt,
			)
			continue
		}
		// Soft-skip missing planned paths when ANY other write_set entry exists
		// or any alternate was already accepted — planners invent extra files
		// (server.py + main.py). Fail only if zero planned paths resolved.
		logging.Get(logging.CategoryCampaign).Warn(
			"micro-checkpoint: skipping missing planned path %s", p,
		)
		continue
	}
	// If write set was non-empty but nothing existed, fail.
	anyExists := false
	for _, p := range writeSet {
		if _, err := os.Stat(p); err == nil {
			anyExists = true
			break
		}
		if findWorkspaceFileByBase(o.workspace, filepath.Base(p)) != "" {
			anyExists = true
			break
		}
	}
	if !anyExists {
		return fmt.Errorf("micro-checkpoint: none of planned write_set paths exist: %v", writeSet)
	}
	return nil
}
