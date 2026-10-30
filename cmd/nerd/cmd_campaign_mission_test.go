package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"codenerd/internal/campaign"
	"codenerd/internal/config"
	"codenerd/internal/deliberation"
	"codenerd/internal/gates"
	"codenerd/internal/observation"
	"codenerd/internal/sessionlog"
	coresys "codenerd/internal/system"
	"codenerd/internal/types"
	"codenerd/internal/usage"
)

const missionFixtureModel = "mission-cycle-script"
const missionFixedSource = "package store\n\nfunc Get() int { return 2 }\n"

// Only model responses are scripted. Both deliberation roles are registered
// shards, read_file/write_file/run_check are real tools, the worker is the
// booted session TaskExecutor, and acceptance and ratchet execute real checks.
type missionCycleModel struct {
	mu           sync.Mutex
	proposerRead bool
	workerRound  int
	roles        map[string]int
}

func (c *missionCycleModel) report(ctx context.Context) {
	usage.RecordFromContext(ctx, usage.Report{Model: missionFixtureModel, Provider: "script", Operation: "chat",
		Usage: types.UsageMetadata{InputTokens: 10, OutputTokens: 10, Known: types.UsageTokens}})
}

func (c *missionCycleModel) Complete(ctx context.Context, prompt string) (string, error) {
	return c.CompleteWithSystem(ctx, "", prompt)
}
func (c *missionCycleModel) CompleteWithSystem(ctx context.Context, system, _ string) (string, error) {
	c.report(ctx)
	switch {
	case strings.Contains(system, "You divide one code-change task"):
		return "STEP store/store.go :: restore the tested return value", nil
	case strings.Contains(system, "adversarial code reviewer"):
		return "NO FINDINGS", nil
	case strings.Contains(system, "INCOMPLETE"):
		return "COMPLETE", nil
	default:
		return `{"contract":"judge/v2","status":"pass","confidence":0.99,"explanation":"the host witness checks the result"}`, nil
	}
}
func (c *missionCycleModel) CompleteWithStreaming(context.Context, string, string, bool) (<-chan string, <-chan types.StreamResult) {
	return types.FailedStream(fmt.Errorf("fixture uses the production native tool path"))
}
func (c *missionCycleModel) CompleteWithTools(ctx context.Context, system, input string, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	c.report(ctx)
	c.mu.Lock()
	defer c.mu.Unlock()
	var frame struct {
		Frame        deliberation.Frame         `json:"frame"`
		Observations []deliberation.ProbeResult `json:"observations"`
	}
	if err := json.Unmarshal([]byte(input), &frame); err == nil && frame.Frame.Request != "" {
		var answer any
		if strings.Contains(system, "read-only proposer") {
			c.roles[deliberation.Proposer]++
			if !c.proposerRead {
				c.proposerRead = true
				return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: "inspect", Name: "read_file", Input: map[string]any{"path": "store/store.go"}}}}, nil
			}
			if len(frame.Observations) == 0 || frame.Observations[0].Error != "" || !strings.Contains(frame.Observations[0].Output, "return 1") {
				return nil, fmt.Errorf("proposer did not receive the real read result")
			}
			answer = deliberation.Proposal{Candidates: []deliberation.Candidate{
				{ID: "restore", Brief: "Restore Get's promised value with the existing behavioral witness", Target: "store/store.go", SuccessTest: "TestGet passes without changing its assertion", Evidence: []string{"store/store.go:3; measured TestGet failure"}},
				{ID: "rewrite", Brief: "Refactor the constant through a helper", Target: "store/store.go", SuccessTest: "TestGet passes", Evidence: []string{"store/store.go:3"}},
			}}
		} else if strings.Contains(system, "codeNERD's challenger") {
			c.roles[deliberation.Challenger]++
			answer = deliberation.Challenge{Recommendation: "restore", Assessments: []deliberation.Assessment{
				{CandidateID: "restore", Success: true},
				{CandidateID: "rewrite", Success: false, Items: []deliberation.Item{{Key: "unnecessary-indirection", Kind: "design", Severity: "critical"}}},
			}}
		} else {
			return nil, fmt.Errorf("role JIT prompt missing: %s", system)
		}
		b, err := json.Marshal(answer)
		return &types.LLMToolResponse{Text: string(b)}, err
	}
	if !strings.Contains(input, "mission_id") {
		return nil, fmt.Errorf("worker lost the admitted mission receipt")
	}
	c.workerRound++
	tool := func(name string, args map[string]any) (*types.LLMToolResponse, error) {
		for _, def := range defs {
			if def.Name == name {
				return &types.LLMToolResponse{ToolCalls: []types.ToolCall{{ID: fmt.Sprintf("worker-%d", c.workerRound), Name: name, Input: args}}}, nil
			}
		}
		return nil, fmt.Errorf("production catalog omitted %s", name)
	}
	switch c.workerRound {
	case 1:
		return tool("write_file", map[string]any{"path": "store/store.go", "content": missionFixedSource})
	case 2:
		return tool("run_check", map[string]any{})
	default:
		return &types.LLMToolResponse{Text: "Restored Get's promised value. The declared check passes.", StopReason: "end_turn"}, nil
	}
}

