package main

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/northstar"
	coresys "codenerd/internal/system"
)

// slotServer is one configured model endpoint, speaking the OpenAI chat
// completions protocol the Ollama client uses. It counts the plan proposals
// it receives (the decomposer's "Campaign Planner" request) and every other
// request separately.
type slotServer struct {
	*httptest.Server
	plans  atomic.Int32
	others atomic.Int32
}

func newSlotServer(t *testing.T, plan string) *slotServer {
	t.Helper()
	s := &slotServer{}
	s.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/chat/completions" {
			http.NotFound(w, r)
			return
		}
		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		content := "{}"
		if strings.Contains(string(body), "Campaign Planner") {
			s.plans.Add(1)
			content = plan
		} else {
			s.others.Add(1)
		}
		resp := map[string]any{
			"id":      "chatcmpl-tier",
			"object":  "chat.completion",
			"choices": []map[string]any{{"index": 0, "finish_reason": "stop", "message": map[string]any{"role": "assistant", "content": content}}},
			"usage":   map[string]any{"prompt_tokens": 10, "completion_tokens": 10, "total_tokens": 20},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Errorf("encode completion: %v", err)
		}
	}))
	t.Cleanup(s.Close)
	return s
}

// `nerd campaign start`'s own path, from a real Cortex boot: the planner slot
// config.json names is the client the plan is proposed to, and the main slot
// is not asked for it.
func TestCampaignStart_RealCortexBootPlansOnThePlannerSlot(t *testing.T) {
	defer northstar.ResetGuardianRegistry()
	mainSrv := newSlotServer(t, tierPlanJSON)
	plannerSrv := newSlotServer(t, tierPlanJSON)

	workspace := t.TempDir()
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatalf("mkdir .nerd: %v", err)
	}
	// Both slots resolved from config.json, as in production. Shard consults
	// are off so the pre-planning gather stays local and the test stays fast.
	cfg := map[string]any{
		"provider": "ollama",
		"ollama":   map[string]any{"endpoint": mainSrv.URL, "model": "main-test-model"},
		"planner":  map[string]any{"provider": "ollama", "endpoint": plannerSrv.URL, "model": "planner-test-model"},
		"campaign": map[string]any{"intelligence": map[string]any{"enable_shard_consult": false}},
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		t.Fatalf("encode config: %v", err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), data, 0o644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	ctx := context.Background()
	cortex, err := coresys.BootCortexWithConfig(ctx, coresys.BootConfig{
		Workspace: workspace,
		DisableSystemShards: []string{
			"constitution_gate", "perception_firewall", "executive_policy",
			"session_planner", "tactile_router", "campaign_runner", "mangle_repair", "legislator",
		},
	})
	if err != nil {
		t.Fatalf("BootCortexWithConfig: %v", err)
	}
	defer func() {
		if err := cortex.Close(); err != nil {
			t.Errorf("close cortex: %v", err)
		}
	}()
	if cortex.PlannerLLM() == nil {
		t.Fatal("the boot built no planner slot from config.json")
	}

	orchCfg, prompts, err := buildCampaignOrchestratorConfig(cortex, workspace, nil, nil)
	if err != nil {
		t.Fatalf("buildCampaignOrchestratorConfig: %v", err)
	}
	if orchCfg.PlanningLLMClient != cortex.PlannerLLM() {
		t.Fatal("the campaign does not plan on the Cortex's planner slot")
	}
	orchestrator, err := newCampaignOrchestrator(orchCfg, prompts)
	if err != nil {
		t.Fatalf("newCampaignOrchestrator: %v", err)
	}
	defer func() {
		if err := orchestrator.Close(); err != nil {
			t.Errorf("close orchestrator: %v", err)
		}
	}()

	res, err := orchestrator.Decompose(ctx, campaign.DecomposeRequest{
		Goal:         "Write the tier routing note",
		CampaignType: campaign.CampaignTypeCustom,
	})
	if err != nil {
		t.Fatalf("Decompose: %v", err)
	}
	if res == nil || res.Campaign == nil || len(res.Campaign.Phases) == 0 || res.Campaign.PlanDegraded {
		t.Fatalf("Decompose did not return the planner's plan: %+v", res)
	}
	if n := plannerSrv.plans.Load(); n == 0 {
		t.Fatal("the plan proposal never reached the planner slot's endpoint")
	}
	if n := mainSrv.plans.Load(); n != 0 {
		t.Fatalf("the plan proposal reached the main slot's endpoint %d time(s)", n)
	}
}
