package perception

import (
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
		Worker:          &config.WorkerLLMConfig{Provider: "meta", Model: "muse-spark-1.2"},
		Planner:         &config.PlannerLLMConfig{Provider: "dashscope", Model: "qwen3.8-max"},
	}

	planner, err := NewPlannerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatalf("NewPlannerClientFromUserConfig: %v", err)
	}
	compat, ok := planner.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("expected *OpenAICompatClient, got %T", planner)
	}
	if compat.model != "qwen3.8-max" {
		t.Errorf("planner model = %q, want qwen3.8-max", compat.model)
	}

	worker, err := NewWorkerClientFromUserConfig(cfg)
	if err != nil {
		t.Fatalf("NewWorkerClientFromUserConfig: %v", err)
	}
	workerCompat, ok := worker.(*OpenAICompatClient)
	if !ok {
		t.Fatalf("expected *OpenAICompatClient for the worker, got %T", worker)
	}
	if workerCompat.model == compat.model {
		t.Error("worker and planner resolved to the same model; the two-tier split would be a no-op")
	}
}

func TestNewPlannerClientFromUserConfig_NoPlannerReturnsNil(t *testing.T) {
	cfg := &config.UserConfig{
		MetaAPIKey: "mt",
		Worker:     &config.WorkerLLMConfig{Provider: "meta", Model: "muse-spark-1.2"},
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
// for no benefit. Treat it as unset.
func TestGetPlannerLLMConfig_IdenticalToWorkerIsUnset(t *testing.T) {
	cfg := &config.UserConfig{
		MetaAPIKey: "mt",
		Worker:     &config.WorkerLLMConfig{Provider: "meta", Model: "muse-spark-1.2"},
		Planner:    &config.PlannerLLMConfig{Provider: "meta", Model: "muse-spark-1.2"},
	}
	if got := cfg.GetPlannerLLMConfig(); got != nil {
		t.Errorf("planner identical to worker should resolve to nil, got %+v", got)
	}

	cfg.Planner.Model = "muse-spark-1.2-pro"
	if got := cfg.GetPlannerLLMConfig(); got == nil {
		t.Error("a planner differing from the worker by model must resolve")
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
