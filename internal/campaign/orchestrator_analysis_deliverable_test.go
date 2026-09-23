package campaign

import (
	"context"
	"errors"
	"testing"

	"codenerd/internal/observation"
	"codenerd/internal/session"
)

// Sweep finding F12: an analysis task delivered when an output it holds is on
// disk with content. The answer's shape -- under 40 runes, or opening "I'll"
// or "let me" -- used to decide, and triggered an inline re-spawn with a
// Go-written prompt: a turn that answered nothing was retried and then passed
// with nothing on disk, and a terse real finding was redone and dropped.

// countingExecutor returns ret for every turn and counts the turns.
func countingExecutor(ret observation.Return, spawns *int) *MockTaskExecutor {
	return &MockTaskExecutor{ExecuteObservedFunc: func(context.Context, session.TaskRequest) (observation.Return, error) {
		*spawns++
		return ret, nil
	}}
}

func TestResearchTask_ADoneTurnWithNothingToPersistFails(t *testing.T) {
	o := newArtifactTestOrchestrator(t)
	spawns := 0
	o.taskExecutor = countingExecutor(observation.Return{Output: "  ", Outcome: "/done"}, &spawns)
	task := &Task{ID: "/task_abc_3_1", PhaseID: "/phase_abc_3", Type: TaskTypeResearch, Description: "Audit internal/world"}

	_, err := o.executeResearchTask(context.Background(), task)
	if !errors.Is(err, ErrNoDeliverable) {
		t.Fatalf("executeResearchTask = %v, want ErrNoDeliverable for a done turn with nothing to persist", err)
	}
	if spawns != 1 {
		t.Fatalf("the handler ran %d turns, want 1: the campaign's retry runs the next attempt, carrying why", spawns)
	}
}

func TestResearchTask_ATerseFindingIsDelivered(t *testing.T) {
	o := newArtifactTestOrchestrator(t)
	spawns := 0
	o.taskExecutor = countingExecutor(observation.Return{Output: "No issues found in internal/world.", Outcome: "/done"}, &spawns)
	task := &Task{ID: "/task_abc_3_2", PhaseID: "/phase_abc_3", Type: TaskTypeResearch, Description: "Audit internal/world"}

	if _, err := o.executeResearchTask(context.Background(), task); err != nil {
		t.Fatalf("executeResearchTask = %v, want success for a finding on disk", err)
	}
	if spawns != 1 {
		t.Fatalf("the handler ran %d turns for one finding, want 1", spawns)
	}
	if !o.hasDeliverableOnDisk(task) {
		t.Fatalf("the finding is not on disk: %+v", task.Artifacts)
	}
}

func TestExplicitShardTask_ADoneTurnWithNothingToPersistFails(t *testing.T) {
	o := newArtifactTestOrchestrator(t)
	spawns := 0
	o.taskExecutor = countingExecutor(observation.Return{Output: "", Outcome: "/done"}, &spawns)
	task := &Task{ID: "/task_abc_4_1", PhaseID: "/phase_abc_4", Type: TaskTypeResearch, Shard: "reviewer", Description: "Audit the invariants of internal/world"}

	_, err := o.executeWithExplicitShard(context.Background(), task)
	if !errors.Is(err, ErrNoDeliverable) {
		t.Fatalf("executeWithExplicitShard = %v, want ErrNoDeliverable for a done turn with nothing to persist", err)
	}
	if spawns != 1 {
		t.Fatalf("the handler ran %d turns, want 1", spawns)
	}
}
