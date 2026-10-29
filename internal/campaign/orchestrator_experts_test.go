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
func runExpertTask(t *testing.T, target string, world []types.Fact) ([]session.TaskRequest, []OrchestratorEvent) {
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
		{Predicate: "technology_import_prefix", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
		// An agent policy/shards.mg does not classify as an executor: an
		// import match makes it an advisor (specialist_should_advise).
		{Predicate: "shard_agent", Args: []any{types.MangleAtom("/yamlexpert")}},
		{Predicate: "shard_agent_domain", Args: []any{types.MangleAtom("/yamlexpert"), types.MangleAtom("/yaml"), types.MangleAtom("/registry")}},
		{Predicate: "technology_import_prefix", Args: []any{"gopkg.in/yaml.v3", types.MangleAtom("/yaml")}},
	}
	if err := kernel.AssertBatch(append(roster, world...)); err != nil {
		t.Fatalf("assert roster: %v", err)
	}
	workspace := t.TempDir()
	events := make(chan OrchestratorEvent, 256)
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
		Campaign: testCampaignConfig(fastRetries(1)), EventChan: events,
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
	var got []OrchestratorEvent
	for {
		select {
		case e := <-events:
			got = append(got, e)
		default:
			return log.all(), got
		}
	}
}

func eventsOf(events []OrchestratorEvent, kind OrchestratorEventType) []OrchestratorEvent {
	var out []OrchestratorEvent
	for _, e := range events {
		if e.Type == kind {
			out = append(out, e)
		}
	}
	return out
}

// A campaign task on a file importing gopkg.in/yaml.v3 needs the YAML expert,
// an advisor (an import match on an agent that is not an executor). There is
// one consultation path: the session
// executor's, which runs the consultations the kernel spawns for the turn's
// Target (internal/session/expert_spawn.go; counted once per task by
// TestCampaignShapedTaskConsultsItsExpertOncePerTask). The campaign sends the
// task's turn, aimed at the file, and no consultation of its own -- it used to
// send one too, so the advisor was asked twice per task -- and records the
// selection as an event.
func TestCampaignTask_AnAdvisingShardAgentIsConsultedOnlyByTheTurn(t *testing.T) {
	t.Parallel()
	const target = "internal/config/load.go"
	reqs, events := runExpertTask(t, target, []types.Fact{
		{Predicate: "dependency_link", Args: []any{target, "pkg:gopkg.in/yaml.v3", "gopkg.in/yaml.v3"}},
	})
	consultations := 0
	for _, r := range reqs {
		if strings.HasPrefix(r.IntentVerb, "/consult/") {
			consultations++
		}
	}
	if consultations != 0 || len(reqs) != 1 {
		t.Fatalf("requests = %+v: the campaign sent %d consultation(s), want only the task's turn", reqs, consultations)
	}
	if reqs[0].Target != target {
		t.Fatalf("the task's turn aims at %q, want %s", reqs[0].Target, target)
	}
	if got := eventsOf(events, EventSpecialistConsulted); len(got) != 1 || got[0].Message != "yamlexpert" {
		t.Fatalf("specialist_consulted events = %+v, want one for yamlexpert", got)
	}
}

// A task on a file that imports Bubble Tea needs the Bubble Tea expert at
// import strength, and it is an executor: specialist_should_execute. The agent
// owns the task through the turn's compile (the turn aims at the file); it is
// not also consulted.
func TestCampaignTask_AnExecutingShardAgentOwnsTheTurn(t *testing.T) {
	t.Parallel()
	const target = "cmd/app/view.go"
	reqs, events := runExpertTask(t, target, []types.Fact{
		{Predicate: "dependency_link", Args: []any{target, "pkg:github.com/charmbracelet/bubbletea", "github.com/charmbracelet/bubbletea"}},
	})
	if len(reqs) != 1 {
		t.Fatalf("requests = %+v, want only the task's turn", reqs)
	}
	if reqs[0].Target != target || strings.HasPrefix(reqs[0].IntentVerb, "/consult/") {
		t.Fatalf("the task's turn = %+v, want it aimed at %s and not a consultation", reqs[0], target)
	}
	if got := eventsOf(events, EventSpecialistExecutes); len(got) != 1 || got[0].Message != "bubbleteaexpert" {
		t.Fatalf("specialist_executes events = %+v, want one for bubbleteaexpert", got)
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
		{Predicate: "technology_import_prefix", Args: []any{"github.com/charmbracelet/bubbletea", types.MangleAtom("/bubbletea")}},
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
