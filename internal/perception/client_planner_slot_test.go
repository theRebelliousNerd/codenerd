package perception

import (
	"codenerd/internal/broker"
	"strings"
	"testing"

	"codenerd/internal/config"
)

// The planner slot exists so a two-tier stack is expressible: an expensive
// reasoning model plus a cheap bulk model. It resolves through the same shared
// factory as the worker, so it supports every provider the main client does.
func TestNewPlannerClientFromUserConfig_BuildsDistinctClient(t *testing.T) {
	cfg := &config.UserConfig{
		DashScopeAPIKey: "ds",
		MetaAPIKey:      "mt",
		Worker:          &config.WorkerLLMConfig{Provider: "meta", Model: metaContributorModel},
		Planner:         &config.PlannerLLMConfig{Provider: "dashscope", Model: "qwen3.8-max"},
	}

	planner, err := NewPlannerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatalf("NewPlannerClientFromUserConfig: %v", err)
	}
	compat, ok := broker.Base(planner).(*OpenAICompatClient)
	if !ok {
		t.Fatalf("expected *OpenAICompatClient, got %T", broker.Base(planner))
	}
	if compat.model != "qwen3.8-max" {
		t.Errorf("planner model = %q, want qwen3.8-max", compat.model)
	}

	worker, err := NewWorkerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatalf("NewWorkerClientFromUserConfig: %v", err)
	}
	workerCompat, ok := broker.Base(worker).(*OpenAICompatClient)
	if !ok {
		t.Fatalf("expected *OpenAICompatClient for the worker, got %T", broker.Base(worker))
	}
	if workerCompat.model == compat.model {
		t.Error("worker and planner resolved to the same model; the two-tier split would be a no-op")
	}
}

func TestNewPlannerClientFromUserConfig_NoPlannerReturnsNil(t *testing.T) {
	cfg := &config.UserConfig{
		MetaAPIKey: "mt",
		Worker:     &config.WorkerLLMConfig{Provider: "meta", Model: metaContributorModel},
	}
	client, err := NewPlannerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client != nil {
		t.Error("no planner block must yield a nil client so reasoning work stays on the worker")
	}
}

// A planner identical to the worker buys nothing but a second connection pool,
// and a non-nil planner makes every reasoning-intensive turn pay a kernel query
// for no benefit. Treat it as unset. Identical means as resolved: the planner's
// default effort (high) differs from the worker's (medium), so a planner that
// names the worker's model and no effort is the same model's high-effort tier
// and stays.
func TestGetPlannerLLMConfig_IdenticalToWorkerIsUnset(t *testing.T) {
	tests := []struct {
		name    string
		worker  *config.WorkerLLMConfig
		planner *config.PlannerLLMConfig
		wantNil bool
	}{
		{name: "identical including effort", wantNil: true,
			worker:  &config.WorkerLLMConfig{Provider: "meta", Model: metaContributorModel, ReasoningEffort: "high"},
			planner: &config.PlannerLLMConfig{Provider: "meta", Model: metaContributorModel, ReasoningEffort: "high"}},
		{name: "same model, default efforts differ",
			worker:  &config.WorkerLLMConfig{Provider: "meta", Model: metaContributorModel},
			planner: &config.PlannerLLMConfig{Provider: "meta", Model: metaContributorModel}},
		{name: "a different model",
			worker:  &config.WorkerLLMConfig{Provider: "meta", Model: metaContributorModel, ReasoningEffort: "high"},
			planner: &config.PlannerLLMConfig{Provider: "meta", Model: "muse-spark-1.2-pro", ReasoningEffort: "high"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := &config.UserConfig{MetaAPIKey: "mt", Worker: tt.worker, Planner: tt.planner}
			if got := cfg.GetPlannerLLMConfig(); (got == nil) != tt.wantNil {
				t.Errorf("GetPlannerLLMConfig() = %+v, want nil=%v", got, tt.wantNil)
			}
		})
	}
}

func TestNewPlannerClientFromUserConfig_MissingKeyNamesTheSlot(t *testing.T) {
	cfg := &config.UserConfig{
		Planner: &config.PlannerLLMConfig{Provider: "dashscope", Model: "qwen3.8-max"},
	}
	_, err := NewPlannerClientFromUserConfig(cfg)
	if err == nil {
		t.Fatal("expected an error when the planner provider's key is missing")
	}
	// The message must say which slot failed and which field to set — a bare
	// "provider=dashscope but key empty" reads as a main-client problem.
	if !strings.Contains(err.Error(), "planner") || !strings.Contains(err.Error(), "dashscope_api_key") {
		t.Errorf("error should name the slot and the missing field, got: %v", err)
	}
}
