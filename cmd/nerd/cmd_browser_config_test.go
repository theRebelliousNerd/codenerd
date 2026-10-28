package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestGetBrowserConfigReadsWorkspaceNativeSettings(t *testing.T) {
	workspace := t.TempDir()
	t.Chdir(workspace)
	nerdDir := filepath.Join(workspace, ".nerd")
	if err := os.MkdirAll(nerdDir, 0o755); err != nil {
		t.Fatal(err)
	}
	configJSON := []byte(`{
		"browser": {
			"headless": true,
			"multi_tab_default": false,
			"max_tabs": 9,
			"max_browsers": 2,
			"idle_tab_timeout_ms": 45000,
			"reaper": {"timeout_ms": 6500, "poll_interval_ms": 120}
		}
	}`)
	if err := os.WriteFile(filepath.Join(nerdDir, "config.json"), configJSON, 0o600); err != nil {
		t.Fatal(err)
	}

	cfg := getBrowserConfig()
	if !cfg.Headless || cfg.IsMultiTabDefault() {
		t.Fatalf("native browser booleans not loaded: %+v", cfg)
	}
	if cfg.GetMaxTabs() != 9 || cfg.GetMaxBrowsers() != 2 || cfg.IdleTabTimeoutMs != 45000 {
		t.Fatalf("native browser limits not loaded: %+v", cfg)
	}
	if cfg.Reaper.TimeoutMs != 6500 || cfg.Reaper.PollIntervalMs != 120 {
		t.Fatalf("browser.reaper was not copied into the CLI manager: %+v", cfg.Reaper)
	}
	if cfg.SessionStore != filepath.Join(workspace, ".nerd", "browser", "sessions.json") {
		t.Fatalf("session store escaped workspace: %s", cfg.SessionStore)
	}
}
