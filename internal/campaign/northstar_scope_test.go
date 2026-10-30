package campaign

import (
	"context"
	"errors"
	"path/filepath"
	"strings"
	"testing"

	"codenerd/internal/core"
	"codenerd/internal/northstar"
	"codenerd/internal/observation"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

type alignmentSnapshot struct {
	facts map[string][]types.Fact
	err   error
}

func (q *alignmentSnapshot) Query(pred string) ([]types.Fact, error) { return q.facts[pred], q.err }
func (q *alignmentSnapshot) QueryAll() (map[string][]types.Fact, error) {
	out := map[string][]types.Fact{}
	for _, pred := range []string{"module_northstar", "effective_module_purpose", "module_requirement"} {
		out[pred] = q.facts[pred]
	}
	return out, q.err
}

type campaignScopeClient struct{ users, systems []string }

func (c *campaignScopeClient) CompleteWithSystem(_ context.Context, system, user string) (string, error) {
	c.systems = append(c.systems, system)
	c.users = append(c.users, user)
	return "SCORE: 0.95\nRESULT: passed", nil
}

// This is the production gate and observer constructor, with a real kernel
// deriving the inherited purpose from the project mission. Before the repair,
// the prose goal reached ModuleForPath and the requirement never reached the LLM.
func TestNorthstarRiskGate_ProseGoalCarriesKernelModuleRefinement(t *testing.T) {
	workspace := t.TempDir()
	store, err := northstar.NewStore(filepath.Join(workspace, ".nerd"))
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SaveVision(&northstar.Vision{Mission: "Project mission", Constraints: []string{"Global constraint"}}); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	kernel, err := core.NewRealKernelWithWorkspace(workspace)
	if err != nil {
		t.Fatal(err)
	}
	if err := kernel.LoadFacts([]types.Fact{
		{Predicate: "module_requirement", Args: []any{"pkg/a", "REQ-INHERITED", "Keep cancellation observable", types.MangleAtom("/blocker")}},
	}); err != nil {
		t.Fatal(err)
	}
	client := &campaignScopeClient{}
	observer := northstar.BuildCampaignObserver(workspace, client, kernel)
	if observer == nil {
		t.Fatal("observer unavailable")
	}
	t.Cleanup(func() { _ = observer.Close() })
	c := riskContractCampaign("/campaign_scope", "pkg/a/file.go")
	c.Goal = "Repair cancellation behavior"
	o := &Orchestrator{workspace: workspace, campaign: c, northstarObserver: observer}
	res, concerns := o.runNorthstarRiskGate(context.Background())
	if res.Outcome != RiskGateOutcomePassed || len(concerns) != 0 {
		t.Fatalf("gate: %+v, %+v", res, concerns)
	}
	if len(client.users) != 1 {
		t.Fatalf("completions = %d", len(client.users))
	}
	for _, want := range []string{"Project mission", "Global constraint", "## Module Northstar (pkg/a)", "**Purpose:** Project mission", "REQ-INHERITED", "Keep cancellation observable", "Repair cancellation behavior"} {
		if !strings.Contains(client.users[0], want) {
			t.Errorf("production gate request lacks %q", want)
		}
	}
	coverage, ok := res.Data["coverage"].(*northstar.AlignmentCoverage)
	if !ok || !coverage.Complete || len(coverage.Modules) != 1 {
		t.Fatalf("gate coverage = %+v", res.Data)
	}
}

func TestNorthstarRiskGate_RecurseFixAndEveryImprovementCarryNodePaths(t *testing.T) {
	for _, angle := range []string{"", "stabilize", "harden", "simplify", "extend"} {
		t.Run("angle="+angle, func(t *testing.T) {
			workspace := t.TempDir()
			store, err := northstar.NewStore(filepath.Join(workspace, ".nerd"))
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = store.Close() })
			g := northstar.NewGuardian(store, northstar.DefaultGuardianConfig())
			if err := g.Initialize(); err != nil {
				t.Fatal(err)
			}
			if err := g.UpdateVision(&northstar.Vision{Mission: "Project mission"}); err != nil {
				t.Fatal(err)
			}
			g.SetQuerier(&alignmentSnapshot{facts: map[string][]types.Fact{
				"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"pkg/a", "Node purpose"}}},
				"module_requirement":       {{Predicate: "module_requirement", Args: []any{"pkg/a", "REQ-1", "Node requirement", "/major"}}},
			}})
			client := &campaignScopeClient{}
			g.SetLLMClient(client)
			paths := []string{"pkg/a"}
			c := RecurseAttemptCampaign(workspace, RecurseAttempt{Angle: angle, Node: SubsystemNode{ID: "a", Title: "cancellation", Paths: paths}})
			// The campaign owns its scope after construction; a caller mutating
			// its node slice must not rewrite the attempt's alignment identity.
			paths[0] = "pkg/b"
			o := &Orchestrator{workspace: workspace, campaign: c, northstarObserver: northstar.NewCampaignObserver(g)}
			res, _ := o.runNorthstarRiskGate(context.Background())
			if res.Outcome != RiskGateOutcomePassed || len(client.users) != 1 || !strings.Contains(client.users[0], "Node requirement") {
				t.Fatalf("recurse scope lost: %+v, %v", res, client.users)
			}
		})
	}
}

