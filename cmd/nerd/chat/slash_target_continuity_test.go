package chat

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"codenerd/internal/config"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/session"
)

// "/fix auth.go preserve the exported API" parsed auth.go as the target and
// the rest as the constraint, then handed the whole request to specialist
// resolution and spawned the plain shard with no target field: the executor
// selected experts from an empty target. The parsed target now reaches both.
func TestSlashFix_TheParsedTargetReachesResolutionAndTheExecutor(t *testing.T) {
	m, _ := journeyModel(t, config.WorkModeYolo)
	if err := os.WriteFile(filepath.Join(m.workspace, "auth.go"), []byte("package auth\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.shardMgr = coreshards.NewShardManager()
	exec := &requestRecordingExecutor{}
	m.taskExecutor = exec

	parts := []string{"/fix", "auth.go", "preserve", "the", "exported", "API"}
	_, cmd := m.handleCmdFix(strings.Join(parts, " "), parts)
	if cmd == nil {
		t.Fatal("the handler returned no command")
	}
	// The spawn is the batch's second command (the first is the spinner).
	batch, ok := cmd().(tea.BatchMsg)
	if !ok || len(batch) < 2 {
		t.Fatalf("handler command = %T, want a batch with the spawn", cmd())
	}
	batch[len(batch)-1]()

	exec.mu.Lock()
	requests := append([]session.TaskRequest(nil), exec.requests...)
	exec.mu.Unlock()
	if len(requests) == 0 {
		t.Fatal("no task was spawned")
	}
	req := requests[len(requests)-1]
	if req.Target != "auth.go" || !strings.Contains(req.Constraint, "preserve the exported API") || !strings.Contains(req.Request, "auth.go preserve") {
		t.Fatalf("spawned request = %+v; want target auth.go, the requirement as constraint, the request as typed", req)
	}
}
