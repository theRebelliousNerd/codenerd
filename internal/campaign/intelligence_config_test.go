package campaign

import (
	"testing"
	"time"

	"codenerd/internal/config"
	"codenerd/internal/core"
	"codenerd/internal/tactile"
)

// The campaign section is the gatherer's policy. NewOrchestrator stamps it
// after either wire, so a gatherer built on the package defaults — what every
// CLI site constructs, then hands over — does not keep those defaults once
// the section says otherwise.
func TestNewOrchestrator_MaxChurnHotspotsReachesTheGatherer(t *testing.T) {
	off := false
	section := testCampaignConfig(func(c *config.CampaignConfig) {
		c.Intelligence.MaxChurnHotspots = 17
		c.Intelligence.EnableGitHistory = &off
	})

	assertGatherer := func(t *testing.T, g *IntelligenceGatherer) {
		t.Helper()
		if g == nil {
			t.Fatal("no gatherer")
		}
		cfg := g.config
		if cfg.MaxChurnHotspots != 17 {
			t.Errorf("max_churn_hotspots = %d, want the section's 17", cfg.MaxChurnHotspots)
		}
		if cfg.EnableGitHistory {
			t.Error("enable_git_history: the section's false was left at the default true")
		}
		if !cfg.EnableWorldModel || cfg.MaxLearnings != 100 || cfg.PerSystemTimeout != 30*time.Second || cfg.ConsultTimeout != 2*time.Minute {
			t.Errorf("keys the section left out did not stay the defaults: %+v", cfg)
		}
	}

	t.Run("supplied", func(t *testing.T) {
		supplied := NewIntelligenceGatherer("", &MockKernel{}, nil, nil, nil, nil, nil, nil, nil)
		supplied.config.MaxChurnHotspots = 999
		orch, err := NewOrchestrator(OrchestratorConfig{
			Workspace:            t.TempDir(),
			Kernel:               &MockKernel{},
			LLMClient:            &MockLLMClient{},
			TaskExecutor:         &MockTaskExecutor{},
			Executor:             tactile.NewDirectExecutor(),
			VirtualStore:         &core.VirtualStore{},
			Campaign:             section,
			IntelligenceGatherer: supplied,
		})
		if err != nil {
			t.Fatalf("NewOrchestrator: %v", err)
		}
		if orch.intelligenceGatherer != supplied {
			t.Fatal("stamping replaced the supplied gatherer")
		}
		if orch.decomposer == nil || orch.decomposer.intelligence != supplied {
			t.Fatal("the supplied gatherer did not reach the decomposer")
		}
		assertGatherer(t, supplied)
	})

	t.Run("default wire", func(t *testing.T) {
		kernel, err := core.NewRealKernel()
		if err != nil {
			t.Skipf("real kernel unavailable: %v", err)
		}
		orch, err := NewOrchestrator(OrchestratorConfig{
			Workspace:    t.TempDir(),
			Kernel:       kernel,
			LLMClient:    &MockLLMClient{},
			TaskExecutor: &MockTaskExecutor{},
			Executor:     tactile.NewDirectExecutor(),
			VirtualStore: &core.VirtualStore{},
			Campaign:     section,
		})
		if err != nil {
			t.Fatalf("NewOrchestrator: %v", err)
		}
		assertGatherer(t, orch.intelligenceGatherer)
		if orch.decomposer == nil || orch.decomposer.intelligence != orch.intelligenceGatherer {
			t.Fatal("the default-wired gatherer did not reach the decomposer")
		}
	})
}
