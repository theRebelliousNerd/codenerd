package campaign

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"codenerd/internal/observation"
)

// campaign-deletion-microcheckpoint #1: a mutating task whose change is the
// removal of its declared file leaves nothing on disk to stat. The attempt's
// recorded removal is the evidence; an attempt that removed nothing still fails.
func TestRunTaskMicroCheckpoint_AcceptsTheAttemptsOwnRemoval(t *testing.T) {
	dir := t.TempDir()
	gone := filepath.Join(dir, "obsolete.go")
	orch := newSnapshotTestOrchestrator()
	orch.workspace = dir
	task := &orch.campaign.Phases[0].Tasks[0]
	task.Type = TaskTypeRefactor
	task.WriteSet = []string{gone}

	orch.beginAttempt(task)
	if err := orch.runTaskMicroCheckpoint(context.Background(), task); err == nil {
		t.Fatal("an attempt that neither left nor removed its file must fail the gate")
	}
	orch.recordAttemptWrites(task, []observation.FileWrite{{
		Path:   gone,
		Before: observation.FileState{Known: true, Exists: true, Content: "package x\n"},
		After:  observation.FileState{Known: true},
	}})
	if _, err := os.Stat(gone); !os.IsNotExist(err) {
		t.Fatalf("precondition: %s must be absent", gone)
	}
	if err := orch.runTaskMicroCheckpoint(context.Background(), task); err != nil {
		t.Fatalf("the attempt's own removal must satisfy the gate: %v", err)
	}
}
