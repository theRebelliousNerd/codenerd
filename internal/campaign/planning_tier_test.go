package campaign

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/articulation"
	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/perception"
	"codenerd/internal/session"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// tierLLM is one configured LLM slot in a routing test. It counts every call
// it serves; complete answers Complete and system answers CompleteWithSystem.
type tierLLM struct {
	calls    atomic.Int32
	complete func(prompt string) string
	system   func(systemPrompt, userPrompt string) string
}

func (c *tierLLM) Complete(_ context.Context, prompt string) (string, error) {
	c.calls.Add(1)
	if c.complete == nil {
		return "", nil
	}
	return c.complete(prompt), nil
}

func (c *tierLLM) CompleteWithSystem(_ context.Context, systemPrompt, userPrompt string) (string, error) {
	c.calls.Add(1)
	if c.system == nil {
		return "", nil
	}
	return c.system(systemPrompt, userPrompt), nil
}

func (c *tierLLM) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan types.StreamResult) {
	c.calls.Add(1)
	return types.FailedStream(errors.New("tierLLM does not stream"))
}

func (c *tierLLM) CompleteWithTools(context.Context, string, string, []types.ToolDefinition) (*types.LLMToolResponse, error) {
	c.calls.Add(1)
	return &types.LLMToolResponse{StopReason: "end_turn"}, nil
}

// identifiedLLM is a slot that reports the model it serves, as every provider
// client does (types.ModelIdentifier).
type identifiedLLM struct {
	tierLLM
	provider, model string
}

func (c *identifiedLLM) ModelIdentity() (string, string) { return c.provider, c.model }

// decoratorLLM is a layer that reports nothing of its own and is walked
// through, like the scheduler, tracing and metering layers are.
type decoratorLLM struct {
	tierLLM
	inner perception.LLMClient
}

func (d *decoratorLLM) Unwrap() types.LLMClient { return d.inner }

// newTierOrchestrator builds an orchestrator whose execution-side calls go to
// execution, whose planning goes to planning, and whose tasks run on worker.
func newTierOrchestrator(t *testing.T, kernel core.Kernel, execution, planning perception.LLMClient, worker session.TaskExecutor, edit func(*config.CampaignConfig)) *Orchestrator {
	t.Helper()
	orch, err := NewOrchestrator(OrchestratorConfig{
		Workspace:         t.TempDir(),
		Kernel:            kernel,
		LLMClient:         execution,
		PlanningLLMClient: planning,
		TaskExecutor:      worker,
		Executor:          tactile.NewDirectExecutor(),
		VirtualStore:      &core.VirtualStore{},
		EventChan:         make(chan OrchestratorEvent, 64),
		Campaign:          testCampaignConfig(edit),
	})
	if err != nil {
		t.Fatalf("NewOrchestrator: %v", err)
	}
	t.Cleanup(func() {
		if err := orch.Close(); err != nil {
			t.Errorf("Close: %v", err)
		}
	})
	return orch
}

// countingWorker is the worker tier: the task executor every campaign task
// runs on.
func countingWorker(calls *atomic.Int32) *MockTaskExecutor {
	return &MockTaskExecutor{
		ExecuteFunc: func(context.Context, session.TaskRequest) (string, error) {
			calls.Add(1)
			return "done", nil
		},
	}
}

// promptRecord is one prompt a planning component asked for, and the client
// it said would consume it.
type promptRecord struct {
	role     CampaignRole
	servedBy perception.LLMClient
}

// recordingPromptProvider records every GetPrompt and serves the static
// prompt, so the components it is wired into behave as in production.
type recordingPromptProvider struct {
	mu      sync.Mutex
	records []promptRecord
}

func (p *recordingPromptProvider) GetPrompt(_ context.Context, role CampaignRole, _ string, servedBy perception.LLMClient) (string, error) {
	p.mu.Lock()
	p.records = append(p.records, promptRecord{role: role, servedBy: servedBy})
	p.mu.Unlock()
	return getStaticPrompt(role), nil
}

func (p *recordingPromptProvider) servedBy(role CampaignRole) []perception.LLMClient {
	p.mu.Lock()
	defer p.mu.Unlock()
	var out []perception.LLMClient
	for _, r := range p.records {
		if r.role == role {
			out = append(out, r.servedBy)
		}
	}
	return out
}

