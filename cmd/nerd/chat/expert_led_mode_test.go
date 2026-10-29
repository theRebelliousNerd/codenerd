package chat

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/session"
	"codenerd/internal/types"
)

// requestRecordingExecutor keeps every task request chat delegation sends.
type requestRecordingExecutor struct {
	mu       sync.Mutex
	requests []session.TaskRequest
}

func (e *requestRecordingExecutor) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	return e.ExecuteWithContext(ctx, req, nil, types.PriorityNormal)
}

func (e *requestRecordingExecutor) ExecuteWithContext(_ context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.requests = append(e.requests, req)
	return "done", nil
}

func (e *requestRecordingExecutor) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", fmt.Errorf("unexpected async execution")
}

func (e *requestRecordingExecutor) GetResult(string) (string, bool, error) {
	return "", false, fmt.Errorf("unexpected result lookup")
}

func (e *requestRecordingExecutor) WaitForResult(context.Context, string) (string, error) {
	return "", fmt.Errorf("unexpected wait")
}

// A task the kernel routes to an executor shard agent runs under its own verb
// with its target -- the coder's persona, write tools and gates -- and the
// executor's expert_spawn consults the agent to lead. It is never spawned as
// the agent's own verb, whose /general catalog cannot write: before this,
// chat handed a /fix on a Bubble Tea file to "bubbleteaexpert" and told it to
// implement the solution with read-only tools.
func TestExpertLedModeRunsTheTaskUnderItsVerb(t *testing.T) {
	m := newRoundtripModel(t)
	assertExpertRoster(t, m)
	ws := t.TempDir()
	file := filepath.Join(ws, "cmd", "nerd", "chat", "view.go")
	if err := os.MkdirAll(filepath.Dir(file), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(file, []byte("package chat\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	m.workspace = ws
	exec := &requestRecordingExecutor{}
	m.taskExecutor = exec

	msg := m.spawnShardWithSpecialists("/fix", "coder", "fix the view's resize handling", "cmd/nerd/chat/view.go")()

	exec.mu.Lock()
	requests := append([]session.TaskRequest(nil), exec.requests...)
	exec.mu.Unlock()
	if len(requests) == 0 {
		t.Fatalf("no task was spawned; message %+v", msg)
	}
	ran := false
	for _, req := range requests {
		if strings.EqualFold(strings.TrimPrefix(req.IntentVerb, "/"), "bubbleteaexpert") {
			t.Errorf("the task was spawned as the agent's own read-only verb: %+v", req)
		}
		if req.IntentVerb == "/fix" && req.Target == "cmd/nerd/chat/view.go" {
			ran = true
		}
	}
	if !ran {
		t.Errorf("the task was not run under /fix with its target; requests %+v", requests)
	}
	if out, ok := msg.(assistantMsg); !ok || !strings.Contains(out.Surface, "bubbleteaexpert") {
		t.Errorf("the surface does not name the lead: %+v", msg)
	}
}