func (c *missionCycleModel) CompleteWithToolResults(ctx context.Context, system string, history []types.Message, defs []types.ToolDefinition) (*types.LLMToolResponse, error) {
	var b strings.Builder
	for _, m := range history {
		b.WriteString(m.Text)
		b.WriteByte('\n')
	}
	return c.CompleteWithTools(ctx, system, b.String(), defs)
}

func TestMissionRecurse_CommandAdapterThroughProductionCycle(t *testing.T) {
	if testing.Short() {
		t.Skip("production cortex and host gates")
	}
	root := t.TempDir()
	for path, body := range map[string]string{
		"go.mod":              "module example.com/mission\n\ngo 1.21\n",
		".gitignore":          ".nerd/\n",
		"store/store.go":      "package store\n\nfunc Get() int { return 1 }\n",
		"store/store_test.go": "package store\n\nimport \"testing\"\n\nfunc TestGet(t *testing.T) {\n\tif Get() != 2 {\n\t\tt.Fatal(\"Get() != 2\")\n\t}\n}\n",
	} {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0644); err != nil {
			t.Fatal(err)
		}
	}
	for _, args := range [][]string{{"init", "--quiet", "-b", "main"}, {"config", "user.name", "fixture"},
		{"config", "user.email", "fixture@example.com"}, {"config", "core.autocrlf", "false"}, {"config", "commit.gpgsign", "false"},
		{"add", "--all"}, {"commit", "--quiet", "-m", "fixture"}} {
		cmd := exec.Command("git", args...)
		cmd.Dir = root
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("fixture git %v: %v %s", args, err, output)
		}
	}
	// Fail-before evidence is observed by the same host runner as the loop.
	before, err := gates.RunWitness(t.Context(), root, []string{"go", "test", "./store", "-run", "^TestGet$", "-count=1"})
	if err != nil || before.Passed || !strings.Contains(before.Output, "Get() != 2") {
		t.Fatalf("missing failing production witness: %+v %v", before, err)
	}

	usage.RegisterPrice(missionFixtureModel, usage.Price{InputPerMTok: 1, OutputPerMTok: 1})
	client := &missionCycleModel{roles: map[string]int{}}
	cfg := config.DefaultUserConfig()
	// No embedding model is needed: boot degrades locally without contacting a
	// provider, and the persisted configuration still passes the real checker.
	cfg.Embedding = &config.EmbeddingConfig{Provider: "ollama"}
	cfg.Provider, cfg.APIKey = "", ""
	cfg.WorkMode = config.WorkModeYolo
	cfg.Mission.MaxCycles = 1
	cfg.Campaign.MaxParallelTasks = cfg.Mission.Concurrency
	if err := os.MkdirAll(filepath.Join(root, ".nerd"), 0700); err != nil {
		t.Fatal(err)
	}
	body, err := json.Marshal(cfg)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".nerd/config.json"), body, 0600); err != nil {
		t.Fatal(err)
	}
	// The campaign adapter reloads this file even when Cortex boot uses an
	// override. Catch invalid fixture configuration before the attempt runs.
	if _, err := config.LoadUserConfig(filepath.Join(root, ".nerd/config.json")); err != nil {
		t.Fatalf("campaign fixture config: %v", err)
	}
	for _, key := range []string{"ANTHROPIC_API_KEY", "OPENAI_API_KEY", "GEMINI_API_KEY", "XAI_API_KEY", "ZAI_API_KEY", "OPENROUTER_API_KEY"} {
		t.Setenv(key, "")
	}
	cortex, err := coresys.BootCortexWithConfig(t.Context(), coresys.BootConfig{Workspace: root, SessionID: "mission-cycle",
		UserConfigOverride: cfg, LLMClientOverride: client,
		DisableSystemShards: []string{"constitution_gate", "perception_firewall", "executive_policy", "session_planner", "tactile_router", "campaign_runner", "mangle_repair", "legislator"}})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = cortex.Close() })
	mission, err := newRecurseMission(cortex, root, "Get preserves its promised value without weakening the witness", cfg)
	if err != nil {
		t.Fatal(err)
	}
	var eventsMu sync.Mutex
	var live []sessionlog.LiveEvent
	ctx := sessionlog.WithLiveProgress(t.Context(), func(_ context.Context, ev sessionlog.LiveEvent) {
		eventsMu.Lock()
		defer eventsMu.Unlock()
		live = append(live, ev)
	})
	res, err := campaign.RunRecurseCycles(ctx, campaign.RecurseCycleConfig{Workspace: root, Kernel: cortex.Kernel, Mission: mission,
		Execute: func(ctx context.Context, a campaign.RecurseAttempt) ([]observation.FileWrite, error) {
			camp := campaign.RecurseAttemptCampaign(root, a)
			defer campaign.ReleaseRecurseAttempt(root, cortex.Kernel, camp)
			cfg, prompts, err := buildCampaignOrchestratorConfig(cortex, root, nil, nil)
			if err != nil {
				return nil, err
			}
			orch, err := newCampaignOrchestrator(cfg, prompts)
			if err != nil {
				return nil, err
			}
			defer orch.Close()
			if err := orch.SetCampaign(camp); err != nil {
				return nil, err
			}
			err = orch.Run(ctx)
			return orch.KeptWrites(), err
		}})
	if err != nil {
		t.Fatal(err)
	}
	if res.Kept != 1 || res.MissionStatus != "/cycles" || mission.State.Cycles != 1 {
		t.Fatalf("full cycle/bound failed: %+v %+v", res, mission.State)
	}
	if client.roles[deliberation.Proposer] == 0 || client.roles[deliberation.Challenger] == 0 || client.workerRound < 2 {
		t.Fatalf("missing real actor path: %+v", client)
	}
	after, err := gates.RunWitness(t.Context(), root, []string{"go", "test", "./store", "-run", "^TestGet$", "-count=1"})
	if err != nil || !after.Passed {
		t.Fatalf("post-fix witness: %+v %v", after, err)
	}
	kept, err := os.ReadFile(filepath.Join(root, "store/store.go"))
	if err != nil || string(kept) != missionFixedSource {
		t.Fatalf("scripted repair was not retained: %q %v", kept, err)
	}
	b, err := os.ReadFile(res.MissionStatePath)
	if err != nil {
		t.Fatal(err)
	}
	var saved campaign.MissionState
	if err := json.Unmarshal(b, &saved); err != nil {
		t.Fatal(err)
	}
	if saved.Pending != nil || len(saved.Receipts) != 1 || saved.Receipts[0].Commit == "" || saved.Receipts[0].OwnedWritesRef == "" || saved.Receipts[0].ChecksRef == "" || saved.Receipts[0].Before == saved.Receipts[0].After || saved.Receipts[0].Evidence != "/supported_gate_deficit" {
		t.Fatalf("receipt does not substantiate cycle: %+v", saved)
	}
	if len(saved.StopFacts) != 1 || len(live) == 0 {
		t.Fatalf("missing policy stop/progress: %+v", saved)
	}
	checks, err := os.ReadFile(saved.Receipts[0].ChecksRef)
	if err != nil {
		t.Fatal(err)
	}
	var artifact struct {
		Before map[string]struct {
			Status string `json:"status"`
		} `json:"before"`
		After map[string]struct {
			Status string `json:"status"`
		} `json:"after"`
	}
	if err := json.Unmarshal(checks, &artifact); err != nil {
		t.Fatal(err)
	}
	if artifact.Before["go:test@store"].Status != "/fail" || artifact.After["go:test@store"].Status != "/pass" {
		t.Fatalf("retained host check does not show fail-before/pass-after: %s", checks)
	}
}
