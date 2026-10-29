package core

import "testing"

// The rows a caller records about a delegated task's dispatches, and the
// verdict derived from them (policy/task_verdict.mg), are the host's. A model
// that could write them could mark its own dispatch done or keep itself being
// sent back.
func TestHostWitness_TheTaskVerdictRowsAreNotTheModels(t *testing.T) {
	for _, pred := range []string{
		"task_dispatch", "task_dispatch_gap", "task_dispatch_blocked",
		"task_dispatch_verified", "task_dispatch_blocker", "task_dispatch_moved", "task_loop_verdict",
	} {
		if predicateAllowed(pred, MangleUpdatePolicy{}) {
			t.Errorf("a model may write %s", pred)
		}
	}
}