// The planning client is required: a door that forgets it must fail at
// construction, not plan on the execution client unnoticed.
func TestNewOrchestrator_RequiresAPlanningClient(t *testing.T) {
	var typedNil *tierLLM
	tests := []struct {
		name     string
		planning perception.LLMClient
	}{
		{name: "nil interface", planning: nil},
		{name: "typed nil", planning: typedNil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := NewOrchestrator(OrchestratorConfig{
				Workspace:         t.TempDir(),
				Kernel:            &MockKernel{},
				LLMClient:         &tierLLM{},
				PlanningLLMClient: tt.planning,
				TaskExecutor:      &MockTaskExecutor{},
				Executor:          tactile.NewDirectExecutor(),
				VirtualStore:      &core.VirtualStore{},
			})
			if !errors.Is(err, ErrNilDependency) {
				t.Fatalf("NewOrchestrator error = %v, want ErrNilDependency", err)
			}
			if !strings.Contains(err.Error(), "planning_llm_client") {
				t.Fatalf("the error must name the missing planning client; got %v", err)
			}
		})
	}
}

// Every planning component holds the planning client and every
// execution-side component the execution client.
func TestNewOrchestrator_WiresEachTierToItsComponents(t *testing.T) {
	execution, planning := &tierLLM{}, &tierLLM{}
	var workerCalls atomic.Int32
	orch := newTierOrchestrator(t, &MockKernel{}, execution, planning, countingWorker(&workerCalls), nil)

	tests := []struct {
		name string
		got  perception.LLMClient
		want perception.LLMClient
	}{
		{name: "decomposer", got: orch.decomposer.llmClient, want: planning},
		{name: "replanner", got: orch.replanner.llmClient, want: planning},
		{name: "assault remediation planning", got: orch.planningClient, want: planning},
		{name: "context pager", got: orch.contextPager.llmClient, want: execution},
		{name: "document fallback", got: orch.llmClient, want: execution},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s holds %p, want %p", tt.name, tt.got, tt.want)
			}
		})
	}
}

// Decomposition runs on the planning client, and its prompts are compiled for
// that client; the execution client and the worker see nothing.
func TestOrchestratorDecompose_RunsOnThePlanningClient(t *testing.T) {
	execution := &tierLLM{}
	planning := &tierLLM{system: func(string, string) string { return sampleRawPlanJSON("Tier routing plan") }}
	var workerCalls atomic.Int32
	orch := newTierOrchestrator(t, &MockKernel{}, execution, planning, countingWorker(&workerCalls), nil)
	prompts := &recordingPromptProvider{}
	orch.SetPromptProvider(prompts)

	res, err := orch.Decompose(context.Background(), DecomposeRequest{
		Goal:         "Route campaign planning to the planner tier",
		CampaignType: CampaignTypeCustom,
	})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if res == nil || res.Campaign == nil || len(res.Campaign.Phases) == 0 {
		t.Fatalf("Decompose returned no plan: %+v", res)
	}
	if planning.calls.Load() == 0 {
		t.Fatal("decomposition never called the planning client")
	}
	if n := execution.calls.Load(); n != 0 {
		t.Fatalf("decomposition called the execution client %d time(s)", n)
	}
	if n := workerCalls.Load(); n != 0 {
		t.Fatalf("decomposition ran %d task(s) on the worker", n)
	}
	served := prompts.servedBy(RolePlanner)
	if len(served) == 0 {
		t.Fatal("the planner prompt was never requested")
	}
	for _, client := range served {
		if client != planning {
			t.Fatalf("the planner prompt was compiled for %p, want the planning client %p", client, planning)
		}
	}
}

