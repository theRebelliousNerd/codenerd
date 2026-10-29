package system

import (
	"fmt"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/types"
)

func TestSessionPlanner_AdoptsEveryDecomposedItem(t *testing.T) {
	s := NewSessionPlannerShard()
	var b strings.Builder
	b.WriteByte('[')
	const n = 60
	for i := 0; i < n; i++ {
		if i > 0 {
			b.WriteByte(',')
		}
		fmt.Fprintf(&b, `{"description":"task %d","priority":%d}`, i, i+1)
	}
	b.WriteByte(']')

	items := s.parseAgendaItems(b.String())
	if len(items) != n {
		t.Fatalf("parsed %d items, want %d", len(items), n)
	}
	s.adoptAgenda(items)

	s.mu.RLock()
	defer s.mu.RUnlock()
	if len(s.agenda) != n {
		t.Fatalf("agenda len = %d, want %d", len(s.agenda), n)
	}
}

func TestSessionPlanner_CheckpointsWhenATaskCompletes(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	s := NewSessionPlannerShard()
	s.Kernel = kernel
	s.adoptAgenda([]AgendaItem{{ID: "task-1", Description: "one", Status: "pending"}})

	if err := kernel.Assert(types.Fact{Predicate: "task_completed", Args: []any{"task-1"}}); err != nil {
		t.Fatalf("assert task_completed: %v", err)
	}
	s.updateAgendaFromKernel()

	s.mu.RLock()
	status := s.agenda[0].Status
	checkpoints := len(s.checkpoints)
	s.mu.RUnlock()
	if status != "completed" {
		t.Fatalf("status = %q, want completed", status)
	}
	if checkpoints != 1 {
		t.Fatalf("checkpoints = %d, want 1 after the task completed", checkpoints)
	}

	s.updateAgendaFromKernel()
	s.mu.RLock()
	checkpoints = len(s.checkpoints)
	s.mu.RUnlock()
	if checkpoints != 1 {
		t.Fatalf("checkpoints = %d after a second sync, want 1", checkpoints)
	}
}
