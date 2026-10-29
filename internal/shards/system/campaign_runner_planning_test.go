package system

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"codenerd/internal/campaign"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/northstar"
	"codenerd/internal/types"
)

// A resumed campaign plans on the planning client SetPlanning supplied and
// runs its execution-side calls on the client the shard itself was given,
// the worker.
func TestCampaignRunner_ResumedCampaignsPlanOnThePlanningClient(t *testing.T) {
	// The config carries a northstar observer whose shared guardian holds a
	// store under the workspace; reset the registry before TempDir cleanup.
	defer northstar.ResetGuardianRegistry()
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	worker := &mockLLMClient{response: "worker"}
	planner := &mockLLMClient{response: "planner"}
	shard := NewCampaignRunnerShard()
	shard.SetParentKernel(kernel)
	shard.SetLLMClient(worker)
	shard.SetPlanning(planner, nil)

	cfg, err := shard.orchestratorConfig(t.TempDir(), coreshards.NewShardManager())
	if err != nil {
		t.Fatalf("orchestratorConfig: %v", err)
	}
	defer func() {
		if err := cfg.NorthstarObserver.Close(); err != nil {
			t.Errorf("close observer: %v", err)
		}
	}()

	tests := []struct {
		name string
		got  types.LLMClient
		want types.LLMClient
	}{
		{name: "planning", got: cfg.PlanningLLMClient, want: planner},
		{name: "execution side", got: cfg.LLMClient, want: worker},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.want {
				t.Fatalf("%s client = %p, want %p", tt.name, tt.got, tt.want)
			}
		})
	}
	shard.mu.RLock()
	prompts := shard.prompts
	shard.mu.RUnlock()
	if prompts != nil {
		t.Fatalf("no assembler must leave no prompt provider, got %T", prompts)
	}
}

// Without a planning client the runner resumes nothing: a resumed campaign
// that failed could not replan.
func TestCampaignRunner_WithoutAPlanningClientResumesNothing(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel: %v", err)
	}
	workspace := t.TempDir()
	campaignsDir := filepath.Join(workspace, ".nerd", "campaigns")
	if err := os.MkdirAll(campaignsDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	data, err := json.Marshal(campaign.Campaign{ID: "/campaign_runnable", Status: campaign.StatusActive, UpdatedAt: time.Now()})
	if err != nil {
		t.Fatalf("marshal campaign: %v", err)
	}
	if err := os.WriteFile(filepath.Join(campaignsDir, "campaign_runnable.json"), data, 0o644); err != nil {
		t.Fatalf("write campaign: %v", err)
	}

	shard := NewCampaignRunnerShard()
	shard.SetParentKernel(kernel)
	shard.SetLLMClient(&mockLLMClient{response: "worker"})
	shard.SetWorkspaceRoot(workspace)
	shard.SetShardManager(coreshards.NewShardManager())

	shard.tick(context.Background())

	shard.mu.RLock()
	defer shard.mu.RUnlock()
	if shard.activeOrch != nil || !shard.lastStartAttempt.IsZero() {
		t.Fatalf("the runner tried to resume %q without a planning client", shard.activeCampaignID)
	}
}
