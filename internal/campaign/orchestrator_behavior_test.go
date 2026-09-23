package campaign

import (
	"context"
	"fmt"
	"testing"

	"codenerd/internal/config"
	"codenerd/internal/core"
	coreshards "codenerd/internal/core/shards"
	"codenerd/internal/tactile"
	"codenerd/internal/types"
)

// stubLLM implements perception.LLMClient for unit tests.
type stubLLM struct{}

func (s *stubLLM) Complete(ctx context.Context, prompt string) (string, error) {
	return "ok", nil
}
func (s *stubLLM) CompleteWithSystem(ctx context.Context, systemPrompt, userPrompt string) (string, error) {
	return "ok", nil
}
func (s *stubLLM) CompleteWithTools(ctx context.Context, systemPrompt, userPrompt string, tools []types.ToolDefinition) (*types.LLMToolResponse, error) {
	return &types.LLMToolResponse{Text: "ok", StopReason: "end_turn"}, nil
}
func (s *stubLLM) CompleteWithStreaming(ctx context.Context, systemPrompt, userPrompt string, enableThinking bool) (<-chan string, <-chan error) {
	ch := make(chan string, 1)
	ch <- "ok"
	close(ch)
	errCh := make(chan error)
	close(errCh)
	return ch, errCh
}

// The campaign section of the user's config reaches the kernel as
// config_param rows, and the rules read those rows: nothing about how a
// campaign runs is a Go literal or a Mangle constant.
func TestOrchestrator_PublishesTheCampaignPolicyAsConfigParams(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	cfg := config.DefaultCampaignConfig()
	cfg.MaxTaskAttempts = 5
	cfg.AcceptanceRounds = 2
	no := false
	cfg.ReplanOnCheckpointFailure = &no

	if _, err := NewOrchestrator(OrchestratorConfig{
		Workspace:    t.TempDir(),
		Kernel:       kernel,
		LLMClient:    &stubLLM{},
		ShardManager: coreshards.NewShardManager(),
		TaskExecutor: &MockTaskExecutor{},
		Executor:     tactile.NewDirectExecutor(),
		VirtualStore: &core.VirtualStore{},
		Campaign:     cfg,
	}); err != nil {
		t.Fatalf("NewOrchestrator() error = %v", err)
	}

	facts, err := kernel.Query("config_param")
	if err != nil {
		t.Fatalf("Query(config_param) error = %v", err)
	}
	got := map[string]string{}
	for _, f := range facts {
		if len(f.Args) == 2 {
			got[types.ExtractString(f.Args[0])] = fmt.Sprintf("%v", f.Args[1])
		}
	}
	for key, want := range map[string]string{
		"/campaign_max_task_attempts":            "5",
		"/campaign_acceptance_rounds":            "2",
		"/campaign_replan_on_checkpoint_failure": "0",
		"/campaign_max_checkpoint_attempts":      "3",
	} {
		if got[key] != want {
			t.Errorf("config_param(%s) = %q, want %q (all: %v)", key, got[key], want, got)
		}
	}

	missing, err := kernel.Query("config_param_missing")
	if err != nil {
		t.Fatalf("Query(config_param_missing) error = %v", err)
	}
	for _, f := range missing {
		if len(f.Args) == 2 && types.ExtractString(f.Args[0]) == "/campaign" {
			t.Errorf("a campaign rule requires %v and the orchestrator did not publish it", f.Args[1])
		}
	}
}

// A threshold a rule needs and the kernel does not hold is named, so the
// orchestrator can refuse to run instead of running on a rule that fails open.
func TestKernel_AMissingCampaignThresholdIsNamed(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}
	facts, err := kernel.Query("config_param_missing")
	if err != nil {
		t.Fatalf("Query(config_param_missing) error = %v", err)
	}
	named := false
	for _, f := range facts {
		if len(f.Args) == 2 && types.ExtractString(f.Args[0]) == "/campaign" &&
			types.ExtractString(f.Args[1]) == "/campaign_acceptance_rounds" {
			named = true
		}
	}
	if !named {
		t.Fatalf("with no config_param rows the kernel does not name /campaign_acceptance_rounds as missing: %v", facts)
	}
}

func TestContextPager_ResetPhaseContextClearsFacts(t *testing.T) {
	kernel, err := core.NewRealKernel()
	if err != nil {
		t.Fatalf("NewRealKernel() error = %v", err)
	}

	cp := NewContextPager(kernel, &stubLLM{}, 0) // 0 uses default budget
	_ = kernel.Assert(core.Fact{
		Predicate: "activation",
		Args:      []any{"file_pattern(\"**/*\")", 100},
	})
	_ = kernel.Assert(core.Fact{
		Predicate: "phase_context_atom",
		Args:      []any{"/phase1", "file_topology(\"x\",_,_,_,_)", 120},
	})

	cp.ResetPhaseContext()

	act, _ := kernel.Query("activation")
	if len(act) != 0 {
		t.Fatalf("expected activation facts cleared, got %v", act)
	}
	phaseAtoms, _ := kernel.Query("phase_context_atom")
	if len(phaseAtoms) != 0 {
		t.Fatalf("expected phase_context_atom facts cleared, got %v", phaseAtoms)
	}
}
