package main

import (
	"os"
	"path/filepath"
	"testing"
)

// The CLI's campaign policy is the workspace config's campaign section: a key
// the user writes reaches the orchestrator, and a config that does not load
// stops the campaign instead of running it on defaults. No Cortex boot: the
// builder reads the section before it touches the Cortex.
func TestCampaignBuilder_TakesThePolicyFromTheWorkspaceConfig(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".nerd"), 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, ".nerd", "config.json"), []byte(body), 0o600); err != nil {
			t.Fatal(err)
		}
	}

	write(`{"campaign": {"max_task_attempts": 1, "max_parallel_tasks": 1}}`)
	cfg, _, err := buildCampaignOrchestratorConfig(nil, dir, nil, nil)
	if err != nil {
		t.Fatalf("buildCampaignOrchestratorConfig: %v", err)
	}
	if cfg.Campaign.MaxTaskAttempts != 1 || cfg.Campaign.MaxParallelTasks != 1 {
		t.Fatalf("campaign policy = %+v, want the file's max_task_attempts 1 and max_parallel_tasks 1", cfg.Campaign)
	}

	write(`{"campaign": {"max_task_attempt": 1}}`)
	if _, _, err := buildCampaignOrchestratorConfig(nil, dir, nil, nil); err == nil {
		t.Fatal("a config the strict decoder refuses built an orchestrator config; the campaign would run on defaults")
	}
}
