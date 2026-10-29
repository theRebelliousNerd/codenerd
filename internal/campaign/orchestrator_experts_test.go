package campaign

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"codenerd/internal/core"
	"codenerd/internal/observation"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// requestLog is a task executor that records every request whole -- verb,
// target and input -- and answers a consultation with advice and a task turn
// by writing the task's file.
type requestLog struct {
	mu        sync.Mutex
	requests  []session.TaskRequest
	workspace string
	advice    string
}

func (r *requestLog) ExecuteObserved(_ context.Context, req session.TaskRequest) (observation.Return, error) {
	r.mu.Lock()
	r.requests = append(r.requests, req)
	r.mu.Unlock()
	if strings.HasPrefix(req.IntentVerb, "/consult/") {
		return observation.Return{Output: r.advice, Outcome: "/done"}, nil
	}
	full := filepath.Join(r.workspace, filepath.FromSlash(req.Target))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		return observation.Return{}, err
	}
	if err := os.WriteFile(full, []byte("written\n"), 0o644); err != nil {
		return observation.Return{}, err
	}
	return observation.Return{Output: "Wrote the file.", Outcome: "/done",
		Writes: []observation.FileWrite{{Path: full, Before: absent, After: known("written\n")}}}, nil
}

func (r *requestLog) ExecuteObservedWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (observation.Return, error) {
	return r.ExecuteObserved(ctx, req)
}

func (r *requestLog) Execute(ctx context.Context, req session.TaskRequest) (string, error) {
	ret, err := r.ExecuteObserved(ctx, req)
	return ret.Output, err
}

func (r *requestLog) ExecuteWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (string, error) {
	return r.Execute(ctx, req)
}

func (r *requestLog) ExecuteAsync(context.Context, session.TaskRequest) (string, error) {
	return "", nil
}

func (r *requestLog) GetResult(string) (string, bool, error) { return "", false, nil }

func (r *requestLog) WaitForResult(context.Context, string) (string, error) { return "", nil }

func (r *requestLog) all() []session.TaskRequest {
	r.mu.Lock()
	defer r.mu.Unlock()
	return append([]session.TaskRequest(nil), r.requests...)
}

// runExpertTask runs one /file_create task on target through the scheduler,
// on the shipped corpus with a two-agent roster: a Mangle expert (a language
// domain, which advises) and a Bubble Tea expert (an import domain, which
// policy/shards.mg classifies as an executor).
func runExpertTask(t *testing.T, target string, world []types.Fact) []session.TaskRequest {
	t.Helper()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("the shipped corpus must load: %v", err)
	}
	roster := []types.Fact{
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/mangleexpert")}},
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/bubbleteaexpert")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/mangleexpert"), types.MangleAtom("/mangle"), types.MangleAtom("/registry")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/bubbleteaexpert"), types.MangleAtom("/bubbletea"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_exact", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		// An advisor-classified agent (policy/shards.mg) with import evidence
		// advises: a language-only match lends knowledge and spawns nothing.
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/testarchitect")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/testarchitect"), types.MangleAtom("/testify"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_exact", Args: []any{"github.com/stretchr/testify", types.MangleAtom("/testify")}},
	}
	if err := kernel.AssertBatch(append(roster, world...)); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
	workspace := t.TempDir()
	log := &requestLog{workspace: workspace, advice: "Keep every negation bound; declare each new predicate."}
	c := &Campaign{
		ID: "/campaign_experts", Type: CampaignTypeFeature, Title: "experts", Goal: "route to the expert",
		Status: StatusActive, CreatedAt: time.Now().UTC(), TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_experts", CampaignID: "/campaign_experts", Name: "build", Status: PhaseInProgress,
			Category: "/implementation", EstimatedComplexity: "/low",
			Objectives: []PhaseObjective{{Type: ObjectiveCreate, Description: "the file", VerificationMethod: VerifyNone}},
			Tasks: []Task{{
				ID: "/task_expert", PhaseID: "/phase_experts", Description: "create " + target,
				Status: TaskPending, Type: TaskTypeFileCreate, Priority: PriorityNormal,
				Artifacts: []TaskArtifact{{Type: "/source_file", Path: target}},
			}},
		}},
	}
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace: workspace, Kernel: kernel, LLMClient: &MockLLMClient{}, PlanningLLMClient: &MockLLMClient{}, TaskExecutor: log,
		Executor: tactile.NewDirectExecutor(), VirtualStore: &core.VirtualStore{},
		Campaign: testCampaignConfig(fastRetries(1)),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	t.Cleanup(func() { _ = orch.Close() })
	orch.replanner = nil
	if err := orch.SetCampaign(c); err != nil {
		t.Fatalf("SetCampaign: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := orch.runPhase(ctx, &orch.campaign.Phases[0]); err != nil && ctx.Err() == nil {
		t.Logf("runPhase returned: %v", err)
	}
	if got := orch.campaign.Phases[0].Tasks[0].Status; got != TaskCompleted {
		t.Fatalf("task status = %s, want completed", got)
	}
	return log.all()
}