// A terminal task failure replans on the planning client.
func TestHandleTaskFailure_ReplansOnThePlanningClient(t *testing.T) {
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	execution := &tierLLM{}
	planning := &tierLLM{complete: func(string) string {
		return `{"success": true, "change_summary": "replanned on the planner", "retry_tasks": [], "skip_tasks": [], "add_tasks": [{"phase_id": "/phase_tier", "description": "Add the follow-up fix", "type": "/file_modify", "priority": "/high", "before_task": ""}], "modify_dependencies": []}`
	}}
	var workerCalls atomic.Int32
	orch := newTierOrchestrator(t, kernel, execution, planning, countingWorker(&workerCalls), func(c *config.CampaignConfig) { c.MaxTaskAttempts = 1 })
	now := time.Now()
	orch.campaign = &Campaign{
		ID: "/campaign_tier", Type: CampaignTypeCustom, Title: "tier", Goal: "g",
		Status: StatusActive, CreatedAt: now, UpdatedAt: now, TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_tier", CampaignID: "/campaign_tier", Name: "p", Status: PhaseInProgress,
			Tasks: []Task{{
				ID: "/task_tier", PhaseID: "/phase_tier", Status: TaskPending,
				Description: "Modify internal/foo/bar.go to add a doc comment", Type: TaskTypeFileModify,
			}},
		}},
	}
	if err := kernel.LoadFacts(orch.campaign.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}

	failTask(orch, "/task_tier", errors.New("terminal logic failure"))

	if n := planning.calls.Load(); n != 1 {
		t.Fatalf("the terminal failure replanned on the planning client %d time(s), want 1", n)
	}
	if n := execution.calls.Load(); n != 0 {
		t.Fatalf("replanning called the execution client %d time(s)", n)
	}
	if orch.campaign.RevisionNumber == 0 {
		t.Fatal("the replan was not applied")
	}
}

// Rolling-wave refinement compiles the replanner prompt for the planning
// client and asks it.
func TestRefineNextPhase_RunsOnThePlanningClient(t *testing.T) {
	execution := &tierLLM{}
	planning := &tierLLM{complete: func(string) string { return `{"tasks": [], "summary": "no change"}` }}
	var workerCalls atomic.Int32
	orch := newTierOrchestrator(t, &MockKernel{}, execution, planning, countingWorker(&workerCalls), nil)
	prompts := &recordingPromptProvider{}
	orch.SetPromptProvider(prompts)
	camp := &Campaign{
		ID: "/campaign_refine_tier", Type: CampaignTypeCustom, Title: "refine", Goal: "g", Status: StatusActive,
		Phases: []Phase{
			{ID: "/phase_done", Name: "done", Order: 0, Status: PhaseCompleted,
				Tasks: []Task{{ID: "/task_done", PhaseID: "/phase_done", Description: "research", Type: TaskTypeResearch, Status: TaskCompleted}}},
			{ID: "/phase_next", Name: "next", Order: 1, Status: PhasePending,
				Tasks: []Task{{ID: "/task_next", PhaseID: "/phase_next", Description: "document it", Type: TaskTypeDocument, Status: TaskPending, WriteSet: []string{"Docs/tier.md"}}}},
		},
	}

	if err := orch.replanner.RefineNextPhase(context.Background(), camp, &camp.Phases[0], nil); err != nil {
		t.Fatalf("RefineNextPhase: %v", err)
	}
	if n := planning.calls.Load(); n != 1 {
		t.Fatalf("refinement asked the planning client %d time(s), want 1", n)
	}
	if n := execution.calls.Load(); n != 0 {
		t.Fatalf("refinement called the execution client %d time(s)", n)
	}
	served := prompts.servedBy(RoleReplanner)
	if len(served) != 1 || served[0] != planning {
		t.Fatalf("the replanner prompt was compiled for %v, want only the planning client", served)
	}
}

