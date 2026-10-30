package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// nerd campaign start sized the campaign's context at 200000 tokens whenever
// the workspace config did not say: a number nobody chose. The budget is the
// workspace's context_window.max_tokens; with none set the command refuses.
func TestCampaignContextBudget_ComesFromTheWorkspaceConfig(t *testing.T) {
	nerdDir := t.TempDir()
	if _, err := campaignContextBudget(nerdDir); err == nil || !strings.Contains(err.Error(), "context_window.max_tokens") {
		t.Fatalf("a workspace with no config got a budget (err %v)", err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), []byte(`{"context_window": {"max_tokens": 64000}}`), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := campaignContextBudget(nerdDir)
	if err != nil || got != 64000 {
		t.Fatalf("budget = %d (%v), want the configured 64000", got, err)
	}
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := campaignContextBudget(nerdDir); err == nil {
		t.Fatal("a config without context_window got a budget")
	}
}
