package session

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/types"
)

// deadlineCritic answers the review at once with one finding worth acting on,
// and records the deadline its call carried.
type deadlineCritic struct {
	hangingLLM
	deadline    time.Time
	hadDeadline bool
}

func (d *deadlineCritic) CompleteWithSystem(ctx context.Context, _, _ string) (string, error) {
	d.deadline, d.hadDeadline = ctx.Deadline()
	return "FINDING a.go:1 high: the package has no declarations", nil
}

// deadlineUplift is the uplift round's provider: it answers with no edits and
// records the deadline its call carried.
type deadlineUplift struct {
	deadline    time.Time
	hadDeadline bool
	called      bool
}

func (d *deadlineUplift) CompleteWithToolResults(ctx context.Context, _ string, _ []types.Message, _ []types.ToolDefinition) (*types.LLMToolResponse, error) {
	d.called = true
	d.deadline, d.hadDeadline = ctx.Deadline()
	return &types.LLMToolResponse{Text: "no change"}, nil
}

// H1 (06-unattended-hardening.md): the adversarial review and its uplift round
// carried clocks of their own, 3 and 5 minutes, set when a review hung for
// twenty minutes and the client had no bound; the client has one now (the
// HTTP client each provider is built with). On 2026-09-19 the planner slot
// took 1 min 59 s to 2 min 50 s per review, and R1-8's was cut at 3 minutes --
// on a change with a defect a review could have found. The review and the
// round are bounded as every request is: by the caller's context and the
// client's own bound, not by a constant of their own.
func TestVerifyAndUpliftWithCritic_AddsNoClockOfItsOwn(t *testing.T) {
	ws := t.TempDir()
	if err := os.WriteFile(filepath.Join(ws, "a.go"), []byte("package p\n"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
	cfg := DefaultExecutorConfig()
	cfg.WorkspaceRoot = ws
	critic := &deadlineCritic{}
	e := &Executor{config: cfg, llmClient: critic}
	uplift := &deadlineUplift{}

	e.verifyAndUpliftWithCritic(context.Background(), uplift, "", nil, nil, nil,
		&ExecutionResult{SuccessfulWriteTools: 1, WrittenPaths: []string{"a.go"}})

	if critic.hadDeadline {
		t.Errorf("the review carried a deadline %v from now; a caller with none gave it none", time.Until(critic.deadline).Round(time.Second))
	}
	if !uplift.called {
		t.Fatal("the uplift round did not run for a high finding")
	}
	if uplift.hadDeadline {
		t.Errorf("the uplift round carried a deadline %v from now; a caller with none gave it none", time.Until(uplift.deadline).Round(time.Second))
	}
}