// The assault's remediation plan is planning: it runs on the planning client
// with a prompt compiled for it.
func TestAssaultRemediationPlan_RunsOnThePlanningClient(t *testing.T) {
	execution := &tierLLM{}
	planning := &tierLLM{complete: func(string) string {
		return `{"summary": "one fix", "recommended_tasks": [{"type": "/shard_task", "priority": "/high", "description": "Fix the failing target", "shard": "coder", "shard_input": "fix it", "artifacts": ["logs/a.log"]}]}`
	}}
	var workerCalls atomic.Int32
	orch := newTierOrchestrator(t, &MockKernel{}, execution, planning, countingWorker(&workerCalls), nil)
	prompts := &recordingPromptProvider{}
	orch.SetPromptProvider(prompts)
	orch.campaign = &Campaign{ID: "/campaign_assault_tier", Type: CampaignTypeAdversarialAssault}

	tasks := orch.llmAssaultRemediationPlan(context.Background(), DefaultAssaultConfig(), "1 of 2 targets failed")

	if len(tasks) != 1 {
		t.Fatalf("remediation plan = %+v, want the planning client's one task", tasks)
	}
	if n := planning.calls.Load(); n != 1 {
		t.Fatalf("remediation planning asked the planning client %d time(s), want 1", n)
	}
	if n := execution.calls.Load(); n != 0 {
		t.Fatalf("remediation planning called the execution client %d time(s)", n)
	}
	served := prompts.servedBy(RoleAssault)
	if len(served) != 1 || served[0] != planning {
		t.Fatalf("the assault prompt was compiled for %v, want only the planning client", served)
	}
}

// Task execution runs on the task executor, the worker tier; neither LLM slot
// the orchestrator holds is asked.
func TestSpawnTask_RunsOnTheWorkerTier(t *testing.T) {
	execution, planning := &tierLLM{}, &tierLLM{}
	var workerCalls atomic.Int32
	orch := newTierOrchestrator(t, &MockKernel{}, execution, planning, countingWorker(&workerCalls), nil)

	if _, err := orch.spawnTask(context.Background(), nil, "/fix", "fix the failing test"); err != nil {
		t.Fatalf("spawnTask: %v", err)
	}
	if n := workerCalls.Load(); n != 1 {
		t.Fatalf("the task ran on the worker %d time(s), want 1", n)
	}
	if n := planning.calls.Load() + execution.calls.Load(); n != 0 {
		t.Fatalf("task execution called an orchestrator LLM slot %d time(s)", n)
	}
}

// recordingAssembler captures the prompt context the JIT provider compiles.
type recordingAssembler struct {
	got *articulation.PromptContext
}

func (a *recordingAssembler) AssembleSystemPrompt(_ context.Context, input any) (string, error) {
	pc, ok := input.(*articulation.PromptContext)
	if !ok {
		return "", errors.New("unexpected prompt context type")
	}
	a.got = pc
	return "compiled", nil
}

// The JIT provider compiles each role for the provider and model of the
// client that will consume it: the model-pinned atoms' fail-closed
// dimensions. A client that reports no identity leaves both unset.
func TestJITPromptProvider_CompilesForTheServingModel(t *testing.T) {
	planner := &identifiedLLM{provider: "anthropic", model: "planner-model"}
	tests := []struct {
		name         string
		servedBy     perception.LLMClient
		wantProvider string
		wantModel    string
	}{
		{name: "an identified client", servedBy: planner, wantProvider: "anthropic", wantModel: "planner-model"},
		{name: "an identity behind decorators", servedBy: &decoratorLLM{inner: &decoratorLLM{inner: planner}}, wantProvider: "anthropic", wantModel: "planner-model"},
		{name: "a client without identity", servedBy: &tierLLM{}},
		{name: "no client", servedBy: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assembler := &recordingAssembler{}
			provider := &jitPromptProvider{assembler: assembler}
			prompt, err := provider.GetPrompt(context.Background(), RolePlanner, "/campaign_pin", tt.servedBy)
			if err != nil {
				t.Fatalf("GetPrompt: %v", err)
			}
			if prompt != "compiled" {
				t.Fatalf("GetPrompt = %q, want the assembler's prompt", prompt)
			}
			pc := assembler.got
			if pc == nil || pc.SessionCtx == nil {
				t.Fatal("no prompt context reached the assembler")
			}
			if pc.ShardType != GetShardTypeForRole(RolePlanner) || pc.SessionCtx.CampaignPhase != GetCampaignPhaseForRole(RolePlanner) {
				t.Fatalf("prompt context = %+v, want the planner role's shard type and phase", pc)
			}
			if got := pc.SessionCtx.ExtraContext["provider"]; got != tt.wantProvider {
				t.Fatalf("provider = %q, want %q", got, tt.wantProvider)
			}
			if got := pc.SessionCtx.ExtraContext["model"]; got != tt.wantModel {
				t.Fatalf("model = %q, want %q", got, tt.wantModel)
			}
		})
	}
}