// A campaign task on a file importing testify needs the test architect, an
// advisor by classification, at import strength: specialist_should_advise. The
// agent is consulted before the turn, on the campaign's own task executor, and
// the turn carries its advice and aims at the file, so its compile carries the
// expert's knowledge.
func TestCampaignTask_AnAdvisingShardAgentIsConsultedFirst(t *testing.T) {
	t.Parallel()
	const target = "internal/policy/rules_test.go"
	reqs := runExpertTask(t, target, []types.Fact{
		{Predicate: "dependency_link", Args: []any{target, "pkg:github.com/stretchr/testify", "github.com/stretchr/testify"}},
	})
	if len(reqs) != 2 {
		t.Fatalf("requests = %+v, want a consultation then the task's turn", reqs)
	}
	consult, turn := reqs[0], reqs[1]
	if consult.IntentVerb != "/consult/testarchitect" || !strings.Contains(consult.Task, target) {
		t.Fatalf("first request = %+v, want the test architect consulted on %s", consult, target)
	}
	if turn.Target != target {
		t.Fatalf("the task's turn aims at %q, want %s", turn.Target, target)
	}
	if !strings.Contains(turn.Task, "ADVICE FROM SHARD AGENT testarchitect") || !strings.Contains(turn.Task, "Keep every negation bound") {
		t.Fatalf("the task's turn does not carry the agent's advice:\n%s", turn.Task)
	}
}

// A task on a file that imports Bubble Tea needs the Bubble Tea expert at
// import strength, and it is an executor: specialist_should_execute. The agent
// owns the task through the turn's compile (the turn aims at the file); it is
// not also consulted.
func TestCampaignTask_AnExecutingShardAgentOwnsTheTurn(t *testing.T) {
	t.Parallel()
	const target = "cmd/app/view.go"
	reqs := runExpertTask(t, target, []types.Fact{
		{Predicate: "dependency_link", Args: []any{target, "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
	})
	if len(reqs) != 1 {
		t.Fatalf("requests = %+v, want only the task's turn", reqs)
	}
	if reqs[0].Target != target || strings.HasPrefix(reqs[0].IntentVerb, "/consult/") {
		t.Fatalf("the task's turn = %+v, want it aimed at %s and not a consultation", reqs[0], target)
	}
}

// The campaign asks the same decision the turn's compile does: the selection
// for a task's files names the executing agent.
func TestCampaignTask_SelectsFromTheKernelDecision(t *testing.T) {
	t.Parallel()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatal(err)
	}
	if err := kernel.AssertBatch([]types.Fact{
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/bubbleteaexpert")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/bubbleteaexpert"), types.MangleAtom("/bubbletea"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_exact", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		{Predicate: "dependency_link", Args: []any{"cmd/app/view.go", "pkg:bubbletea", "github.com/charmbracelet/bubbletea"}},
	}); err != nil {
		t.Fatal(err)
	}
	o := &Orchestrator{kernel: kernel, workspace: t.TempDir()}
	task := &Task{ID: "/t", WriteSet: []string{"cmd/app/view.go", "internal/**/*.go"}}
	targets := o.taskTargets(task)
	if len(targets) != 1 || targets[0] != "cmd/app/view.go" {
		t.Fatalf("taskTargets = %v, want the one file (the glob names none)", targets)
	}
	sel := o.selectTaskExperts(task, "/fix", targets)
	if len(sel.Needs) != 1 || sel.Needs[0].Agent != "bubbleteaexpert" || !sel.Needs[0].Execute {
		t.Fatalf("selection = %+v, want bubbleteaexpert to execute", sel)
	}
}
