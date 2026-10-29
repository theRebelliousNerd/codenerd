package shards

import (
	"strings"
	"testing"
	"time"

	"codenerd/internal/types"
)

func TestBaseShardAgentPermissionsAndStop(t *testing.T) {
	cfg := types.ShardConfig{
		Permissions: []types.ShardPermission{types.PermissionReadFile},
	}
	agent := NewBaseShardAgent("agent-1", cfg)

	if !agent.HasPermission(types.PermissionReadFile) {
		t.Fatalf("expected read permission")
	}
	if agent.HasPermission(types.PermissionExecCmd) {
		t.Fatalf("unexpected exec permission")
	}

	if err := agent.Stop(); err != nil {
		t.Fatalf("stop failed: %v", err)
	}
	if agent.GetState() != types.ShardStateCompleted {
		t.Fatalf("expected state completed after stop")
	}
	if err := agent.Stop(); err != nil {
		t.Fatalf("stop should be idempotent: %v", err)
	}
}

func TestBaseShardAgentBuildSessionContextPrompt(t *testing.T) {
	ctx := &types.SessionContext{
		CurrentDiagnostics: []string{"lint error"},
		TestState:          "/failing",
		FailingTests:       []string{"TestOne"},
		TDDRetryCount:      2,
		RecentFindings:     []string{"finding"},
		ImpactedFiles:      []string{"file.go"},
		GitBranch:          "main",
		GitRecentCommits:   []string{"commit one"},
		CampaignActive:     true,
		CampaignPhase:      "phase-1",
		CampaignGoal:       "goal",
		PriorShardOutputs: []types.ShardSummary{
			{
				ShardType: "tester",
				Task:      "run tests",
				Summary:   "failed",
				Timestamp: time.Now(),
				Success:   false,
			},
		},
		RecentActions:     []string{"action"},
		KnowledgeAtoms:    []string{"atom"},
		SpecialistHints:   []string{"hint"},
		BlockedActions:    []string{"rm -rf"},
		SafetyWarnings:    []string{"warning"},
		CompressedHistory: "compressed history",
	}

	cfg := types.ShardConfig{SessionContext: ctx}
	agent := NewBaseShardAgent("agent-1", cfg)

	prompt := agent.BuildSessionContextPrompt()
	expect := []string{
		"CURRENT BUILD/LINT ISSUES:",
		"TEST STATE: FAILING",
		"RECENT FINDINGS:",
		"IMPACTED FILES:",
		"GIT CONTEXT:",
		"CAMPAIGN CONTEXT:",
		"PRIOR SHARD RESULTS:",
		"SESSION ACTIONS:",
		"DOMAIN KNOWLEDGE:",
		"SAFETY CONSTRAINTS:",
		"SESSION HISTORY (compressed):",
	}
	for _, fragment := range expect {
		if !strings.Contains(prompt, fragment) {
			t.Fatalf("expected prompt to include %q", fragment)
		}
	}
}