// No assembler means no JIT provider, so each role keeps its static prompt.
func TestNewJITPromptProvider_NilAssemblerLeavesTheStaticFallback(t *testing.T) {
	if provider := NewJITPromptProvider(nil); provider != nil {
		t.Fatalf("NewJITPromptProvider(nil) = %T, want nil", provider)
	}
}

// A requirement added to a running campaign is planned on the planning
// client and persisted; the replan trigger it records is consumed by the
// next replan instead of riding along in every later one.
func TestHandleNewRequirement_PlansOnThePlanningClientAndTheNextReplanConsumesItsTrigger(t *testing.T) {
	kernel, err := core.NewRealKernelWithWorkspace(t.TempDir())
	if err != nil {
		t.Fatalf("NewRealKernelWithWorkspace: %v", err)
	}
	execution := &tierLLM{}
	planning := &tierLLM{complete: func(prompt string) string {
		if strings.Contains(prompt, "New Requirement:") {
			return `{"new_tasks": [{"phase_order": 0, "description": "Research the added requirement", "type": "/research", "priority": "/high"}], "modified_tasks": [], "summary": "added the requirement"}`
		}
		return `{"success": true, "change_summary": "replanned", "retry_tasks": [], "skip_tasks": [], "add_tasks": [], "modify_dependencies": []}`
	}}
	var workerCalls atomic.Int32
	orch := newTierOrchestrator(t, kernel, execution, planning, countingWorker(&workerCalls), func(c *config.CampaignConfig) { c.MaxTaskAttempts = 1 })
	now := time.Now()
	orch.campaign = &Campaign{
		ID: "/campaign_requirement_tier", Type: CampaignTypeCustom, Title: "requirement", Goal: "g",
		Status: StatusActive, CreatedAt: now, UpdatedAt: now, TotalPhases: 1, TotalTasks: 1,
		Phases: []Phase{{
			ID: "/phase_requirement", CampaignID: "/campaign_requirement_tier", Name: "p", Status: PhaseInProgress,
			Tasks: []Task{{
				ID: "/task_requirement", PhaseID: "/phase_requirement", Status: TaskPending,
				Description: "Modify internal/foo/bar.go to add a doc comment", Type: TaskTypeFileModify,
			}},
		}},
	}
	if err := kernel.LoadFacts(orch.campaign.ToFacts()); err != nil {
		t.Fatalf("load campaign facts: %v", err)
	}

	if err := orch.HandleNewRequirement(context.Background(), "Also research the routing"); err != nil {
		t.Fatalf("HandleNewRequirement: %v", err)
	}
	if n := planning.calls.Load(); n != 1 {
		t.Fatalf("the requirement was planned on the planning client %d time(s), want 1", n)
	}
	if n := execution.calls.Load(); n != 0 {
		t.Fatalf("integrating the requirement called the execution client %d time(s)", n)
	}
	if got := len(orch.campaign.Phases[0].Tasks); got != 2 {
		t.Fatalf("phase has %d task(s) after the requirement, want 2", got)
	}
	snapshots, err := filepath.Glob(filepath.Join(orch.nerdDir, "campaigns", "*.json"))
	if err != nil {
		t.Fatalf("glob snapshots: %v", err)
	}
	if len(snapshots) == 0 {
		t.Fatal("the revised plan was not persisted")
	}
	if got := orch.replanner.getReplanTriggers(orch.campaign.ID); len(got) != 1 {
		t.Fatalf("replan triggers after the requirement = %+v, want the one /new_requirement trigger", got)
	}

	failTask(orch, "/task_requirement", errors.New("terminal logic failure"))

	if n := planning.calls.Load(); n != 2 {
		t.Fatalf("the terminal failure did not replan on the planning client (calls = %d, want 2)", n)
	}
	if got := orch.replanner.getReplanTriggers(orch.campaign.ID); len(got) != 0 {
		t.Fatalf("replan triggers after the replan = %+v, want none: the replan consumed them", got)
	}
}
