package campaign

import (
	"context"
	"fmt"
	"os"

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
	for _, w := range o.attemptWrites(task) {
		writeSet = append(writeSet, w.Path)
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
		logging.Get(logging.CategoryCampaign).Warn("micro-checkpoint: planned path %s is not on disk", p)
	}
	return fmt.Errorf("micro-checkpoint: none of the planned write_set paths exist and the attempt wrote none of its own: %v", writeSet)
}