func TestNorthstarRiskGate_UnavailableModuleEvidencePreservesProjectVerdict(t *testing.T) {
	workspace := t.TempDir()
	store, err := northstar.NewStore(filepath.Join(workspace, ".nerd"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	g := northstar.NewGuardian(store, northstar.DefaultGuardianConfig())
	if err := g.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateVision(&northstar.Vision{Mission: "Project mission"}); err != nil {
		t.Fatal(err)
	}
	g.SetQuerier(&alignmentSnapshot{err: errors.New("snapshot unavailable")})
	client := &campaignScopeClient{}
	g.SetLLMClient(client)
	o := &Orchestrator{workspace: workspace, campaign: riskContractCampaign("/campaign_unknown_scope", "pkg/a/file.go"), northstarObserver: northstar.NewCampaignObserver(g)}
	res, _ := o.runNorthstarRiskGate(context.Background())
	if res.Outcome != RiskGateOutcomePassed || len(client.users) != 1 || !strings.Contains(client.users[0], "Project mission") || strings.Contains(client.users[0], "## Module Northstar") {
		t.Fatalf("scope suppressed project assessment: %+v; requests=%v", res, client.users)
	}
	coverage, ok := res.Data["coverage"].(*northstar.AlignmentCoverage)
	if !ok || coverage.Outcome != northstar.ScopeUnknown || coverage.Complete || len(coverage.Issues) != 1 || !strings.Contains(coverage.Issues[0], "snapshot unavailable") {
		t.Fatalf("gate lost unknown scope and reason: %+v", res.Data)
	}
}

func TestCompleteTask_ObservedEffectsAndArtifactsReachGuardianSeparately(t *testing.T) {
	o := newSnapshotTestOrchestrator()
	o.workspace = t.TempDir()
	store, err := northstar.NewStore(filepath.Join(o.workspace, ".nerd"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	cfg := northstar.DefaultGuardianConfig()
	cfg.PeriodicCheckInterval = 1
	g := northstar.NewGuardian(store, cfg)
	if err := g.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateVision(&northstar.Vision{Mission: "Project mission"}); err != nil {
		t.Fatal(err)
	}
	g.SetQuerier(&alignmentSnapshot{facts: map[string][]types.Fact{
		"effective_module_purpose": {
			{Predicate: "effective_module_purpose", Args: []any{"pkg/a", "Artifact purpose"}},
			{Predicate: "effective_module_purpose", Args: []any{"pkg/b", "Observed purpose"}},
		},
	}})
	client := &campaignScopeClient{}
	g.SetLLMClient(client)
	o.northstarObserver = northstar.NewCampaignObserver(g)
	task := &o.campaign.Phases[0].Tasks[0]
	task.Artifacts = []TaskArtifact{{Path: "pkg/a/report.md"}}
	o.taskExecutor = &MockTaskExecutor{ExecuteObservedFunc: func(context.Context, session.TaskRequest) (observation.Return, error) {
		return observation.Return{Output: "done", Outcome: "/done", Changed: []string{"pkg/b/actual.go"}, Writes: []observation.FileWrite{{Path: filepath.Join(o.workspace, "pkg/b/actual.go")}}}, nil
	}}
	result, err := o.spawnTask(context.Background(), task, "/fix", "Repair cancellation")
	if err != nil {
		t.Fatal(err)
	}
	o.completeTask(task, result)
	if len(client.users) != 1 {
		t.Fatalf("completions = %d", len(client.users))
	}
	for _, want := range []string{"[artifact] pkg/a/report.md", "[observed_effect] pkg/b/actual.go", "Artifact purpose", "Observed purpose"} {
		if !strings.Contains(client.users[0], want) {
			t.Errorf("task request lacks %q", want)
		}
	}
	observations, err := store.GetRecentObservations(20)
	if err != nil {
		t.Fatal(err)
	}
	for _, obs := range observations {
		if obs.Type == northstar.ObsFileChanged && obs.Subject == "pkg/a/report.md" {
			t.Fatal("artifact recorded as observed modification")
		}
	}
}

func TestStartNextPhase_TransportsPhaseScopeThroughObserver(t *testing.T) {
	o := newSnapshotTestOrchestrator()
	o.workspace = t.TempDir()
	store, err := northstar.NewStore(filepath.Join(o.workspace, ".nerd"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	g := northstar.NewGuardian(store, northstar.DefaultGuardianConfig())
	if err := g.Initialize(); err != nil {
		t.Fatal(err)
	}
	if err := g.UpdateVision(&northstar.Vision{Mission: "Project mission"}); err != nil {
		t.Fatal(err)
	}
	g.SetQuerier(&alignmentSnapshot{facts: map[string][]types.Fact{
		"effective_module_purpose": {{Predicate: "effective_module_purpose", Args: []any{"pkg/b", "Phase purpose"}}},
	}})
	client := &campaignScopeClient{}
	g.SetLLMClient(client)
	o.northstarObserver = northstar.NewCampaignObserver(g)
	// With no prior assessed campaign coverage, the first phase needs its own check.
	phase := &o.campaign.Phases[0]
	phase.Name = "Repair cancellation"
	phase.Tasks[0].WriteSet = []string{"pkg/b/phase.go"}
	if err := o.kernel.Assert(core.Fact{Predicate: "phase_eligible", Args: []any{phase.ID}}); err != nil {
		t.Fatal(err)
	}
	if err := o.startNextPhase(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(client.users) != 1 || !strings.Contains(client.users[0], "Phase purpose") || !strings.Contains(client.users[0], "[planned_write] pkg/b/phase.go") {
		t.Fatalf("phase scope lost: %v", client.users)
	}
}
