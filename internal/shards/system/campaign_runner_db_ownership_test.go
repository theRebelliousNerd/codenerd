package system

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/northstar"
	"codenerd/internal/observation"
	"codenerd/internal/prompt"
	prsync "codenerd/internal/prompt/sync"
	"codenerd/internal/session"
	"codenerd/internal/types"
)

type ownershipAdvisor struct {
	*coreshards.BaseShardAgent
	calls *atomic.Int32
}

func (a *ownershipAdvisor) Execute(context.Context, string) (string, error) {
	a.calls.Add(1)
	return "VOTE: REJECT\nCONFIDENCE: 100\nREASONING:\nKeep this protected change blocked.", nil
}

// The risk gate must stop before a task is run. Embedding the unused methods
// keeps this fixture focused on the actual observed execution boundary.
type ownershipBlockedExecutor struct {
	session.TaskExecutor
	calls atomic.Int32
}

func (e *ownershipBlockedExecutor) ExecuteObserved(context.Context, session.TaskRequest) (observation.Return, error) {
	e.calls.Add(1)
	return observation.Return{}, errors.New("protected campaign reached task execution")
}

func (e *ownershipBlockedExecutor) ExecuteObservedWithContext(ctx context.Context, req session.TaskRequest, _ *types.SessionContext, _ types.SpawnPriority) (observation.Return, error) {
	return e.ExecuteObserved(ctx, req)
}

func TestCampaignRunnerAdmittedRiskConsultationKeepsOwnerDB(t *testing.T) {
	defer northstar.ResetGuardianRegistry()
	ws := t.TempDir()
	dir := filepath.Join(ws, ".nerd", "agents", "coder")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "prompts.yaml"), []byte("- id: coder/identity\n  category: identity\n  is_mandatory: true\n  content: OWNED_CODER_CAMPAIGN_MARKER\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ws, "nerd.md"), []byte("---\nschema: nerd/v1\ncritical:\n  - protected\n---\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	syncer := prsync.NewAgentSynchronizer(ws, prompt.NewAtomLoader(nil))
	if err := syncer.SyncAll(t.Context()); err != nil {
		t.Fatal(err)
	}
	dbPath := types.AgentKnowledgeDBPath(filepath.Join(ws, ".nerd"), "coder")
	compiler, err := prompt.NewJITPromptCompiler()
	if err != nil {
		t.Fatal(err)
	}
	defer compiler.Close()
	if err := prompt.RegisterAgentDBWithJIT(compiler, "coder", dbPath); err != nil {
		t.Fatal(err)
	}
	owner, _ := compiler.LookupShardDB("coder")
	if owner == nil {
		t.Fatal("missing owner registration")
	}
	manager := coreshards.NewShardManager()
	manager.SetNerdDir(filepath.Join(ws, ".nerd"))
	manager.SetJITRegistrar(prompt.CreateJITDBRegistrar(compiler))
	var calls atomic.Int32
	for _, name := range []string{"coder", "tester", "reviewer", "researcher"} {
		manager.RegisterShard(name, func(id string, cfg types.ShardConfig) types.ShardAgent {
			return &ownershipAdvisor{BaseShardAgent: coreshards.NewBaseShardAgent(id, cfg), calls: &calls}
		})
		lifecycle := types.ShardTypeEphemeral
		if name == "coder" {
			lifecycle = types.ShardTypeUser
		}
		manager.DefineProfile(name, types.ShardConfig{Name: name, Type: lifecycle, Timeout: 5 * time.Second})
	}
	kernel, err := core.NewRealKernelWithWorkspace(ws)
	if err != nil {
		t.Fatal(err)
	}
	runner := NewCampaignRunnerShard()
	runner.SetParentKernel(kernel)
	runner.SetVirtualStore(&core.VirtualStore{})
	client := &mockLLMClient{response: "controlled campaign fixture"}
	runner.SetLLMClient(client)
	runner.SetPlanning(client, nil)
	executor := &ownershipBlockedExecutor{}
	runner.SetTaskExecutor(executor)
	newOrchestrator := func() *campaign.Orchestrator {
		cfg, err := runner.orchestratorConfig(ws, manager)
		if err != nil {
			t.Fatal(err)
		}
		// Select the audit's gated branch while retaining the production board,
		// consultation engine and manager adapter. No acceptance gate is bypassed.
		gated := true
		cfg.CampaignRiskOverride = &gated
		cfg.AdvisoryGateToggle = campaign.RiskGateToggleEnabled
		cfg.EdgeGateToggle = campaign.RiskGateToggleDisabled
		cfg.NorthstarGateToggle = campaign.RiskGateToggleDisabled
		cfg.RiskIntelligenceTimeout = time.Millisecond
		orch, err := campaign.NewOrchestrator(cfg)
		if err != nil {
			t.Fatal(err)
		}
		return orch
	}
	seed := newOrchestrator()
	now := time.Now()
	camp := &campaign.Campaign{
		ID: "/campaign_db_ownership", Type: campaign.CampaignTypeCustom,
		Title: "database ownership", Goal: "review a protected change", Status: campaign.StatusPaused,
		CreatedAt: now, UpdatedAt: now, TotalPhases: 1, TotalTasks: 1,
		Phases: []campaign.Phase{{ID: "/phase_db_ownership", CampaignID: "/campaign_db_ownership", Name: "review", Status: campaign.PhasePending,
			Tasks: []campaign.Task{{ID: "/task_db_ownership", PhaseID: "/phase_db_ownership", Type: campaign.TaskTypeFileModify,
				Description: "review the protected change", Status: campaign.TaskPending, WriteSet: []string{"protected/service.go"}}}}},
	}
	if err := seed.SetCampaign(camp); err != nil {
		_ = seed.Close()
		t.Fatal(err)
	}
	if err := seed.Close(); err != nil {
		t.Fatal(err)
	}
	orch := newOrchestrator()
	defer orch.Close()
	report, err := orch.ResumeCampaign(camp.ID, campaign.AdmissionPolicy{})
	if err != nil || !report.Admitted {
		t.Fatalf("campaign admission: report=%+v error=%v", report, err)
	}
	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	err = orch.Run(ctx)
	var blocked *campaign.RiskBlockedError
	if !errors.As(err, &blocked) {
		t.Fatalf("Run = %v, want a risk gate block", err)
	}
	advisoryReached := false
	for _, result := range blocked.Evaluation.Results {
		if result.Name == campaign.RiskGateAdvisory && result.Enabled && result.Outcome == campaign.RiskGateOutcomeBlocked {
			advisoryReached = true
		}
	}
	if !advisoryReached || calls.Load() < 4 || executor.calls.Load() != 0 {
		t.Fatalf("advisoryReached=%v advisor calls=%d task calls=%d", advisoryReached, calls.Load(), executor.calls.Load())
	}
	assertCurrentOwnerDB(t, compiler, owner)
}

func assertCurrentOwnerDB(t *testing.T, compiler *prompt.JITPromptCompiler, owner *sql.DB) {
	t.Helper()
	current, ok := compiler.LookupShardDB("coder")
	if !ok || current != owner {
		t.Fatal("resumed campaign consultation retired the registry owner's database")
	}
	if err := owner.PingContext(t.Context()); err != nil {
		t.Fatal(err)
	}
}
